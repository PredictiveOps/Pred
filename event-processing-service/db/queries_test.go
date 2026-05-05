package db

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"
)

// preClean deletes rows matching the condition before the test runs and again
// on teardown, making tests safe to re-run against a persistent test database.
func preClean(t *testing.T, gdb *gorm.DB, model any, where string, args ...any) {
	t.Helper()
	gdb.Where(where, args...).Delete(model)
	t.Cleanup(func() { gdb.Where(where, args...).Delete(model) })
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping db integration tests")
	}
	ctx := context.Background()
	gdb, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return gdb
}

func TestInsertEvent_StoresPayload(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	payload := map[string]any{"device_id": "DEV-01", "v_rms": 0.42}
	raw, _ := json.Marshal(payload)

	id, err := InsertEvent(ctx, gdb, "tenant-event", raw)
	if err != nil {
		t.Fatalf("InsertEvent: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	var e Event
	if err := gdb.First(&e, id).Error; err != nil {
		t.Fatalf("fetch event: %v", err)
	}
	if len(e.Payload) == 0 {
		t.Fatal("payload was empty after round-trip")
	}
	if e.TenantID != "tenant-event" {
		t.Fatalf("tenant_id = %q, want %q", e.TenantID, "tenant-event")
	}
}

func newPF(tenantID, assetID string, ts time.Time) *ProcessedFeatures {
	raw, _ := json.Marshal(map[string]any{"rms": 1.0})
	return &ProcessedFeatures{
		TenantID:         tenantID,
		DeviceID:         "DEV-01",
		AssetID:          assetID,
		Features:         raw,
		FeatureVersion:   "v1",
		FeatureTimestamp: ts,
	}
}

func TestInsertProcessedFeatures(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	pf := newPF("tenant-pf", "asset-a", time.Now())
	if err := InsertProcessedFeatures(ctx, gdb, pf); err != nil {
		t.Fatalf("InsertProcessedFeatures: %v", err)
	}
	if pf.ID == 0 {
		t.Fatal("expected non-zero ID after insert")
	}
}

func TestGetLatestProcessedFeatures_ReturnsNewest(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-latest"
	assetID := "asset-latest"

	now := time.Now().UTC().Truncate(time.Millisecond)
	older := newPF(tenantID, assetID, now.Add(-time.Hour))
	newer := newPF(tenantID, assetID, now)

	if err := InsertProcessedFeatures(ctx, gdb, older); err != nil {
		t.Fatalf("insert older: %v", err)
	}
	if err := InsertProcessedFeatures(ctx, gdb, newer); err != nil {
		t.Fatalf("insert newer: %v", err)
	}

	got, err := GetLatestProcessedFeatures(ctx, gdb, tenantID, assetID)
	if err != nil {
		t.Fatalf("GetLatestProcessedFeatures: %v", err)
	}
	if got.ID != newer.ID {
		t.Fatalf("got ID=%d, want ID=%d (newer record)", got.ID, newer.ID)
	}
}

func TestGetProcessedFeaturesByAsset_TenantIsolation(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	assetID := "asset-isolated"

	pfA := newPF("tenant-A", assetID, time.Now())
	pfB := newPF("tenant-B", assetID, time.Now())
	if err := InsertProcessedFeatures(ctx, gdb, pfA); err != nil {
		t.Fatalf("insert tenant-A: %v", err)
	}
	if err := InsertProcessedFeatures(ctx, gdb, pfB); err != nil {
		t.Fatalf("insert tenant-B: %v", err)
	}

	results, err := GetProcessedFeaturesByAsset(ctx, gdb, "tenant-A", assetID, 100)
	if err != nil {
		t.Fatalf("GetProcessedFeaturesByAsset: %v", err)
	}
	for _, r := range results {
		if r.TenantID != "tenant-A" {
			t.Errorf("returned record with tenant_id=%q, want %q", r.TenantID, "tenant-A")
		}
	}

	found := false
	for _, r := range results {
		if r.ID == pfA.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("tenant-A record not found in results")
	}
}

// newPrediction builds a minimal Prediction for a given tenant.
// predictionID must be unique across the test run.
func newPrediction(tenantID, predictionID string) *Prediction {
	return &Prediction{
		PredictionID:    predictionID,
		TenantID:        tenantID,
		DeviceID:        "DEV-01",
		AssetID:         "asset-pred",
		ModelName:       "test-model",
		ModelVersion:    "v1",
		AnomalyScore:    0.75,
		PredictedStatus: "warning",
		ReviewStatus:    "pending_review",
		Reviewed:        false,
	}
}

func TestInsertPrediction_AndGetByID(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &Prediction{}, "prediction_id = ?", "pred-001")

	pred := newPrediction("tenant-pred", "pred-001")
	if err := InsertPrediction(ctx, gdb, pred); err != nil {
		t.Fatalf("InsertPrediction: %v", err)
	}
	if pred.ID == 0 {
		t.Fatal("expected non-zero ID after insert")
	}

	got, err := GetPredictionByID(ctx, gdb, "tenant-pred", "pred-001")
	if err != nil {
		t.Fatalf("GetPredictionByID: %v", err)
	}
	if got.PredictionID != "pred-001" {
		t.Errorf("PredictionID = %q, want %q", got.PredictionID, "pred-001")
	}
	if got.AnomalyScore != 0.75 {
		t.Errorf("AnomalyScore = %v, want 0.75", got.AnomalyScore)
	}
}

func TestGetPredictionByID_WrongTenantReturnsError(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &Prediction{}, "prediction_id = ?", "pred-002")

	pred := newPrediction("tenant-owner", "pred-002")
	if err := InsertPrediction(ctx, gdb, pred); err != nil {
		t.Fatalf("InsertPrediction: %v", err)
	}

	_, err := GetPredictionByID(ctx, gdb, "tenant-other", "pred-002")
	if err == nil {
		t.Fatal("expected error when fetching with wrong tenant, got nil")
	}
}

