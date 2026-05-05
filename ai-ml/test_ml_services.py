


"""
Comprehensive ML Services Test Suite
Tests all ML service layers using an in-memory SQLite database.
No external dependencies (PostgreSQL, Docker) required.

Usage:
    cd ai-ml
    python test_ml_services.py
"""

from __future__ import annotations

import json
import sys
import traceback
from datetime import datetime, timezone
from pathlib import Path

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

# ── Setup ────────────────────────────────────────────────────────────────────
PROJECT_ROOT = Path(__file__).resolve().parent

# In-memory SQLite for isolated testing
ENGINE = create_engine("sqlite:///:memory:", echo=False)
SessionLocal = sessionmaker(bind=ENGINE, expire_on_commit=False)

# Create tables
from db_models import Base
Base.metadata.create_all(ENGINE)

# ── Test State ───────────────────────────────────────────────────────────────
TENANT = "test_tenant"
DEVICE = "test_device_001"
ASSET = "bearing_motor_001"
USER_ENGINEER = "engineer_01"
USER_ADMIN = "admin_01"

RESULTS = {"passed": 0, "failed": 0, "errors": []}


def run_test(name, fn):
    """Run a test function and track results."""
    try:
        result = fn()
        RESULTS["passed"] += 1
        print(f"  ✅ {name}")
        return result
    except Exception as e:
        RESULTS["failed"] += 1
        RESULTS["errors"].append((name, str(e)))
        print(f"  ❌ {name}: {e}")
        traceback.print_exc()
        return None


# ── Feature columns for the real model ───────────────────────────────────────
FEATURE_COLS_PATH = PROJECT_ROOT / "results" / "models" / "feature_columns.json"
if FEATURE_COLS_PATH.exists():
    with open(FEATURE_COLS_PATH) as f:
        FEATURE_COLUMNS = json.load(f)
else:
    FEATURE_COLUMNS = [
        "vibration_x_mean", "vibration_x_standard_deviation",
        "vibration_x_minimum", "vibration_x_maximum",
    ]

# Generate realistic dummy feature values
def make_features(profile="normal"):
    base = {col: 0.0 for col in FEATURE_COLUMNS}
    if profile == "normal":
        for col in base:
            base[col] = 0.5
    elif profile == "warning":
        for col in base:
            base[col] = 2.5
    elif profile == "critical":
        for col in base:
            base[col] = 8.0
    return base


# ============================================================================
# 1. TimeSeriesRepository Tests
# ============================================================================
def test_timeseries_repository():
    print("\n📦 TimeSeriesRepository Tests")
    from timeseries_repository import TimeSeriesRepository

    db = SessionLocal()
    repo = TimeSeriesRepository(db)

    saved = run_test("Save processed features", lambda: (
        repo.save_processed_features(
            tenant_id=TENANT, device_id=DEVICE, asset_id=ASSET,
            features=make_features("normal"),
            feature_timestamp=datetime.now(timezone.utc),
        )
    ))

    def test_get_latest():
        r = repo.get_latest_features(TENANT, ASSET)
        assert r is not None, "No features found"
        assert r.tenant_id == TENANT
        assert r.asset_id == ASSET
        return r

    run_test("Get latest features", test_get_latest)

    # Save more features for different profiles
    repo.save_processed_features(
        tenant_id=TENANT, device_id=DEVICE, asset_id=ASSET,
        features=make_features("warning"),
        feature_timestamp=datetime.now(timezone.utc),
    )

    def test_get_by_asset():
        r = repo.get_features_by_asset(TENANT, ASSET, limit=10)
        assert len(r) >= 2, f"Expected >=2 features, got {len(r)}"
        return r

    run_test("Get features by asset", test_get_by_asset)

    def test_get_by_device():
        r = repo.get_features_by_device(TENANT, DEVICE, limit=10)
        assert len(r) >= 2, f"Expected >=2 features, got {len(r)}"
        return r

    run_test("Get features by device", test_get_by_device)

    def test_empty_tenant():
        r = repo.get_latest_features("nonexistent_tenant", ASSET)
        assert r is None, "Should return None for unknown tenant"

    run_test("Returns None for unknown tenant", test_empty_tenant)

    db.close()


