FROM golang:alpine AS builder
WORKDIR /app

# Download dependencies first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest
WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/main .

CMD ["./main"]
