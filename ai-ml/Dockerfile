# --------- Stage 1: Build ---------
FROM python:3.12-slim AS builder
WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# --------- Stage 2: Runtime ---------
FROM python:3.12-slim
WORKDIR /app

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH=/root/.local/bin:$PATH

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Copy Python dependencies from builder
COPY --from=builder /root/.local /root/.local

# Copy application code and model artifacts
COPY src/prediction_api.py src/prediction_module.py src/kafka_producer.py \
     src/db_models.py src/prediction_service.py src/review_service.py \
     src/retraining_service.py src/model_version_service.py \
     src/timeseries_repository.py src/alert_notifier.py src/prediction_persister.py ./
COPY artifacts/ /artifacts/
COPY alert-notifier-entrypoint.sh prediction-persister-entrypoint.sh /

EXPOSE 8003

CMD ["sh", "-c", "python kafka_producer.py & uvicorn prediction_api:app --host 0.0.0.0 --port 8003"]


