FROM python:3.11-slim

WORKDIR /app

# Install system dependencies for psycopg2 and scikit-learn
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY src/ ./src/
COPY artifacts/ ./artifacts/

# Copy entrypoint scripts
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY alert-notifier-entrypoint.sh /alert-notifier-entrypoint.sh
COPY prediction-persister-entrypoint.sh /prediction-persister-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh /alert-notifier-entrypoint.sh /prediction-persister-entrypoint.sh

# Set environment variables
ENV PYTHONPATH=/app/src
ENV DATABASE_URL=postgresql://postgres:postgres@postgres:5432/predictions
ENV API_HOST=0.0.0.0
ENV API_PORT=8000
ENV MODEL_DIR=/app/artifacts/models

# Expose port
EXPOSE 8000

# Run the API with startup wait for postgres
ENTRYPOINT ["/docker-entrypoint.sh"]