# ============================================================================
# 2. PredictionService Tests
# ============================================================================
def test_prediction_service():
    print("\n🔮 PredictionService Tests")
    from prediction_service import PredictionService

    db = SessionLocal()
    svc = PredictionService(db)

    def test_save():
        p = svc.save_prediction(
            tenant_id=TENANT, device_id=DEVICE, asset_id=ASSET,
            model_name="isolation_forest", model_version="v1",
            anomaly_score=0.42, predicted_status="warning",
        )
        assert p.prediction_id is not None
        assert p.review_status == "pending_review"
        assert p.reviewed is False
        return p

    pred = run_test("Save prediction", test_save)
    pred_id = pred.prediction_id if pred else None

    def test_get_by_id():
        p = svc.get_prediction_by_id(TENANT, pred_id)
        assert p is not None
        assert p.anomaly_score == 0.42
        return p

    if pred_id:
        run_test("Get prediction by ID", test_get_by_id)

    def test_pending():
        ps = svc.get_pending_predictions(TENANT, limit=10)
        assert len(ps) >= 1
        assert all(p.review_status == "pending_review" for p in ps)
        return ps

    run_test("Get pending predictions", test_pending)

    def test_update_status():
        p = svc.update_prediction_status(TENANT, pred_id, "reviewed", True)
        assert p.review_status == "reviewed"
        assert p.reviewed is True
        return p

    if pred_id:
        run_test("Update prediction status", test_update_status)

    def test_reviewed():
        ps = svc.get_reviewed_predictions(TENANT, limit=10)
        assert len(ps) >= 1
        return ps

    run_test("Get reviewed predictions", test_reviewed)

    def test_by_asset():
        ps = svc.get_predictions_by_asset(TENANT, ASSET, limit=10)
        assert len(ps) >= 1
        return ps

    run_test("Get predictions by asset", test_by_asset)

    def test_count():
        c = svc.count_predictions_by_status(TENANT, "reviewed")
        assert c >= 1
        return c

    run_test("Count predictions by status", test_count)

    def test_not_found():
        p = svc.get_prediction_by_id(TENANT, "nonexistent-id")
        assert p is None
        return p

    run_test("Returns None for unknown prediction", test_not_found)

    db.close()


# ============================================================================
# 3. ReviewService Tests
# ============================================================================
def test_review_service():
    print("\n📝 ReviewService Tests")
    from prediction_service import PredictionService
    from review_service import ReviewService

    db = SessionLocal()
    pred_svc = PredictionService(db)
    rev_svc = ReviewService(db)

    # Create predictions to review
    labels = ["normal", "warning", "critical"]
    pred_ids = []
    for label in labels:
        p = pred_svc.save_prediction(
            tenant_id=TENANT, device_id=DEVICE, asset_id=ASSET,
            model_name="isolation_forest", model_version="v1",
            anomaly_score=0.5, predicted_status=label,
        )
        pred_ids.append(p.prediction_id)

    def test_review():
        r = rev_svc.review_prediction(
            tenant_id=TENANT, prediction_id=pred_ids[0],
            device_id=DEVICE, asset_id=ASSET,
            model_prediction="normal", reviewed_label="normal",
            reviewed_by=USER_ENGINEER,
            review_comment="Looks good", is_training_eligible=True,
        )
        assert r.review_id is not None
        assert r.reviewed_label == "normal"
        return r

    run_test("Review prediction", test_review)

    # Review the rest
    for i, label in enumerate(labels[1:], 1):
        rev_svc.review_prediction(
            tenant_id=TENANT, prediction_id=pred_ids[i],
            device_id=DEVICE, asset_id=ASSET,
            model_prediction=label, reviewed_label=label,
            reviewed_by=USER_ENGINEER, is_training_eligible=True,
        )

    def test_get_reviews():
        rs = rev_svc.get_reviewed_predictions(TENANT, limit=10)
        assert len(rs) >= 3
        return rs

    run_test("Get reviewed predictions", test_get_reviews)

    def test_eligible():
        rs = rev_svc.get_training_eligible_reviews(TENANT)
        assert len(rs) >= 3
        return rs

    run_test("Get training-eligible reviews", test_eligible)

    def test_count():
        c = rev_svc.count_training_eligible_reviews(TENANT)
        assert c >= 3
        return c

    run_test("Count training-eligible reviews", test_count)

    def test_label_counts():
        counts = rev_svc.count_reviews_by_label(TENANT, training_eligible_only=True)
        assert "normal" in counts
        assert "warning" in counts
        assert "critical" in counts
        return counts

    run_test("Count reviews by label", test_label_counts)

    def test_by_asset():
        rs = rev_svc.get_reviews_by_asset(TENANT, ASSET, limit=10)
        assert len(rs) >= 3
        return rs

    run_test("Get reviews by asset", test_by_asset)

    def test_by_pred_id():
        r = rev_svc.get_review_by_prediction_id(TENANT, pred_ids[0])
        assert r is not None
        return r

    run_test("Get review by prediction ID", test_by_pred_id)

    def test_mark_ineligible():
        rs = rev_svc.get_training_eligible_reviews(TENANT, limit=1)
        r = rev_svc.mark_training_ineligible(TENANT, rs[0].review_id)
        assert r.is_training_eligible is False
        return r

    run_test("Mark review training-ineligible", test_mark_ineligible)

    db.close()