func TestGetPendingPredictions_TenantScopedAndStatusFiltered(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-pending"
	preClean(t, gdb, &Prediction{}, "prediction_id IN ?", []string{"pred-p1", "pred-p2", "pred-p3"})

	// Insert two pending predictions for the target tenant.
	for _, id := range []string{"pred-p1", "pred-p2"} {
		if err := InsertPrediction(ctx, gdb, newPrediction(tenantID, id)); err != nil {
			t.Fatalf("InsertPrediction %s: %v", id, err)
		}
	}
	// Insert a pending prediction for a different tenant — must not appear.
	if err := InsertPrediction(ctx, gdb, newPrediction("tenant-other-pending", "pred-p3")); err != nil {
		t.Fatalf("InsertPrediction pred-p3: %v", err)
	}

	results, err := GetPendingPredictions(ctx, gdb, tenantID, 100)
	if err != nil {
		t.Fatalf("GetPendingPredictions: %v", err)
	}
	for _, r := range results {
		if r.TenantID != tenantID {
			t.Errorf("result has tenant_id=%q, want %q", r.TenantID, tenantID)
		}
		if r.ReviewStatus != "pending_review" {
			t.Errorf("result has review_status=%q, want pending_review", r.ReviewStatus)
		}
	}

	// Confirm both inserted IDs are present.
	ids := map[string]bool{}
	for _, r := range results {
		ids[r.PredictionID] = true
	}
	for _, wantID := range []string{"pred-p1", "pred-p2"} {
		if !ids[wantID] {
			t.Errorf("prediction %q missing from pending results", wantID)
		}
	}
}

func TestGetPendingPredictions_ExcludesReviewed(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-reviewed"
	preClean(t, gdb, &Prediction{}, "prediction_id = ?", "pred-reviewed-1")

	pred := newPrediction(tenantID, "pred-reviewed-1")
	if err := InsertPrediction(ctx, gdb, pred); err != nil {
		t.Fatalf("InsertPrediction: %v", err)
	}
	if err := UpdatePredictionStatus(ctx, gdb, tenantID, "pred-reviewed-1", "reviewed", true); err != nil {
		t.Fatalf("UpdatePredictionStatus: %v", err)
	}

	results, err := GetPendingPredictions(ctx, gdb, tenantID, 100)
	if err != nil {
		t.Fatalf("GetPendingPredictions: %v", err)
	}
	for _, r := range results {
		if r.PredictionID == "pred-reviewed-1" {
			t.Error("reviewed prediction must not appear in pending results")
		}
	}
}

