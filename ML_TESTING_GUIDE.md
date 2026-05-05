# ML Pipeline Testing & Verification Guide

**Project:** Predictive Maintenance System  
**Date:** May 2, 2026  
**Status:** Post-Alignment Verification

---

## Overview

This guide provides step-by-step instructions to test and verify the ML pipeline after alignment fixes.

---

## Prerequisites

### 1. Services Running

Verify all required services are running:

```bash
# Check database
psql -h localhost -p 5433 -U user -d predictions -c "SELECT 1"

# Check Python API
curl http://localhost:8000/health

# Check Go service (if needed)
curl http://localhost:8080/health
```

### 2. Virtual Environment

Activate Python environment:

```bash
cd ai-ml
source .venv/bin/activate
```

### 3. Dependencies Installed

```bash
cd ai-ml
pip install -r requirements.txt
```

---

## Test 1: Health Check

**Objective:** Verify API is running  
**Time:** < 1 minute

```bash
curl http://localhost:8000/health
```

**Expected Response:**
```json
{
  "status": "ok",
  "service": "bearing_anomaly_prediction",
  "version": "1.0.0"
}
```

**Pass Criteria:** HTTP 200, status="ok"

---

## Test 2: Save Processed Features

**Objective:** Verify time-series feature storage works  
**Time:** 1-2 minutes

```bash
curl -X POST http://localhost:8000/processed-features \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_1",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5,
      "kurtosis": 3.8,
      "crest_factor": 4.2,
      "spectral_energy": 156.7,
      "temperature": 45.3
    },
    "feature_timestamp": "2024-05-02T10:00:00Z"
  }'
```

**Expected Response:**
```json
{
  "id": 1,
  "tenant_id": "tenant_1",
  "device_id": "device_001",
  "asset_id": "bearing_001",
  "features": {...},
  "feature_version": "v1",
  "created_at": "2024-05-02T10:00:00Z",
  "feature_timestamp": "2024-05-02T10:00:00Z"
}
```

**Pass Criteria:**
- HTTP 200
- Record created in database
- All fields preserved correctly

---

## Test 3: Get Latest Features

**Objective:** Verify feature retrieval from time-series storage  
**Time:** 1 minute

```bash
curl "http://localhost:8000/processed-features/latest/bearing_001?tenant_id=tenant_1"
```

**Expected Response:**
```json
{
  "id": 1,
  "tenant_id": "tenant_1",
  "device_id": "device_001",
  "asset_id": "bearing_001",
  "features": {
    "rms": 2.5,
    "kurtosis": 3.8,
    "crest_factor": 4.2,
    "spectral_energy": 156.7,
    "temperature": 45.3
  },
  "feature_version": "v1",
  "created_at": "2024-05-02T10:00:00Z",
  "feature_timestamp": "2024-05-02T10:00:00Z"
}
```

**Pass Criteria:**
- HTTP 200
- Returns latest features (highest created_at)
- Correct asset_id and tenant_id

---

## Test 4: Run Prediction

**Objective:** Verify ML model inference and prediction storage  
**Time:** 2-3 minutes

```bash
curl -X POST http://localhost:8000/predict \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_1",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5,
      "kurtosis": 3.8,
      "crest_factor": 4.2,
      "spectral_energy": 156.7,
      "temperature": 45.3
    }
  }'
```

**Expected Response:**
```json
{
  "prediction_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "tenant_1",
  "device_id": "device_001",
  "asset_id": "bearing_001",
  "model_name": "vibration_isolation_forest",
  "model_version": "v1",
  "anomaly_score": 0.1827,
  "predicted_status": "warning",
  "review_status": "pending_review",
  "reviewed": false,
  "timestamp": "2024-05-02T10:15:30Z",
  "severity_level": 2,
  "is_anomaly": true,
  "recommended_action": "Early abnormal behavior detected..."
}
```