# ============================================================================
# 4. RetrainingService Tests
# ============================================================================
def test_retraining_service():
    print("\n🔄 RetrainingService Tests")
    from review_service import ReviewService
    from retraining_service import RetrainingService

    db = SessionLocal()
    rev_svc = ReviewService(db)
    retrain_svc = RetrainingService(db, rev_svc)

    def test_set_config():
        c = retrain_svc.set_config(
            tenant_id=TENANT, minimum_reviewed_records=2,
            auto_retrain_enabled=False, requires_manual_approval=True,
            updated_by=USER_ADMIN,
        )
        assert c.minimum_reviewed_records == 2
        return c

    run_test("Set retraining config", test_set_config)

    def test_get_config():
        c = retrain_svc.get_config(TENANT)
        assert c is not None
        assert c.minimum_reviewed_records == 2
        return c

    run_test("Get retraining config", test_get_config)

    def test_eligibility():
        r = retrain_svc.check_retraining_eligibility(TENANT)
        assert "eligible" in r
        assert "eligible_count" in r
        assert "label_distribution" in r
        return r

    elig = run_test("Check retraining eligibility", test_eligibility)

    def test_create_request():
        r = retrain_svc.create_retraining_request(
            tenant_id=TENANT, requested_by=USER_ENGINEER,
        )
        assert r.request_id is not None
        assert r.status == "created"
        return r

    req = run_test("Create retraining request", test_create_request)
    req_id = req.request_id if req else None

    def test_approve():
        r = retrain_svc.approve_retraining_request(
            tenant_id=TENANT, request_id=req_id, approved_by=USER_ADMIN,
        )
        assert r.status == "approved"
        assert r.approved_by == USER_ADMIN
        return r

    if req_id:
        run_test("Approve retraining request", test_approve)

    # Test reject flow
    req2 = retrain_svc.create_retraining_request(TENANT, USER_ENGINEER)

    def test_reject():
        r = retrain_svc.reject_retraining_request(
            tenant_id=TENANT, request_id=req2.request_id,
            rejection_reason="Insufficient data quality",
        )
        assert r.status == "rejected"
        return r

    run_test("Reject retraining request", test_reject)

    def test_update_status():
        req3 = retrain_svc.create_retraining_request(TENANT, USER_ENGINEER)
        r = retrain_svc.update_request_status(TENANT, req3.request_id, "in_progress")
        assert r.status == "in_progress"
        r = retrain_svc.update_request_status(TENANT, req3.request_id, "completed")
        assert r.status == "completed"
        assert r.completed_at is not None
        return r

    run_test("Update request status lifecycle", test_update_status)

    def test_get_request():
        r = retrain_svc.get_request(TENANT, req_id)
        assert r is not None
        return r

    if req_id:
        run_test("Get retraining request", test_get_request)

    def test_no_config():
        r = retrain_svc.check_retraining_eligibility("nonexistent_tenant")
        assert r["eligible"] is False
        assert "No retraining configuration" in r["reason"]

    run_test("Ineligible without config", test_no_config)

    db.close()