func TestUpdatePredictionStatus_TransitionsAndReviewedFlag(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-update"
	preClean(t, gdb, &Prediction{}, "prediction_id = ?", "pred-upd-1")

	pred := newPrediction(tenantID, "pred-upd-1")
	if err := InsertPrediction(ctx, gdb, pred); err != nil {
		t.Fatalf("InsertPrediction: %v", err)
	}

	cases := []struct {
		status   string
		reviewed bool
	}{
		{"reviewed", true},
		{"archived", true},
		{"pending_review", false},
	}
	for _, tc := range cases {
		if err := UpdatePredictionStatus(ctx, gdb, tenantID, "pred-upd-1", tc.status, tc.reviewed); err != nil {
			t.Fatalf("UpdatePredictionStatus(%q, %v): %v", tc.status, tc.reviewed, err)
		}
		got, err := GetPredictionByID(ctx, gdb, tenantID, "pred-upd-1")
		if err != nil {
			t.Fatalf("GetPredictionByID after update: %v", err)
		}
		if got.ReviewStatus != tc.status {
			t.Errorf("review_status = %q, want %q", got.ReviewStatus, tc.status)
		}
		if got.Reviewed != tc.reviewed {
			t.Errorf("reviewed = %v, want %v", got.Reviewed, tc.reviewed)
		}
	}
}

// newReview builds a minimal PredictionReview. reviewID and predictionID must be
// unique across the test run (PredictionReview has a uniqueIndex on each).
func newReview(tenantID, reviewID, predictionID string, eligible bool) *PredictionReview {
	return &PredictionReview{
		ReviewID:           reviewID,
		TenantID:           tenantID,
		PredictionID:       predictionID,
		DeviceID:           "DEV-01",
		AssetID:            "asset-review",
		ModelPrediction:    "warning",
		ReviewedLabel:      "normal",
		ReviewedBy:         "user-1",
		IsTrainingEligible: &eligible,
		ReviewedAt:         time.Now(),
	}
}

func TestInsertPredictionReview(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &PredictionReview{}, "review_id = ?", "rv-001")

	r := newReview("tenant-rv", "rv-001", "pred-rv-001", true)
	if err := InsertPredictionReview(ctx, gdb, r); err != nil {
		t.Fatalf("InsertPredictionReview: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("expected non-zero ID after insert")
	}
}

func TestGetReviewsByTenant_TenantIsolation(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &PredictionReview{}, "review_id IN ?", []string{"rv-A-1", "rv-B-1"})

	if err := InsertPredictionReview(ctx, gdb, newReview("tenant-rv-A", "rv-A-1", "pred-rv-A-1", true)); err != nil {
		t.Fatalf("insert tenant-rv-A review: %v", err)
	}
	if err := InsertPredictionReview(ctx, gdb, newReview("tenant-rv-B", "rv-B-1", "pred-rv-B-1", true)); err != nil {
		t.Fatalf("insert tenant-rv-B review: %v", err)
	}

	results, err := GetReviewsByTenant(ctx, gdb, "tenant-rv-A", 100)
	if err != nil {
		t.Fatalf("GetReviewsByTenant: %v", err)
	}
	for _, r := range results {
		if r.TenantID != "tenant-rv-A" {
			t.Errorf("result has tenant_id=%q, want %q", r.TenantID, "tenant-rv-A")
		}
	}
	found := false
	for _, r := range results {
		if r.ReviewID == "rv-A-1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("rv-A-1 not found in GetReviewsByTenant results")
	}
}

func TestCountTrainingEligibleReviews(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-count"
	preClean(t, gdb, &PredictionReview{}, "review_id IN ?", []string{"rv-cnt-1", "rv-cnt-2", "rv-cnt-3"})

	// Start from the current count so the test is additive-safe.
	before, err := CountTrainingEligibleReviews(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("CountTrainingEligibleReviews (before): %v", err)
	}

	eligible := []*PredictionReview{
		newReview(tenantID, "rv-cnt-1", "pred-cnt-1", true),
		newReview(tenantID, "rv-cnt-2", "pred-cnt-2", true),
	}
	ineligible := newReview(tenantID, "rv-cnt-3", "pred-cnt-3", false)

	for _, r := range eligible {
		if err := InsertPredictionReview(ctx, gdb, r); err != nil {
			t.Fatalf("insert eligible review: %v", err)
		}
	}
	if err := InsertPredictionReview(ctx, gdb, ineligible); err != nil {
		t.Fatalf("insert ineligible review: %v", err)
	}

	after, err := CountTrainingEligibleReviews(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("CountTrainingEligibleReviews (after): %v", err)
	}
	if after-before != 2 {
		t.Errorf("count increased by %d, want 2", after-before)
	}
}

