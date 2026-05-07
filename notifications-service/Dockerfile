# --------- Stage 1: Build ---------
FROM golang:1.26.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o notifications-service .

# --------- Stage 2: Runtime ---------
FROM alpine:3.23
WORKDIR /app
COPY --from=builder /app/notifications-service .
EXPOSE 8080

# Run the service
CMD ["./notifications-service"]
