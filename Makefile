SHELL := /bin/bash
COMPOSE := docker compose -f docker-compose.test.yml

.PHONY: test-all test-down-all

test-all:
	$(COMPOSE) up -d --wait
	trap '$(COMPOSE) down -v' EXIT; \
	  $(MAKE) -C notifications-service test-only & \
	  $(MAKE) -C event-processing-service test-only & \
	  $(MAKE) -C ingestion-service test-only & \
	  wait && \
	  if [ -d e2e ]; then go test ./e2e/...; fi

test-down-all:
	$(COMPOSE) down -v