func TestGetTrainingEligibleReviews_FiltersAndIsolates(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-eligible"
	preClean(t, gdb, &PredictionReview{}, "review_id IN ?", []string{"rv-el-1", "rv-el-2", "rv-el-3", "rv-el-4"})

	eligible := []*PredictionReview{
		newReview(tenantID, "rv-el-1", "pred-el-1", true),
		newReview(tenantID, "rv-el-2", "pred-el-2", true),
	}
	ineligible := newReview(tenantID, "rv-el-3", "pred-el-3", false)
	otherTenant := newReview("tenant-eligible-other", "rv-el-4", "pred-el-4", true)

	for _, r := range eligible {
		if err := InsertPredictionReview(ctx, gdb, r); err != nil {
			t.Fatalf("insert eligible: %v", err)
		}
	}
	if err := InsertPredictionReview(ctx, gdb, ineligible); err != nil {
		t.Fatalf("insert ineligible: %v", err)
	}
	if err := InsertPredictionReview(ctx, gdb, otherTenant); err != nil {
		t.Fatalf("insert other-tenant: %v", err)
	}

	results, err := GetTrainingEligibleReviews(ctx, gdb, tenantID, 100)
	if err != nil {
		t.Fatalf("GetTrainingEligibleReviews: %v", err)
	}
	for _, r := range results {
		if r.TenantID != tenantID {
			t.Errorf("result tenant_id=%q, want %q", r.TenantID, tenantID)
		}
		if r.IsTrainingEligible == nil || !*r.IsTrainingEligible {
			t.Errorf("result review_id=%q has IsTrainingEligible=false", r.ReviewID)
		}
	}
	ids := map[string]bool{}
	for _, r := range results {
		ids[r.ReviewID] = true
	}
	for _, wantID := range []string{"rv-el-1", "rv-el-2"} {
		if !ids[wantID] {
			t.Errorf("eligible review %q missing from results", wantID)
		}
	}
	if ids["rv-el-3"] {
		t.Error("ineligible review rv-el-3 must not appear in results")
	}
}

// ---- Step 8: RetrainingConfig + RetrainingRequest ----

func TestGetRetrainingConfig_MissingReturnsNil(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &RetrainingConfig{}, "tenant_id = ?", "tenant-cfg-missing")

	cfg, err := GetRetrainingConfig(ctx, gdb, "tenant-cfg-missing")
	if err != nil {
		t.Fatalf("GetRetrainingConfig: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil for missing config, got %+v", cfg)
	}
}

func TestUpsertRetrainingConfig_IdempotentUpdate(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-cfg-upsert"
	preClean(t, gdb, &RetrainingConfig{}, "tenant_id = ?", tenantID)

	cfg := &RetrainingConfig{
		TenantID:               tenantID,
		MinimumReviewedRecords: 200,
		AutoRetrainEnabled:     false,
		RequiresManualApproval: true,
		UpdatedBy:              "user-1",
	}
	if err := UpsertRetrainingConfig(ctx, gdb, cfg); err != nil {
		t.Fatalf("UpsertRetrainingConfig (create): %v", err)
	}

	// Update a field and upsert again.
	cfg.MinimumReviewedRecords = 500
	cfg.AutoRetrainEnabled = true
	if err := UpsertRetrainingConfig(ctx, gdb, cfg); err != nil {
		t.Fatalf("UpsertRetrainingConfig (update): %v", err)
	}

	got, err := GetRetrainingConfig(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("GetRetrainingConfig: %v", err)
	}
	if got == nil {
		t.Fatal("expected config, got nil")
	}
	if got.MinimumReviewedRecords != 500 {
		t.Errorf("MinimumReviewedRecords = %d, want 500", got.MinimumReviewedRecords)
	}
	if !got.AutoRetrainEnabled {
		t.Error("AutoRetrainEnabled = false, want true")
	}
}

func newRetrainingRequest(tenantID, requestID string) *RetrainingRequest {
	return &RetrainingRequest{
		RequestID:         requestID,
		TenantID:          tenantID,
		Status:            "created",
		TrainingDataCount: 50,
		RequestedBy:       "user-1",
	}
}

