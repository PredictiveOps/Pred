# Pred

This is the Predictive Maintenance Project of group H for the SE module (CS3023) for CSE batch 23.

## Features

It's a multi-tenant system that allows users to manage their equipment and receive notifications when maintenance is required.

## Project setup

This repository contains all the code to all the services.

### Prerequisites

| Tool           | Version | Install                                                                                                                                        |
| -------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| git            | >=2.0   | [git-scm.com/downloads](https://git-scm.com/downloads)                                                                                         |
| golang         | >=1.23  | [go.dev/doc/install](https://go.dev/doc/install)                                                                                               |
| node           | >=20    | [nodejs.org/en/download](https://nodejs.org/en/download)                                                                                       |
| Docker         | >=20.10 | [docs.docker.com/get-docker](https://docs.docker.com/get-docker/)                                                                              |
| Docker Compose | >=2.0   | [docs.docker.com/compose/install](https://docs.docker.com/compose/install/). Or Docker Desktop. Or [OrbStack](https://orbstack.dev/) on macOS. |

### Installation

Clone the repository:

```sh
git clone https://github.com/PredictiveOps/Pred.git
cd Pred
```

Start the shared infrastructure (Postgres on host port `5433`, Kafka on `9092`, Mosquitto MQTTS on `8883`). The Postgres container creates the databases listed in `POSTGRES_MULTIPLE_DATABASES` on first boot:

```sh
docker compose up -d
```

Set up the notifications service:

```sh
cd notifications-service
cp .env.example .env       # edit if defaults don't match
go mod download
go run .
```

Set up the event processing service:

```sh
cd event-processing-service
cp .env.example .env       # edit if defaults don't match
go mod download
go run .
```

Set up the web frontend:

```sh
cd web-frontend
cp .env.example .env
npm install
npm run dev
```

### Keycloak Setup

For authentication setup, see [keycloak/README.md](./keycloak/README.md).

### Mosquitto Setup

For MQTT broker setup and credentials, see [mosquitto/README.md](./mosquitto/README.md).

### Running tests

Go services share a test Postgres instance (host port `5434`) and a test Kafka broker (host port `19092`) defined in `docker-compose.test.yml`. Each service gets its own database in the test Postgres. Both run alongside the dev compose without port conflicts.

Run all service tests and end-to-end tests in one command from the repo root:

```sh
make test-all      # brings up test infra, runs all service tests in parallel + e2e, tears down on exit
make test-down-all # tear down test infra manually if needed
```

To test a single service:

```sh
cd notifications-service
make test         # brings up test infra and runs `go test ./...`
make test-down    # tear down when finished

cd ../event-processing-service
make test
make test-down
```

### System Simulation

To simulate the whole system (for testing or demoing purposes), run:

```sh
make simulate
```

## Services

All services must have a `.env.example` file with the required and optional environment variables with their default values.

All services must have a `README.md` file with the following sections:

- What it is supposed to do
- How to run it

Service READMEs must not document database internals (tables, columns, indexes). That level of detail belongs in the code (models, migrations).
