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
# COPY prediction_api.py prediction_module.py ./
COPY prediction_api.py prediction_module.py kafka_consumer.py ./
COPY results/ ./results/

EXPOSE 8000

# Run the FastAPI app
# CMD ["uvicorn", "prediction_api:app", "--host", "0.0.0.0", "--port", "8000"]
CMD ["sh", "-c", "python kafka_consumer.py & uvicorn prediction_api:app --host 0.0.0.0 --port 8000"]


