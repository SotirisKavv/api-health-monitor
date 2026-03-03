# api-health-monitor (Go + k3s)

## Run locally
make run

## Database
SQLite database file: internal/store/targets.db
Schema is applied at startup via embedded SQL.

## Endpoints
- GET /healthz
- GET /readyz
- GET /metrics

### Targets API (v1)
- GET /v1/targets
- GET /v1/targets/{id}
- POST /v1/targets
- PATCH /v1/targets
- DELETE /v1/targets/{id}
