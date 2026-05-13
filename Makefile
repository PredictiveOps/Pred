SHELL := /bin/bash
COMPOSE := docker compose -f docker-compose.test.yml

.PHONY: test-all test-down-all coverage-summary clean-test create-mosquitto-certificates create-device-keys format

format:
	gofmt -w notifications-service event-processing-service ingestion-service
	npm --prefix web-frontend run format

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

DEVICE_ID ?= 1
create-device-keys:
	mkdir -p ./device-simulation/keys/device-$(DEVICE_ID)
	openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 \
	  -out ./device-simulation/keys/device-$(DEVICE_ID)/private.pem
	openssl pkey -in ./device-simulation/keys/device-$(DEVICE_ID)/private.pem \
	  -pubout -out ./device-simulation/keys/device-$(DEVICE_ID)/public.pem
	@echo "Keys written to device-simulation/keys/device-$(DEVICE_ID)/"

NGROK_DOMAIN ?=
create-mosquitto-certificates:
	mkdir -p ./mosquitto/certs
	openssl genpkey -algorithm RSA -out ./mosquitto/certs/ca.key -pkeyopt rsa_keygen_bits:4096
	openssl req -x509 -new -nodes -key ./mosquitto/certs/ca.key -sha256 -days 3650 -out ./mosquitto/certs/ca.crt -subj "/CN=Pred Local CA"
	openssl genpkey -algorithm RSA -out ./mosquitto/certs/server.key -pkeyopt rsa_keygen_bits:2048
	openssl req -new -key ./mosquitto/certs/server.key -out ./mosquitto/certs/server.csr -subj "/CN=localhost"
	if [ -z "$(NGROK_DOMAIN)" ]; then \
	  printf '[req]\ndistinguished_name = req_distinguished_name\nreq_extensions = req_ext\nprompt = no\n\n[req_distinguished_name]\nCN = localhost\n\n[req_ext]\nsubjectAltName = @alt_names\n\n[alt_names]\nDNS.1 = localhost\nIP.1 = 127.0.0.1\n' > ./mosquitto/certs/server.ext; \
	else \
	  printf '[req]\ndistinguished_name = req_distinguished_name\nreq_extensions = req_ext\nprompt = no\n\n[req_distinguished_name]\nCN = localhost\n\n[req_ext]\nsubjectAltName = @alt_names\n\n[alt_names]\nDNS.1 = localhost\nDNS.2 = $(NGROK_DOMAIN)\nIP.1 = 127.0.0.1\n' > ./mosquitto/certs/server.ext; \
	fi
	openssl x509 -req -in ./mosquitto/certs/server.csr -CA ./mosquitto/certs/ca.crt -CAkey ./mosquitto/certs/ca.key -CAcreateserial \
	  -out ./mosquitto/certs/server.crt -days 825 -sha256 -extfile ./mosquitto/certs/server.ext
	chmod 600 ./mosquitto/certs/server.key
	chmod 644 ./mosquitto/certs/server.crt ./mosquitto/certs/ca.crt
	@if [ -n "$(NGROK_DOMAIN)" ]; then echo "Certificates created with SANs: localhost, $(NGROK_DOMAIN)"; else echo "Certificates created with SANs: localhost, 127.0.0.1"; fi