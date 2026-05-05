package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"event-processing-service/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping api integration tests")
	}
	gdb, err := db.Open(context.Background(), url)
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

func preClean(t *testing.T, gdb *gorm.DB, model any, where string, args ...any) {
	t.Helper()
	gdb.Where(where, args...).Delete(model)
	t.Cleanup(func() { gdb.Where(where, args...).Delete(model) })
}

// do fires a request against the router and returns the recorder.
func do(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v — body: %s", err, rec.Body.String())
	}
	return out
}

// TestRoutes_GetPendingPredictions seeds two pending predictions and verifies
// the handler returns 200 with both in the list, scoped to the tenant.
func TestRoutes_GetPendingPredictions(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "rt-tenant-pred"
	predIDs := []string{"rt-pred-1", "rt-pred-2"}
	preClean(t, gdb, &db.Prediction{}, "prediction_id IN ?", predIDs)

	for _, id := range predIDs {
		p := &db.Prediction{
			PredictionID:    id,
			TenantID:        tenantID,
			DeviceID:        "DEV-01",
			AssetID:         "asset-rt",
			ModelName:       "test-model",
			ModelVersion:    "v1",
			AnomalyScore:    0.8,
			PredictedStatus: "warning",
			ReviewStatus:    "pending_review",
		}
		if err := db.InsertPrediction(ctx, gdb, p); err != nil {
			t.Fatalf("seed prediction %s: %v", id, err)
		}
	}

	router := NewRouter(gdb)
	rec := do(t, router, http.MethodGet, "/tenants/"+tenantID+"/predictions/pending", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)

	count := int(body["count"].(float64))
	if count < 2 {
		t.Errorf("count = %d, want ≥ 2", count)
	}

	predictions := body["predictions"].([]any)
	ids := map[string]bool{}
	for _, raw := range predictions {
		p := raw.(map[string]any)
		ids[p["prediction_id"].(string)] = true
		if p["tenant_id"].(string) != tenantID {
			t.Errorf("result tenant_id = %q, want %q", p["tenant_id"], tenantID)
		}
	}
	for _, wantID := range predIDs {
		if !ids[wantID] {
			t.Errorf("prediction %q missing from response", wantID)
		}
	}
}

// TestRoutes_GetReviewedPredictions seeds reviews for two tenants and verifies
// the handler returns only the requesting tenant's reviews.
func TestRoutes_GetReviewedPredictions(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "rt-tenant-rv"
	reviewIDs := []string{"rt-rv-1", "rt-rv-2", "rt-rv-other"}
	preClean(t, gdb, &db.PredictionReview{}, "review_id IN ?", reviewIDs)

	eligible := true
	for i, rv := range []struct{ reviewID, predID, tenant string }{
		{"rt-rv-1", "rt-pred-rv-1", tenantID},
		{"rt-rv-2", "rt-pred-rv-2", tenantID},
		{"rt-rv-other", "rt-pred-rv-other", "rt-tenant-rv-other"},
	} {
		_ = i
		r := &db.PredictionReview{
			ReviewID:           rv.reviewID,
			TenantID:           rv.tenant,
			PredictionID:       rv.predID,
			DeviceID:           "DEV-01",
			AssetID:            "asset-rt",
			ModelPrediction:    "warning",
			ReviewedLabel:      "normal",
			ReviewedBy:         "user-1",
			IsTrainingEligible: &eligible,
			ReviewedAt:         time.Now(),
		}
		if err := db.InsertPredictionReview(ctx, gdb, r); err != nil {
			t.Fatalf("seed review %s: %v", rv.reviewID, err)
		}
	}

	router := NewRouter(gdb)
	rec := do(t, router, http.MethodGet, "/tenants/"+tenantID+"/reviews", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)

	reviews := body["reviews"].([]any)
	ids := map[string]bool{}
	for _, raw := range reviews {
		rv := raw.(map[string]any)
		if rv["tenant_id"].(string) != tenantID {
			t.Errorf("result tenant_id = %q, want %q", rv["tenant_id"], tenantID)
		}
		ids[rv["review_id"].(string)] = true
	}
	for _, wantID := range []string{"rt-rv-1", "rt-rv-2"} {
		if !ids[wantID] {
			t.Errorf("review %q missing from response", wantID)
		}
	}
	if ids["rt-rv-other"] {
		t.Error("cross-tenant review rt-rv-other must not appear")
	}
}

