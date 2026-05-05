COMPOSE := docker compose -f ../docker-compose.test.yml
TEST_DATABASE_URL := postgres://postgres:postgres@localhost:5434/events_test?sslmode=disable
TEST_KAFKA_BROKERS := localhost:19092

.PHONY: test test-only test-down

test:
	$(COMPOSE) up -d --wait
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_KAFKA_BROKERS="$(TEST_KAFKA_BROKERS)" go test ./...

test-only:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_KAFKA_BROKERS="$(TEST_KAFKA_BROKERS)" go test ./...

test-down:
	$(COMPOSE) down -v