**Pass Criteria:**
- HTTP 200
- ✅ `review_status` = "pending_review"
- ✅ `reviewed` = false
- ✅ `anomaly_score` between 0 and 1
- ✅ `predicted_status` in [normal, warning, critical]
- ✅ `severity_level` correct (normal=1, warning=2, critical=3)
- ✅ Record stored in predictions table
- **CRITICAL:** Every new prediction must have review_status=pending_review

---

## Test 5: Get Pending Predictions

**Objective:** Verify we can retrieve unreviewed predictions  
**Time:** 1 minute

```bash
curl "http://localhost:8000/predictions/pending?tenant_id=tenant_1&limit=10"
```

**Expected Response:**
```json
[
  {
    "prediction_id": "550e8400-e29b-41d4-a716-446655440000",
    "tenant_id": "tenant_1",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "model_name": "vibration_isolation_forest",
    "model_version": "v1",
    "anomaly_score": 0.1827,
    "predicted_status": "warning",
    "review_status": "pending_review",
    "reviewed": false,
    "timestamp": "2024-05-02T10:15:30Z"
  }
]
```

**Pass Criteria:**
- HTTP 200
- ✅ All returned predictions have `review_status` = "pending_review"
- ✅ All returned predictions have `reviewed` = false
- ✅ Correct limit applied

---

## Test 6: Review Prediction

**Objective:** Verify human review workflow  
**Time:** 2 minutes

```bash
# Get the prediction_id from Test 4
PRED_ID="550e8400-e29b-41d4-a716-446655440000"

curl -X POST "http://localhost:8000/predictions/$PRED_ID/review?tenant_id=tenant_1" \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "reviewed_label": "warning",
    "reviewed_by": "engineer_1",
    "review_comment": "Model predicted correctly. Vibration trend confirms warning status.",
    "is_training_eligible": true
  }'
```

**Expected Response:**
```json
{
  "review_id": "550e8400-e29b-41d4-a716-446655440001",
  "prediction_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "tenant_1",
  "device_id": "device_001",
  "asset_id": "bearing_001",
  "model_prediction": "warning",
  "reviewed_label": "warning",
  "reviewed_by": "engineer_1",
  "review_comment": "Model predicted correctly...",
  "is_training_eligible": true,
  "reviewed_at": "2024-05-02T10:20:00Z"
}
```

**Pass Criteria:**
- HTTP 200
- ✅ Review created with all fields
- ✅ `is_training_eligible` = true
- ✅ `reviewed_by` = engineer_1
- ✅ Original prediction updated to `reviewed` = true, `review_status` = "reviewed"
- ✅ PredictionReview record created in database

**Verification Query:**
```bash
# Verify prediction status updated
curl "http://localhost:8000/predictions/$PRED_ID?tenant_id=tenant_1" | jq '.reviewed, .review_status'
# Should return: true, "reviewed"
```

---

## Test 7: Verify Review Storage

**Objective:** Confirm reviews are stored separately from predictions  
**Time:** 1 minute

```bash
curl "http://localhost:8000/reviews?tenant_id=tenant_1"
```

**Expected Response:**
```json
[
  {
    "review_id": "550e8400-e29b-41d4-a716-446655440001",
    "prediction_id": "550e8400-e29b-41d4-a716-446655440000",
    "tenant_id": "tenant_1",
    ...
  }
]
```

**Pass Criteria:**
- HTTP 200
- ✅ Reviews are in separate table
- ✅ Each review linked to prediction via prediction_id
- ✅ Only training_eligible reviews should be counted for retraining

---

## Test 8: Count Training Eligible Reviews

**Objective:** Verify retraining eligibility counter  
**Time:** 1 minute

```bash
curl "http://localhost:8000/reviews/training-eligible-count?tenant_id=tenant_1"
```

**Expected Response:**
```json
{
  "count": 1
}
```

**Pass Criteria:**
- HTTP 200
- ✅ Count includes only reviews with `is_training_eligible` = true
- ✅ Count increments after each eligible review

