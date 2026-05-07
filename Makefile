SHELL := /bin/bash
COMPOSE := docker compose -f docker-compose.test.yml

.PHONY: test-all test-down-all coverage-summary clean-test

test-all:
	$(COMPOSE) up -d --wait
	set +e; \
	  $(MAKE) -C notifications-service test-only & \
	  $(MAKE) -C event-processing-service test-only & \
	  $(MAKE) -C ingestion-service test-only & \
	  wait; STATUS=$$?; \
	  if [ -d e2e ] && [ $$STATUS -eq 0 ]; then go test ./e2e/...; STATUS=$$?; fi; \
	  $(MAKE) --no-print-directory coverage-summary; \
	  $(COMPOSE) down -v; \
	  exit $$STATUS

coverage-summary:
	@echo ""
	@echo "╔══════════════════════════════════════════╗"
	@echo "║         Test Coverage Summary            ║"
	@echo "╠══════════════════════════════════════════╣"
	@for svc in notifications-service event-processing-service ingestion-service; do \
	  if [ -f $$svc/coverage.out ]; then \
	    (cd $$svc && go tool cover -func=coverage.out) 2>/dev/null | awk -v svc="$$svc" \
	      '/^total:/{pct=$$NF; gsub(/%/,"",pct); if(pct+0>=80)c="\033[32m"; else if(pct+0>=50)c="\033[33m"; else c="\033[31m"; printf "║  %-30s %s%7s\033[0m  ║\n", svc, c, $$NF}'; \
	  else \
	    printf "║  %-30s \033[90m%7s\033[0m  ║\n" "$$svc" "n/a"; \
	  fi; \
	done
	@echo "╚══════════════════════════════════════════╝"
	@echo ""

test-down-all:
	$(COMPOSE) down -v

clean-test:
	go clean -testcache
	@for svc in notifications-service event-processing-service ingestion-service; do \
	  rm -f $$svc/coverage.out; \
	done

simulate-cleanup:
	docker compose -f docker-compose.simulation.yml -p pred-simulation down -v

simulate:
	$(MAKE) simulate-cleanup
	docker compose -f docker-compose.simulation.yml -p pred-simulation up --build