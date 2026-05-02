# --------- Stage 1: Build ---------
FROM golang:1.26.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ingestion-service .

# --------- Stage 2: Runtime ---------
FROM alpine:3.23
WORKDIR /app
COPY --from=builder /app/ingestion-service .
EXPOSE 8081

# Default environment variables
ENV PORT=8081
ENV DATABASE_URL="postgres://postgres:postgres@host.docker.internal:5433/ingestion?sslmode=disable"
ENV KAFKA_BROKERS="host.docker.internal:9092"
ENV KAFKA_TOPIC="ingestion_events"
ENV MQTT_BROKER="tcp://host.docker.internal:1883"
ENV MQTT_CLIENT_ID="ingestion-service-dev"
ENV MQTT_TOPIC="devices/+/data"
ENV MQTT_USERNAME=""
ENV MQTT_PASSWORD=""


# Run the service
CMD ["./ingestion-service"]