// TestRoutes_GetRetrainingConfig_NotFound verifies a 404 when no config exists.
func TestRoutes_GetRetrainingConfig_NotFound(t *testing.T) {
	gdb := openTestDB(t)
	tenantID := "rt-tenant-cfg-none"
	preClean(t, gdb, &db.RetrainingConfig{}, "tenant_id = ?", tenantID)

	router := NewRouter(gdb)
	rec := do(t, router, http.MethodGet, "/tenants/"+tenantID+"/retraining/config", nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestRoutes_UpdateAndGetRetrainingConfig verifies that PUT creates/updates config
// and a subsequent GET returns the saved values.
func TestRoutes_UpdateAndGetRetrainingConfig(t *testing.T) {
	gdb := openTestDB(t)
	tenantID := "rt-tenant-cfg"
	preClean(t, gdb, &db.RetrainingConfig{}, "tenant_id = ?", tenantID)

	router := NewRouter(gdb)

	reqBody := map[string]any{
		"minimum_reviewed_records": 250,
		"auto_retrain_enabled":     true,
		"requires_manual_approval": false,
		"updated_by":               "admin",
	}
	rec := do(t, router, http.MethodPut, "/tenants/"+tenantID+"/retraining/config", reqBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}

	rec = do(t, router, http.MethodGet, "/tenants/"+tenantID+"/retraining/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["tenant_id"].(string) != tenantID {
		t.Errorf("tenant_id = %q, want %q", body["tenant_id"], tenantID)
	}
	if int(body["minimum_reviewed_records"].(float64)) != 250 {
		t.Errorf("minimum_reviewed_records = %v, want 250", body["minimum_reviewed_records"])
	}
	if !body["auto_retrain_enabled"].(bool) {
		t.Error("auto_retrain_enabled = false, want true")
	}
}

// TestRoutes_ApproveRetrainingRequest seeds a request with status "created" and
// verifies that POST .../approve transitions it to "approved".
func TestRoutes_ApproveRetrainingRequest(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "rt-tenant-rr"
	requestID := "rt-rr-001"
	preClean(t, gdb, &db.RetrainingRequest{}, "request_id = ?", requestID)

	req := &db.RetrainingRequest{
		RequestID:         requestID,
		TenantID:          tenantID,
		Status:            "created",
		TrainingDataCount: 100,
		RequestedBy:       "user-1",
	}
	if err := db.InsertRetrainingRequest(ctx, gdb, req); err != nil {
		t.Fatalf("seed retraining request: %v", err)
	}

	router := NewRouter(gdb)
	path := "/tenants/" + tenantID + "/retraining/" + requestID + "/approve"
	rec := do(t, router, http.MethodPost, path, map[string]any{"approved_by": "admin"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["status"].(string) != "approved" {
		t.Errorf("status = %q, want approved", body["status"])
	}
	if body["request_id"].(string) != requestID {
		t.Errorf("request_id = %q, want %q", body["request_id"], requestID)
	}

	// Verify DB state was updated.
	got, err := db.GetRetrainingRequestByID(ctx, gdb, tenantID, requestID)
	if err != nil {
		t.Fatalf("GetRetrainingRequestByID: %v", err)
	}
	if got.Status != "approved" {
		t.Errorf("DB status = %q, want approved", got.Status)
	}
}

// TestRoutes_GetModelVersions seeds a trained model version and verifies the
// handler returns it when filtered by status=trained.
func TestRoutes_GetModelVersions(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "rt-tenant-mv"
	modelID := "rt-model-list"
	preClean(t, gdb, &db.ModelVersion{}, "tenant_id = ? AND model_id = ?", tenantID, modelID)

	mv := &db.ModelVersion{
		ModelID:          modelID,
		TenantID:         tenantID,
		ModelName:        "anomaly-detector",
		ModelVersion:     "v1",
		DeploymentStatus: "trained",
		TrainingDate:     time.Now(),
	}
	if err := db.InsertModelVersion(ctx, gdb, mv); err != nil {
		t.Fatalf("seed model version: %v", err)
	}

	router := NewRouter(gdb)
	rec := do(t, router, http.MethodGet, "/tenants/"+tenantID+"/models/versions?status=trained", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	versions := body["versions"].([]any)
	if len(versions) == 0 {
		t.Fatal("versions list is empty, expected at least one")
	}
	found := false
	for _, raw := range versions {
		v := raw.(map[string]any)
		if v["model_id"].(string) == modelID {
			found = true
			if v["deployment_status"].(string) != "trained" {
				t.Errorf("deployment_status = %q, want trained", v["deployment_status"])
			}
		}
	}
	if !found {
		t.Errorf("model_id %q not found in versions response", modelID)
	}
}

// TestRoutes_GetActiveModelVersion_NotFound verifies a 404 when no active model is set.
func TestRoutes_GetActiveModelVersion_NotFound(t *testing.T) {
	gdb := openTestDB(t)
	tenantID := "rt-tenant-active-none"
	preClean(t, gdb, &db.ActiveModelVersion{}, "tenant_id = ?", tenantID)

	router := NewRouter(gdb)
	rec := do(t, router, http.MethodGet, "/tenants/"+tenantID+"/models/active", nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestRoutes_ApproveModelVersion seeds a trained version and verifies POST .../approve
// returns 200 with the correct identifiers.
func TestRoutes_ApproveModelVersion(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "rt-tenant-mv-approve"
	modelID := "rt-model-approve"
	preClean(t, gdb, &db.ModelVersion{}, "tenant_id = ? AND model_id = ?", tenantID, modelID)

	mv := &db.ModelVersion{
		ModelID:          modelID,
		TenantID:         tenantID,
		ModelName:        "anomaly-detector",
		ModelVersion:     "v1",
		DeploymentStatus: "trained",
		TrainingDate:     time.Now(),
	}
	if err := db.InsertModelVersion(ctx, gdb, mv); err != nil {
		t.Fatalf("seed model version: %v", err)
	}

	router := NewRouter(gdb)
	path := "/tenants/" + tenantID + "/models/" + modelID + "/v1/approve"
	rec := do(t, router, http.MethodPost, path, map[string]any{"approved_by": "admin"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["status"].(string) != "approved" {
		t.Errorf("status = %q, want approved", body["status"])
	}

	// Confirm DB state.
	got, err := db.GetModelVersion(ctx, gdb, tenantID, modelID, "v1")
	if err != nil {
		t.Fatalf("GetModelVersion: %v", err)
	}
	if got.DeploymentStatus != "approved" {
		t.Errorf("deployment_status = %q, want approved", got.DeploymentStatus)
	}
}

// TestRoutes_DeployModelVersion seeds a model version and verifies POST .../deploy
// returns 200 and sets it as the active model for the tenant.
func TestRoutes_DeployModelVersion(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	tenantID := "rt-tenant-mv-deploy"
	modelID := "rt-model-deploy"
	preClean(t, gdb, &db.ModelVersion{}, "tenant_id = ? AND model_id = ?", tenantID, modelID)
	preClean(t, gdb, &db.ActiveModelVersion{}, "tenant_id = ?", tenantID)

	mv := &db.ModelVersion{
		ModelID:          modelID,
		TenantID:         tenantID,
		ModelName:        "anomaly-detector",
		ModelVersion:     "v1",
		DeploymentStatus: "approved",
		TrainingDate:     time.Now(),
	}
	if err := db.InsertModelVersion(ctx, gdb, mv); err != nil {
		t.Fatalf("seed model version: %v", err)
	}

	router := NewRouter(gdb)
	path := "/tenants/" + tenantID + "/models/" + modelID + "/v1/deploy"
	rec := do(t, router, http.MethodPost, path, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec)
	if body["status"].(string) != "deployed" {
		t.Errorf("status = %q, want deployed", body["status"])
	}

	// Verify the active model was set.
	active, err := db.GetActiveModelVersion(ctx, gdb, tenantID)
	if err != nil {
		t.Fatalf("GetActiveModelVersion: %v", err)
	}
	if active == nil {
		t.Fatal("expected active model after deploy, got nil")
	}
	if active.ActiveModelID != modelID {
		t.Errorf("ActiveModelID = %q, want %q", active.ActiveModelID, modelID)
	}
	if active.ActiveVersion != "v1" {
		t.Errorf("ActiveVersion = %q, want v1", active.ActiveVersion)
	}

	// Verify DB deployment status.
	got, err := db.GetModelVersion(ctx, gdb, tenantID, modelID, "v1")
	if err != nil {
		t.Fatalf("GetModelVersion: %v", err)
	}
	if got.DeploymentStatus != "deployed" {
		t.Errorf("deployment_status = %q, want deployed", got.DeploymentStatus)
	}
}
