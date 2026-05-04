"""
Integration test and usage examples for the ML Pipeline API.
Demonstrates the complete human-in-the-loop workflow.
"""

from datetime import datetime, timezone

import requests

# Configuration
API_BASE = "http://localhost:8000"
TENANT_ID = "demo_tenant"
DEVICE_ID = "demo_device_001"
ASSET_ID = "bearing_motor_001"

# Users
ENGINEER_1 = "maintenance_engineer_01"
ADMIN_1 = "admin_user_01"

# Sample features
SAMPLE_FEATURES = {
    "rms": 2.5,
    "kurtosis": 3.8,
    "crest_factor": 4.2,
    "spectral_energy": 156.7,
    "temperature": 45.3,
}

NORMAL_FEATURES = {
    "rms": 1.2,
    "kurtosis": 2.1,
    "crest_factor": 2.8,
    "spectral_energy": 95.2,
    "temperature": 38.5,
}

CRITICAL_FEATURES = {
    "rms": 5.8,
    "kurtosis": 6.2,
    "crest_factor": 8.5,
    "spectral_energy": 298.3,
    "temperature": 62.1,
}


def test_health_check():
    """Test health endpoint."""
    print("\n=== Health Check ===")
    response = requests.get(f"{API_BASE}/health")
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200


def test_save_features():
    """Test saving processed features."""
    print("\n=== Save Processed Features ===")

    payload = {
        "tenant_id": TENANT_ID,
        "device_id": DEVICE_ID,
        "asset_id": ASSET_ID,
        "features": SAMPLE_FEATURES,
        "feature_timestamp": datetime.now(timezone.utc).isoformat(),
    }

    response = requests.post(f"{API_BASE}/processed-features", json=payload)
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_get_latest_features():
    """Test retrieving latest features."""
    print("\n=== Get Latest Features ===")

    response = requests.get(
        f"{API_BASE}/processed-features/latest/{ASSET_ID}",
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_predict():
    """Test ML prediction."""
    print("\n=== Run Prediction ===")

    payload = {
        "tenant_id": TENANT_ID,
        "device_id": DEVICE_ID,
        "asset_id": ASSET_ID,
        "features": SAMPLE_FEATURES,
    }

    response = requests.post(f"{API_BASE}/predict", json=payload)
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_get_pending_predictions():
    """Test getting pending predictions."""
    print("\n=== Get Pending Predictions ===")

    response = requests.get(
        f"{API_BASE}/predictions/pending",
        params={"tenant_id": TENANT_ID, "limit": 10},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_review_prediction(prediction_id):
    """Test reviewing a prediction."""
    print(f"\n=== Review Prediction: {prediction_id} ===")

    payload = {
        "device_id": DEVICE_ID,
        "asset_id": ASSET_ID,
        "reviewed_label": "warning",
        "reviewed_by": ENGINEER_1,
        "review_comment": "High vibration detected. Monitor closely.",
        "is_training_eligible": True,
    }

    response = requests.post(
        f"{API_BASE}/predictions/{prediction_id}/review",
        json=payload,
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_get_reviews():
    """Test getting reviews."""
    print("\n=== Get Reviews ===")

    response = requests.get(
        f"{API_BASE}/reviews",
        params={"tenant_id": TENANT_ID, "limit": 10},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_training_eligible_count():
    """Test getting count of training-eligible reviews."""
    print("\n=== Training Eligible Count ===")

    response = requests.get(
        f"{API_BASE}/reviews/training-eligible-count",
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_retraining_config():
    """Test retraining configuration."""
    print("\n=== Get Retraining Config ===")

    response = requests.get(
        f"{API_BASE}/retraining/config",
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    if response.status_code == 404:
        print("Config not found, setting default...")

        # Set default config
        payload = {
            "minimum_reviewed_records": 100,  # Lower for testing
            "auto_retrain_enabled": False,
            "requires_manual_approval": True,
        }

        response = requests.put(
            f"{API_BASE}/retraining/config",
            json=payload,
            params={"tenant_id": TENANT_ID, "updated_by": ADMIN_1},
        )
        print(f"Set status: {response.status_code}")
        print(f"Response: {response.json()}")
    else:
        print(f"Response: {response.json()}")

    assert response.status_code == 200
    return response.json()


def test_retraining_eligibility():
    """Test checking retraining eligibility."""
    print("\n=== Check Retraining Eligibility ===")

    response = requests.get(
        f"{API_BASE}/retraining/eligibility",
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_create_retraining_request():
    """Test creating retraining request."""
    print("\n=== Create Retraining Request ===")

    response = requests.post(
        f"{API_BASE}/retraining/request",
        params={"tenant_id": TENANT_ID, "requested_by": ENGINEER_1},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_approve_retraining(request_id):
    """Test approving retraining request."""
    print(f"\n=== Approve Retraining: {request_id} ===")

    response = requests.post(
        f"{API_BASE}/retraining/{request_id}/approve",
        params={"tenant_id": TENANT_ID, "approved_by": ADMIN_1},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_model_versions():
    """Test getting model versions."""
    print("\n=== Get Model Versions ===")

    response = requests.get(
        f"{API_BASE}/models/versions",
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    return response.json()


def test_get_active_model():
    """Test getting active model."""
    print("\n=== Get Active Model ===")

    response = requests.get(
        f"{API_BASE}/models/active",
        params={"tenant_id": TENANT_ID},
    )
    print(f"Status: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code in [200, 404]  # Might not be set yet
    return response.json() if response.status_code == 200 else None


def run_workflow():
    """Run complete human-in-the-loop workflow."""
    print("=" * 80)
    print("COMPLETE HUMAN-IN-THE-LOOP ML PIPELINE WORKFLOW")
    print("=" * 80)

    # Step 1: Health check
    test_health_check()

    # Step 2: Configure retraining
    test_retraining_config()

    # Step 3: Save processed features
    features_response = test_save_features()

    # Step 4: Get latest features
    test_get_latest_features()

    # Step 5: Run prediction
    prediction = test_predict()
    prediction_id = prediction["prediction_id"]

    # Step 6: Get pending predictions
    pending = test_get_pending_predictions()
    print(f"Found {len(pending)} pending predictions")

    # Step 7: Get training eligible count before review
    before_count = test_training_eligible_count()

    # Step 8: Review the prediction
    review = test_review_prediction(prediction_id)

    # Step 9: Get training eligible count after review
    after_count = test_training_eligible_count()

    # Step 10: Get all reviews
    test_get_reviews()

    # Step 11: Check retraining eligibility
    eligibility = test_retraining_eligibility()

    # Step 12: Create retraining request (if eligible)
    if eligibility.get("eligible"):
        print("\n✓ Retraining is ELIGIBLE")
        retrain_req = test_create_retraining_request()
        request_id = retrain_req["request_id"]

        # Step 13: Approve retraining
        test_approve_retraining(request_id)
    else:
        print(f"\n✗ Retraining not eligible: {eligibility.get('reason')}")
        print(f"   Have {eligibility.get('eligible_count')} reviews, "
              f"need {eligibility.get('required_count')}")

    # Step 14: Get model versions
    test_model_versions()

    # Step 15: Get active model
    test_get_active_model()

    print("\n" + "=" * 80)
    print("WORKFLOW COMPLETE")
    print("=" * 80)


if __name__ == "__main__":
    run_workflow()
