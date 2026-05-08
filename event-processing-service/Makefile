COMPOSE := docker compose -f ../docker-compose.test.yml
TEST_DATABASE_URL := postgres://postgres:postgres@localhost:5434/events_test?sslmode=disable
TEST_KAFKA_BROKERS := localhost:19092

.PHONY: test test-only test-down show-coverage

test:
	$(COMPOSE) up -d --wait
	set +e; \
	  TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_KAFKA_BROKERS="$(TEST_KAFKA_BROKERS)" \
	    go test -coverprofile=coverage.out -covermode=atomic ./...; STATUS=$$?; \
	  $(MAKE) --no-print-directory show-coverage; \
	  exit $$STATUS

test-only:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_KAFKA_BROKERS="$(TEST_KAFKA_BROKERS)" \
	  go test -coverprofile=coverage.out -covermode=atomic ./...

show-coverage:
	@if [ -f coverage.out ]; then \
	  go tool cover -func=coverage.out | awk \
	    '/^total:/{pct=$$NF; gsub(/%/,"",pct); if(pct+0>=80)c="\033[32m"; else if(pct+0>=50)c="\033[33m"; else c="\033[31m"; printf "\n" c "  Total coverage: %s\033[0m\n\n", $$NF; next} {printf "  %-65s %s\n", $$1, $$NF}'; \
	fi

test-down:
	$(COMPOSE) down -v