---

## Test 9: Check Retraining Eligibility

**Objective:** Verify retraining eligibility logic  
**Time:** 2 minutes

### Scenario A: Not Enough Reviews

```bash
curl "http://localhost:8000/retraining/eligibility?tenant_id=tenant_1"
```

**Expected Response (after 1 review):**
```json
{
  "eligible": false,
  "reason": "Insufficient training data. Have 1, need 500",
  "eligible_count": 1,
  "required_count": 500,
  "label_distribution": {
    "warning": 1
  }
}
```

**Pass Criteria:**
- ✅ `eligible` = false
- ✅ Reason indicates threshold not met
- ✅ Label distribution shown

### Scenario B: Enough Reviews but Missing Labels

After adding many reviews but only of "warning" status:

**Expected Response:**
```json
{
  "eligible": false,
  "reason": "Missing label types: critical, normal",
  "eligible_count": 500,
  "required_count": 500,
  "label_distribution": {
    "warning": 500
  },
  "missing_labels": ["critical", "normal"]
}
```

**Pass Criteria:**
- ✅ `eligible` = false
- ✅ Missing labels identified
- ✅ Requires samples of ALL label types

### Scenario C: Fully Eligible

After adding reviews with all label types and reaching threshold:

**Expected Response:**
```json
{
  "eligible": true,
  "eligible_count": 500,
  "required_count": 500,
  "label_distribution": {
    "normal": 150,
    "warning": 200,
    "critical": 150
  }
}
```

**Pass Criteria:**
- ✅ `eligible` = true
- ✅ Count >= threshold
- ✅ All label types present with at least some samples

---

## Test 10: Retraining Configuration

**Objective:** Verify configuration management  
**Time:** 2 minutes

### Get Default Config

```bash
curl "http://localhost:8000/retraining/config?tenant_id=tenant_1"
```

**Expected Response:**
```json
{
  "tenant_id": "tenant_1",
  "minimum_reviewed_records": 500,
  "auto_retrain_enabled": false,
  "requires_manual_approval": true
}
```

### Update Config

```bash
curl -X PUT "http://localhost:8000/retraining/config?tenant_id=tenant_1&updated_by=admin_1" \
  -H "Content-Type: application/json" \
  -d '{
    "minimum_reviewed_records": 100,
    "auto_retrain_enabled": false,
    "requires_manual_approval": true
  }'
```

**Expected Response:**
```json
{
  "tenant_id": "tenant_1",
  "minimum_reviewed_records": 100,
  "auto_retrain_enabled": false,
  "requires_manual_approval": true
}
```

**Pass Criteria:**
- ✅ Configuration persisted
- ✅ Per-tenant config (different tenants can have different thresholds)
- ✅ Can be updated at runtime

---

## Test 11: Retraining Request Workflow

**Objective:** Verify retraining request lifecycle  
**Time:** 3 minutes

### Step 1: Create Request

```bash
curl -X POST "http://localhost:8000/retraining/request?tenant_id=tenant_1&requested_by=engineer_1"
```

**Expected Response:**
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440002",
  "tenant_id": "tenant_1",
  "status": "created",
  "training_data_count": 500,
  "requested_by": "engineer_1",
  "approved_by": null,
  "created_at": "2024-05-02T10:30:00Z",
  "updated_at": "2024-05-02T10:30:00Z"
}
```

**Pass Criteria:**
- ✅ Request created with `status` = "created"
- ✅ `requested_by` = engineer_1
- ✅ `approved_by` = null (awaiting approval)

### Step 2: Approve Request

```bash
REQUEST_ID="550e8400-e29b-41d4-a716-446655440002"

curl -X POST "http://localhost:8000/retraining/$REQUEST_ID/approve?tenant_id=tenant_1&approved_by=admin_1"
```

**Expected Response:**
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440002",
  "tenant_id": "tenant_1",
  "status": "approved",
  "training_data_count": 500,
  "requested_by": "engineer_1",
  "approved_by": "admin_1",
  "created_at": "2024-05-02T10:30:00Z",
  "updated_at": "2024-05-02T10:35:00Z"
}
```

