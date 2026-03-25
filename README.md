# api-health-monitor (Go + k3s)

A lightweight API health monitor written in Go, with SQLite storage and Prometheus metrics.

## Run locally

### Prerequisites
- Go (matching `go.mod`)
- Make

### Start
```bash
make run
```

By default the app listens on `:8080` and uses `monitor.db` in the current working directory.

## Run with Docker

### Build image
```bash
make docker-build
```

### Run (foreground)
```bash
make docker-run
```

### Run (detached)
```bash
make docker-run-detached
```

### Stop
```bash
make docker-stop
```

### Run with Docker Compose
```bash
docker compose up --build
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `ADDR` | `:8080` | HTTP server bind address |
| `DB_PATH` | `./data/monitor.db` | SQLite database file path |

Schema migrations are applied automatically on startup.

## Endpoints

### Operational
- `GET /` - basic OK response
- `GET /healthz` - liveness probe
- `GET /readyz` - readiness probe
- `GET /metrics` - Prometheus metrics

### API (`/v1`)
- `GET /v1/status`
- `GET /v1/targets`
- `GET /v1/targets/{id}`
- `GET /v1/targets/{id}/checks`
- `POST /v1/targets`
- `PATCH /v1/targets`
- `DELETE /v1/targets/{id}`

## Examples

### Health / readiness / metrics
```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -s http://localhost:8080/metrics | head
```

### Create a target
```bash
curl -i -X POST http://localhost:8080/v1/targets \
	-H "Content-Type: application/json" \
	-d '{
		"name": "Example",
		"url": "https://example.com",
		"method": "GET",
		"interval": 30
	}'
```

### List targets
```bash
curl -s http://localhost:8080/v1/targets
```

### Get one target
```bash
curl -s http://localhost:8080/v1/targets/<target-id>
```

### Update a target
```bash
curl -i -X PATCH http://localhost:8080/v1/targets \
	-H "Content-Type: application/json" \
	-d '{
		"id": "<target-id>",
		"name": "Example Updated",
		"url": "https://example.com",
		"method": "GET",
		"interval": 60,
		"enabled": true
	}'
```

### Get checks for a target
```bash
curl -s http://localhost:8080/v1/targets/<target-id>/checks
```

### Delete a target
```bash
curl -i -X DELETE http://localhost:8080/v1/targets/<target-id>
```