# ============================================================================
# 5. ModelVersionService Tests
# ============================================================================
def test_model_version_service():
    print("\n📦 ModelVersionService Tests")
    from model_version_service import ModelVersionService

    db = SessionLocal()
    svc = ModelVersionService(db)

    def test_create():
        mv = svc.create_model_version(
            tenant_id=TENANT, model_name="isolation_forest",
            model_version="v1", model_path="/models/if_v1.pkl",
            training_data_count=1000, validation_score=0.95,
        )
        assert mv.model_id is not None
        assert mv.deployment_status == "trained"
        return mv

    mv = run_test("Create model version", test_create)
    model_id = mv.model_id if mv else None

    def test_get():
        r = svc.get_model_version(TENANT, model_id, "v1")
        assert r is not None
        assert r.validation_score == 0.95
        return r

    if model_id:
        run_test("Get model version", test_get)

    def test_latest():
        r = svc.get_latest_model_version(TENANT, model_id)
        assert r is not None
        return r

    if model_id:
        run_test("Get latest model version", test_latest)

    def test_approve():
        r = svc.approve_model_version(TENANT, model_id, "v1", USER_ADMIN)
        assert r.deployment_status == "approved"
        assert r.approved_by == USER_ADMIN
        return r

    if model_id:
        run_test("Approve model version", test_approve)

    def test_deploy():
        r = svc.deploy_model_version(TENANT, model_id, "v1")
        assert r.deployment_status == "deployed"
        return r

    if model_id:
        run_test("Deploy model version", test_deploy)

    def test_active():
        r = svc.get_active_model(TENANT)
        assert r is not None
        assert r.model_id == model_id
        return r

    if model_id:
        run_test("Get active model", test_active)

    def test_by_status():
        r = svc.get_model_versions_by_status(TENANT, "deployed", limit=10)
        assert len(r) >= 1
        return r

    run_test("Get model versions by status", test_by_status)

    # Test reject flow
    mv2 = svc.create_model_version(TENANT, "isolation_forest", "v2")

    def test_reject():
        r = svc.reject_model_version(TENANT, mv2.model_id, "v2")
        assert r.deployment_status == "rejected"
        return r

    run_test("Reject model version", test_reject)

    def test_no_active():
        r = svc.get_active_model("nonexistent_tenant")
        assert r is None

    run_test("No active model for unknown tenant", test_no_active)

    db.close()


# ============================================================================
# 6. BearingAnomalyPredictor Tests (ML Model)
# ============================================================================
def test_prediction_module():
    print("\n🤖 BearingAnomalyPredictor (ML Model) Tests")
    from prediction_module import BearingAnomalyPredictor, get_maintenance_decision

    model_path = PROJECT_ROOT / "results" / "models" / "vibration_isolation_forest.pkl"
    feature_cols_path = PROJECT_ROOT / "results" / "models" / "feature_columns.json"
    thresholds_path = PROJECT_ROOT / "results" / "models" / "anomaly_thresholds.json"

    if not model_path.exists():
        print("  ⚠️  SKIPPED: Model file not found at", model_path)
        return

    def test_load():
        p = BearingAnomalyPredictor(
            model_path=model_path,
            feature_columns_path=feature_cols_path,
            thresholds_path=thresholds_path,
        )
        assert p.artifacts.model is not None
        assert len(p.artifacts.feature_columns) > 0
        return p

    predictor = run_test("Load ML model artifacts", test_load)
    if not predictor:
        return

    def test_predict():
        features = make_features("normal")
        result = predictor.predict(features, DEVICE, ASSET)
        assert "anomaly_score" in result
        assert "predicted_status" in result
        assert result["predicted_status"] in ("normal", "warning", "critical")
        assert "severity_level" in result
        assert "is_anomaly" in result
        assert "recommended_action" in result
        print(f"       → status={result['predicted_status']}, score={result['anomaly_score']:.4f}")
        return result

    run_test("Predict with normal features", test_predict)

    def test_predict_critical():
        features = make_features("critical")
        result = predictor.predict(features, DEVICE, ASSET)
        assert result["predicted_status"] in ("normal", "warning", "critical")
        print(f"       → status={result['predicted_status']}, score={result['anomaly_score']:.4f}")
        return result

    run_test("Predict with critical features", test_predict_critical)

    def test_bad_features():
        try:
            predictor.predict({"bad_col": 1.0}, DEVICE, ASSET)
            assert False, "Should have raised ValueError"
        except ValueError as e:
            assert "missing_columns" in str(e) or "do not match" in str(e)

    run_test("Reject invalid feature columns", test_bad_features)

    def test_maintenance_decisions():
        for status in ["normal", "warning", "critical", None, "unknown"]:
            d = get_maintenance_decision(status)
            assert "severity_level" in d
            assert "is_anomaly" in d
            assert "recommended_action" in d

    run_test("Maintenance decision mapping", test_maintenance_decisions)


