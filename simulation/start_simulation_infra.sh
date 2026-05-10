#!/bin/bash
# Start required Docker containers for simulation pipeline
# Simulation -> MQTT -> Ingestion -> Kafka

set -e

echo "=== Starting Simulation Infrastructure ==="
echo ""

# Check if docker compose is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed"
    exit 1
fi

# Required containers for complete pipeline
REQUIRED_SERVICES="mosquitto postgres redis kafka kafka-ui ingestion-service event-processing-service ml-service alert-notifier prediction-persister prometheus"

echo "[1/8] Starting base infrastructure (mosquitto, postgres, redis, kafka, kafka-ui, prometheus)..."
docker compose up -d mosquitto postgres redis kafka kafka-ui prometheus

echo ""
echo "[2/8] Waiting for base services to be healthy..."
MAX_WAIT=60
WAITED=0

while [ $WAITED -lt $MAX_WAIT ]; do
    HEALTHY=true
    
    for service in mosquitto postgres redis kafka-ui prometheus; do
        STATUS=$(docker compose ps -q $service 2>/dev/null | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
        if [ "$STATUS" != "healthy" ] && [ "$STATUS" != "running" ]; then
            HEALTHY=false
            break
        fi
    done
    
    if [ "$HEALTHY" = true ]; then
        echo "      All base services are healthy!"
        break
    fi
    
    echo "      Waiting... ($WAITED/$MAX_WAIT seconds)"
    sleep 2
    WAITED=$((WAITED + 2))
done

if [ "$HEALTHY" = false ]; then
    echo "Warning: Some services may not be healthy yet, but continuing..."
fi

echo ""
echo "[3/8] Creating Kafka topics..."
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
    --create --bootstrap-server localhost:9092 \
    --topic device-events --partitions 1 --replication-factor 1 2>/dev/null || echo "      Topic device-events already exists or will be created"
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
    --create --bootstrap-server localhost:9092 \
    --topic processed-data --partitions 1 --replication-factor 1 2>/dev/null || echo "      Topic processed-data already exists or will be created"
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
    --create --bootstrap-server localhost:9092 \
    --topic predictions --partitions 1 --replication-factor 1 2>/dev/null || echo "      Topic predictions already exists or will be created"
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
    --create --bootstrap-server localhost:9092 \
    --topic alerts --partitions 1 --replication-factor 1 2>/dev/null || echo "      Topic alerts already exists or will be created"

echo ""
echo "[4/8] Starting ingestion-service..."
docker compose up -d ingestion-service

echo ""
echo "[5/8] Verifying ingestion service..."
MAX_RETRY=10
RETRY=0
while [ $RETRY -lt $MAX_RETRY ]; do
    if curl -s http://localhost:2500/health > /dev/null 2>&1; then
        echo "      Ingestion service is ready!"
        break
    fi
    echo "      Waiting for ingestion service... ($RETRY/$MAX_RETRY)"
    sleep 1
    RETRY=$((RETRY + 1))
done

echo ""
echo "[6/8] Starting processing services (event-processing-service, ml-service, alert-notifier, prediction-persister)..."
docker compose up -d event-processing-service ml-service alert-notifier prediction-persister

echo ""
echo "[7/8] Verifying kafka-ui..."
sleep 2
if curl -s http://localhost:8085 > /dev/null 2>&1; then
    echo "      Kafka UI is ready!"
else
    echo "      Warning: Kafka UI may still be starting..."
fi

echo ""
echo "[8/8] Verifying ML service..."
MAX_RETRY=10
RETRY=0
while [ $RETRY -lt $MAX_RETRY ]; do
    if curl -s http://localhost:8004/health > /dev/null 2>&1; then
        echo "      ML service is ready!"
        break
    fi
    echo "      Waiting for ML service... ($RETRY/$MAX_RETRY)"
    sleep 2
    RETRY=$((RETRY + 1))
done

echo ""
echo "=== Infrastructure Ready ==="
echo ""
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" | grep -E "(mosquitto|postgres|redis|kafka|ingestion|kafka-ui|event-processing|ml-service|alert-notifier|prediction-persister|prometheus)" || true
echo ""
echo "Services:"
echo "  - Mosquitto (MQTT):           localhost:8883"
echo "  - Ingestion API:              http://localhost:2500"
echo "  - Kafka:                      localhost:9092"
echo "  - Kafka UI:                   http://localhost:8085"
echo "  - Event Processing Service:    http://localhost:8001"
echo "  - ML Service:                 http://localhost:8004"
echo "  - Prometheus:                 http://localhost:9090"
echo "  - PostgreSQL:                 localhost:5433"
echo ""
echo "Run simulation:"
echo "  cd simulation && ./run_simulation.sh 1 new 100"
echo ""
echo "Watch Kafka data:"
echo "  docker compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic device-events --from-beginning"