func TestInsertRetrainingRequest_AndGetByID(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &RetrainingRequest{}, "request_id = ?", "rr-001")

	req := newRetrainingRequest("tenant-rr", "rr-001")
	if err := InsertRetrainingRequest(ctx, gdb, req); err != nil {
		t.Fatalf("InsertRetrainingRequest: %v", err)
	}
	if req.ID == 0 {
		t.Fatal("expected non-zero ID after insert")
	}

	got, err := GetRetrainingRequestByID(ctx, gdb, "tenant-rr", "rr-001")
	if err != nil {
		t.Fatalf("GetRetrainingRequestByID: %v", err)
	}
	if got.RequestID != "rr-001" {
		t.Errorf("RequestID = %q, want %q", got.RequestID, "rr-001")
	}
	if got.Status != "created" {
		t.Errorf("Status = %q, want %q", got.Status, "created")
	}
}

func TestGetRetrainingRequestByID_WrongTenantReturnsError(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &RetrainingRequest{}, "request_id = ?", "rr-002")

	req := newRetrainingRequest("tenant-rr-owner", "rr-002")
	if err := InsertRetrainingRequest(ctx, gdb, req); err != nil {
		t.Fatalf("InsertRetrainingRequest: %v", err)
	}

	_, err := GetRetrainingRequestByID(ctx, gdb, "tenant-rr-other", "rr-002")
	if err == nil {
		t.Fatal("expected error for wrong tenant, got nil")
	}
}

func TestUpdateRetrainingRequestStatus_StateMachine(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-rr-sm"
	preClean(t, gdb, &RetrainingRequest{}, "request_id = ?", "rr-sm-1")

	req := newRetrainingRequest(tenantID, "rr-sm-1")
	if err := InsertRetrainingRequest(ctx, gdb, req); err != nil {
		t.Fatalf("InsertRetrainingRequest: %v", err)
	}

	transitions := []string{"approved", "in_progress", "completed"}
	for _, status := range transitions {
		if err := UpdateRetrainingRequestStatus(ctx, gdb, tenantID, "rr-sm-1", status); err != nil {
			t.Fatalf("UpdateRetrainingRequestStatus(%q): %v", status, err)
		}
		got, err := GetRetrainingRequestByID(ctx, gdb, tenantID, "rr-sm-1")
		if err != nil {
			t.Fatalf("GetRetrainingRequestByID after %q: %v", status, err)
		}
		if got.Status != status {
			t.Errorf("status = %q, want %q", got.Status, status)
		}
	}
}

// ---- Step 9: ModelVersion lifecycle ----

func newModelVersion(tenantID, modelID, version string) *ModelVersion {
	return &ModelVersion{
		ModelID:          modelID,
		TenantID:         tenantID,
		ModelName:        "anomaly-detector",
		ModelVersion:     version,
		DeploymentStatus: "trained",
		TrainingDate:     time.Now(),
	}
}

func TestInsertModelVersion_AndGetByVersion(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	preClean(t, gdb, &ModelVersion{}, "tenant_id = ? AND model_id = ?", "tenant-mv", "model-A")

	mv := newModelVersion("tenant-mv", "model-A", "v1")
	if err := InsertModelVersion(ctx, gdb, mv); err != nil {
		t.Fatalf("InsertModelVersion: %v", err)
	}
	if mv.ID == 0 {
		t.Fatal("expected non-zero ID after insert")
	}

	got, err := GetModelVersion(ctx, gdb, "tenant-mv", "model-A", "v1")
	if err != nil {
		t.Fatalf("GetModelVersion: %v", err)
	}
	if got.ModelVersion != "v1" {
		t.Errorf("ModelVersion = %q, want %q", got.ModelVersion, "v1")
	}
	if got.DeploymentStatus != "trained" {
		t.Errorf("DeploymentStatus = %q, want trained", got.DeploymentStatus)
	}
}

func TestGetLatestModelVersion_ReturnsNewest(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-mv-latest"
	modelID := "model-B"
	preClean(t, gdb, &ModelVersion{}, "tenant_id = ? AND model_id = ?", tenantID, modelID)

	for _, ver := range []string{"v1", "v2", "v3"} {
		if err := InsertModelVersion(ctx, gdb, newModelVersion(tenantID, modelID, ver)); err != nil {
			t.Fatalf("InsertModelVersion %s: %v", ver, err)
		}
	}

	got, err := GetLatestModelVersion(ctx, gdb, tenantID, modelID)
	if err != nil {
		t.Fatalf("GetLatestModelVersion: %v", err)
	}
	if got.ModelVersion != "v3" {
		t.Errorf("ModelVersion = %q, want v3", got.ModelVersion)
	}
}