**Pass Criteria:**
- ✅ Request status changed to "approved"
- ✅ `approved_by` = admin_1
- ✅ Ready for external retraining job to pick up

---

## Test 12: Model Versioning

**Objective:** Verify model version tracking  
**Time:** 3 minutes

### Get Active Model

```bash
curl "http://localhost:8000/models/active?tenant_id=tenant_1"
```

**Expected Response:**
```json
{
  "model_id": "vibration_isolation_forest_abc123",
  "model_name": "vibration_isolation_forest",
  "model_version": "v1",
  "deployment_status": "deployed",
  "training_data_count": 10000,
  "training_date": "2024-05-01T12:00:00Z",
  "validation_score": 0.92,
  "approved_by": "admin_1",
  "created_at": "2024-05-01T12:00:00Z"
}
```

**Pass Criteria:**
- ✅ Returns currently active model
- ✅ `deployment_status` = "deployed"

### Get All Versions

```bash
curl "http://localhost:8000/models/versions?tenant_id=tenant_1"
```

**Expected Response:**
```json
[
  {
    "model_id": "vibration_isolation_forest_abc123",
    "model_name": "vibration_isolation_forest",
    "model_version": "v1",
    "deployment_status": "deployed",
    "training_data_count": 10000,
    "training_date": "2024-05-01T12:00:00Z",
    "validation_score": 0.92,
    "approved_by": "admin_1",
    "created_at": "2024-05-01T12:00:00Z"
  }
]
```

**Pass Criteria:**
- ✅ Lists all versions for tenant
- ✅ Each version has proper metadata
- ✅ Deployment status tracked

---

## Test 13: Integration Test - Full Workflow

**Objective:** End-to-end pipeline test  
**Time:** 10 minutes

Run the integration test:

```bash
cd ai-ml
python workflow_integration_test.py
```

**Expected Output:**
```
=== Health Check ===
Status: 200

=== Save Processed Features ===
Status: 200

=== Get Latest Features ===
Status: 200

=== Run Prediction ===
Status: 200

=== Get Pending Predictions ===
Status: 200

=== Review Prediction ===
Status: 200

=== Get Reviews ===
Status: 200

=== Check Training Eligible Count ===
Status: 200

=== Get Retraining Config ===
Status: 200

=== Check Retraining Eligibility ===
Status: 200

=== Create Retraining Request ===
Status: 200

=== Approve Retraining Request ===
Status: 200

=== Get Model Versions ===
Status: 200

✅ All tests passed
```

**Pass Criteria:**
- ✅ All HTTP 200 responses
- ✅ No errors in output
- ✅ Complete workflow executes

---

## Test 14: Database Verification

**Objective:** Verify data is stored correctly in database  
**Time:** 2 minutes

### Connect to Database

```bash
psql -h localhost -p 5433 -U predictions_user -d predictions
```

### Verify Tables Exist

```sql
-- Check all pipeline tables exist
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name LIKE '%features%' 
   OR table_name LIKE 'prediction%'
   OR table_name LIKE 'retraining%'
   OR table_name LIKE 'model%';
```

**Expected Tables:**
- ✅ processed_features
- ✅ predictions
- ✅ prediction_reviews
- ✅ retraining_configs
- ✅ retraining_requests
- ✅ model_versions
- ✅ active_model_versions

### Verify Data

```sql
-- Check features
SELECT COUNT(*) as feature_count FROM processed_features;

-- Check predictions (should show review_status = pending_review)
SELECT COUNT(*) as pending_predictions 
FROM predictions 
WHERE review_status = 'pending_review';

-- Check reviews (only training_eligible counted)
SELECT COUNT(*) as eligible_reviews 
FROM prediction_reviews 
WHERE is_training_eligible = true;

-- Check model versions
SELECT model_id, model_version, deployment_status 
FROM model_versions 
ORDER BY created_at DESC;
```

