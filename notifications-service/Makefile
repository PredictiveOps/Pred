COMPOSE := docker compose -f ../docker-compose.test.yml
TEST_DATABASE_URL := postgres://postgres:postgres@localhost:5434/notifications_test?sslmode=disable

.PHONY: test test-down

test:
	$(COMPOSE) up -d --wait
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

test-down:
	$(COMPOSE) down -v