func TestUpdateModelVersionStatus(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-mv-status"
	modelID := "model-C"
	preClean(t, gdb, &ModelVersion{}, "tenant_id = ? AND model_id = ?", tenantID, modelID)

	if err := InsertModelVersion(ctx, gdb, newModelVersion(tenantID, modelID, "v1")); err != nil {
		t.Fatalf("InsertModelVersion: %v", err)
	}

	for _, status := range []string{"pending_approval", "approved", "deployed"} {
		if err := UpdateModelVersionStatus(ctx, gdb, tenantID, modelID, "v1", status); err != nil {
			t.Fatalf("UpdateModelVersionStatus(%q): %v", status, err)
		}
		got, err := GetModelVersion(ctx, gdb, tenantID, modelID, "v1")
		if err != nil {
			t.Fatalf("GetModelVersion after %q: %v", status, err)
		}
		if got.DeploymentStatus != status {
			t.Errorf("deployment_status = %q, want %q", got.DeploymentStatus, status)
		}
	}
}

func TestGetModelVersionsByStatus(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-mv-by-status"
	modelID := "model-D"
	preClean(t, gdb, &ModelVersion{}, "tenant_id = ? AND model_id = ?", tenantID, modelID)

	// Insert two trained, one approved.
	if err := InsertModelVersion(ctx, gdb, newModelVersion(tenantID, modelID, "v1")); err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	if err := InsertModelVersion(ctx, gdb, newModelVersion(tenantID, modelID, "v2")); err != nil {
		t.Fatalf("insert v2: %v", err)
	}
	mv3 := newModelVersion(tenantID, modelID, "v3")
	if err := InsertModelVersion(ctx, gdb, mv3); err != nil {
		t.Fatalf("insert v3: %v", err)
	}
	if err := UpdateModelVersionStatus(ctx, gdb, tenantID, modelID, "v3", "approved"); err != nil {
		t.Fatalf("update v3 to approved: %v", err)
	}

	trained, err := GetModelVersionsByStatus(ctx, gdb, tenantID, "trained", 100)
	if err != nil {
		t.Fatalf("GetModelVersionsByStatus(trained): %v", err)
	}
	for _, r := range trained {
		if r.DeploymentStatus != "trained" {
			t.Errorf("result has deployment_status=%q, want trained", r.DeploymentStatus)
		}
	}
	versions := map[string]bool{}
	for _, r := range trained {
		versions[r.ModelVersion] = true
	}
	if !versions["v1"] || !versions["v2"] {
		t.Error("expected v1 and v2 in trained results")
	}
	if versions["v3"] {
		t.Error("v3 (approved) must not appear in trained results")
	}
}

func TestSetActiveModelVersion_AndGet(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "tenant-mv-active"
	preClean(t, gdb, &ActiveModelVersion{}, "tenant_id = ?", tenantID)

	// No active version yet → should return nil.
	got, err := GetActiveModelVersion(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("GetActiveModelVersion (empty): %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing active version, got %+v", got)
	}

	if err := SetActiveModelVersion(ctx, gdb, tenantID, "model-E", "v1"); err != nil {
		t.Fatalf("SetActiveModelVersion v1: %v", err)
	}
	got, err = GetActiveModelVersion(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("GetActiveModelVersion after set: %v", err)
	}
	if got.ActiveVersion != "v1" {
		t.Errorf("ActiveVersion = %q, want v1", got.ActiveVersion)
	}

	// Replace with v2 — the upsert must overwrite, not insert a second row.
	if err := SetActiveModelVersion(ctx, gdb, tenantID, "model-E", "v2"); err != nil {
		t.Fatalf("SetActiveModelVersion v2: %v", err)
	}
	got, err = GetActiveModelVersion(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("GetActiveModelVersion after replace: %v", err)
	}
	if got.ActiveVersion != "v2" {
		t.Errorf("ActiveVersion = %q, want v2 after replace", got.ActiveVersion)
	}
}