---

## Test 15: Error Handling

**Objective:** Verify error messages are consistent  
**Time:** 2 minutes

### Test Missing Prediction

```bash
curl "http://localhost:8000/predictions/invalid_id?tenant_id=tenant_1"
```

**Expected:**
- ✅ HTTP 404
- ✅ Message: "Prediction not found"

### Test Invalid Features

```bash
curl -X POST http://localhost:8000/predict \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant_1",
    "device_id": "device_001",
    "asset_id": "bearing_001",
    "features": {
      "rms": 2.5
    }
  }'
```

**Expected:**
- ✅ HTTP 400/500 (depending on implementation)
- ✅ Error message: "Error running prediction: ..."

**Pass Criteria:**
- ✅ Consistent error message format
- ✅ Appropriate HTTP status codes
- ✅ Clear error descriptions

---

## Test Summary Checklist

- [ ] Test 1: Health Check ✅
- [ ] Test 2: Save Features ✅
- [ ] Test 3: Get Latest Features ✅
- [ ] Test 4: Run Prediction (review_status=pending_review) ✅
- [ ] Test 5: Get Pending Predictions ✅
- [ ] Test 6: Review Prediction ✅
- [ ] Test 7: Verify Review Storage ✅
- [ ] Test 8: Count Training Eligible ✅
- [ ] Test 9: Check Retraining Eligibility ✅
- [ ] Test 10: Configuration Management ✅
- [ ] Test 11: Retraining Workflow ✅
- [ ] Test 12: Model Versioning ✅
- [ ] Test 13: Integration Test ✅
- [ ] Test 14: Database Verification ✅
- [ ] Test 15: Error Handling ✅

**Overall Result:** ✅ PASS/❌ FAIL

---

## Critical Success Criteria

These MUST pass for pipeline to be considered aligned:

1. ✅ Every new prediction has `review_status` = "pending_review"
2. ✅ Every new prediction has `reviewed` = false
3. ✅ Only reviews with `is_training_eligible` = true are used
4. ✅ Retraining requires all label types (normal, warning, critical)
5. ✅ Features and predictions stored in separate tables
6. ✅ Multi-tenancy enforced (different tenants isolated)
7. ✅ All timestamps in UTC
8. ✅ Error messages consistent and informative

---

## Troubleshooting

### Issue: "No features found"

```bash
# Make sure features are saved first
curl -X POST http://localhost:8000/processed-features \
  -H "Content-Type: application/json" \
  -d '{...}'
```

### Issue: "Prediction not found" after review

The prediction_id must match exactly. Get it from the predict response:
```bash
PRED_ID=$(curl -s http://localhost:8000/predict ... | jq -r '.prediction_id')
curl -X POST "http://localhost:8000/predictions/$PRED_ID/review?tenant_id=tenant_1" -d '{...}'
```

### Issue: Retraining not eligible

Check the eligibility response to see what's missing:
```bash
curl "http://localhost:8000/retraining/eligibility?tenant_id=tenant_1" | jq '.'
```

Common issues:
- Not enough reviews (< 500 by default)
- Missing label types (need normal, warning, AND critical)
- Reviews marked as not training_eligible

---

## Performance Notes

- Features retrieved by latest created_at (indexed)
- Predictions filtered by tenant_id and review_status (indexed)
- Reviews queried by is_training_eligible (indexed)
- All multi-tenant queries O(1) via tenant_id index

---

## Next Steps After Verification

1. ✅ All tests pass → Move to production
2. ✅ Review alignment report for any edge cases
3. ⏭️ Implement external retraining job (watches approved requests)
4. ⏭️ Integrate dashboard UI (calls review endpoints)
5. ⏭️ Add monitoring and alerting

---

**Last Updated:** 2026-05-02  
**Next Review:** After first retraining job completes