# ============================================================================
# 7. End-to-End Workflow Test
# ============================================================================
def test_e2e_workflow():
    print("\n🔗 End-to-End Workflow Test")
    from timeseries_repository import TimeSeriesRepository
    from prediction_service import PredictionService
    from review_service import ReviewService
    from retraining_service import RetrainingService
    from model_version_service import ModelVersionService

    db = SessionLocal()
    ts_repo = TimeSeriesRepository(db)
    pred_svc = PredictionService(db)
    rev_svc = ReviewService(db)
    retrain_svc = RetrainingService(db, rev_svc)
    model_svc = ModelVersionService(db)

    e2e_tenant = "e2e_tenant"
    e2e_asset = "e2e_bearing_001"

    def step1():
        f = ts_repo.save_processed_features(
            tenant_id=e2e_tenant, device_id=DEVICE, asset_id=e2e_asset,
            features=make_features("warning"),
            feature_timestamp=datetime.now(timezone.utc),
        )
        assert f.id is not None
        return f

    run_test("Step 1: Ingest features", step1)

    def step2():
        f = ts_repo.get_latest_features(e2e_tenant, e2e_asset)
        assert f is not None
        p = pred_svc.save_prediction(
            tenant_id=e2e_tenant, device_id=DEVICE, asset_id=e2e_asset,
            model_name="isolation_forest", model_version="v1",
            anomaly_score=0.65, predicted_status="warning",
        )
        assert p.review_status == "pending_review"
        return p

    pred = run_test("Step 2: Generate prediction", step2)
    pid = pred.prediction_id if pred else None

    def step3():
        pending = pred_svc.get_pending_predictions(e2e_tenant)
        assert len(pending) >= 1
        return pending

    run_test("Step 3: List pending predictions", step3)

    def step4():
        r = rev_svc.review_prediction(
            tenant_id=e2e_tenant, prediction_id=pid,
            device_id=DEVICE, asset_id=e2e_asset,
            model_prediction="warning", reviewed_label="warning",
            reviewed_by=USER_ENGINEER, is_training_eligible=True,
        )
        pred_svc.update_prediction_status(e2e_tenant, pid, "reviewed", True)
        p = pred_svc.get_prediction_by_id(e2e_tenant, pid)
        assert p.reviewed is True
        return r

    if pid:
        run_test("Step 4: Human review", step4)

    # Add more reviewed predictions for all labels
    for label in ["normal", "critical"]:
        p = pred_svc.save_prediction(
            tenant_id=e2e_tenant, device_id=DEVICE, asset_id=e2e_asset,
            model_name="isolation_forest", model_version="v1",
            anomaly_score=0.5, predicted_status=label,
        )
        rev_svc.review_prediction(
            tenant_id=e2e_tenant, prediction_id=p.prediction_id,
            device_id=DEVICE, asset_id=e2e_asset,
            model_prediction=label, reviewed_label=label,
            reviewed_by=USER_ENGINEER, is_training_eligible=True,
        )

    def step5():
        retrain_svc.set_config(e2e_tenant, minimum_reviewed_records=2, updated_by=USER_ADMIN)
        elig = retrain_svc.check_retraining_eligibility(e2e_tenant)
        assert elig["eligible"] is True, f"Not eligible: {elig.get('reason')}"
        print(f"       → eligible_count={elig['eligible_count']}, labels={elig['label_distribution']}")
        return elig

    run_test("Step 5: Check retraining eligibility", step5)

    def step6():
        req = retrain_svc.create_retraining_request(e2e_tenant, USER_ENGINEER)
        assert req.status == "created"
        retrain_svc.approve_retraining_request(e2e_tenant, req.request_id, USER_ADMIN)
        req = retrain_svc.get_request(e2e_tenant, req.request_id)
        assert req.status == "approved"
        return req

    run_test("Step 6: Request & approve retraining", step6)

    def step7():
        mv = model_svc.create_model_version(
            tenant_id=e2e_tenant, model_name="isolation_forest",
            model_version="v2", validation_score=0.97,
            training_data_count=3,
        )
        model_svc.approve_model_version(e2e_tenant, mv.model_id, "v2", USER_ADMIN)
        model_svc.deploy_model_version(e2e_tenant, mv.model_id, "v2")
        active = model_svc.get_active_model(e2e_tenant)
        assert active is not None
        assert active.deployment_status == "deployed"
        print(f"       → deployed model={active.model_id} v{active.model_version}")
        return active

    run_test("Step 7: Deploy new model version", step7)

    db.close()


# ============================================================================
# Main
# ============================================================================
def main():
    print("=" * 70)
    print("🧪 PredictiveOps ML Services — Comprehensive Test Suite")
    print("=" * 70)

    test_timeseries_repository()
    test_prediction_service()
    test_review_service()
    test_retraining_service()
    test_model_version_service()
    test_prediction_module()
    test_e2e_workflow()

    # Summary
    total = RESULTS["passed"] + RESULTS["failed"]
    print("\n" + "=" * 70)
    print(f"📊 Results: {RESULTS['passed']}/{total} passed, {RESULTS['failed']} failed")
    if RESULTS["errors"]:
        print("\n❌ Failed tests:")
        for name, err in RESULTS["errors"]:
            print(f"   • {name}: {err}")
    print("=" * 70)

    sys.exit(0 if RESULTS["failed"] == 0 else 1)


if __name__ == "__main__":
    main()
