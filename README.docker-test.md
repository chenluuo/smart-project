# Docker Test Environment

`compose.test.yaml` starts an isolated frontend and Go API environment for local testing. It has its own Docker volumes, so it does not reuse or change data from `compose.yaml`.

## Start

```powershell
docker compose -f compose.test.yaml up -d --build
```

Open these addresses after the containers are healthy:

- Frontend: `http://127.0.0.1:5174`
- API health check: `http://127.0.0.1:8081/actuator/health`
- MinIO console: `http://127.0.0.1:9003`

The lightweight test environment includes MySQL, Redis, MinIO, MQTT, the Go API, the frontend, and `agent-service`. It supports manual offline-backlog delivery in the Q&A page. Regular AI chat still requires the context, tool, ingest, and vector-service stack from the root `compose.yaml`.

## Verify And Stop

```powershell
docker compose -f compose.test.yaml ps
docker compose -f compose.test.yaml logs -f go-api frontend
docker compose -f compose.test.yaml down
```

Use `docker compose -f compose.test.yaml down -v` only when a completely empty test database and object store are wanted.
