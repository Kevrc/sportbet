# SportBet

Casa de apuestas deportivas con dinero virtual. Proyecto de Arquitectura de Software.

- **Backend:** Go 1.26, chi, pgx y goose, en `backend/`
- **Frontend:** Angular 22, en `frontend/`
- **Base de datos:** PostgreSQL 17, con Docker Compose

> El README completo se arma el lunes. Antes de escribir código, lee [AGENTS.md](AGENTS.md); las decisiones de diseño están en [docs/adr/0001-decisiones-base.md](docs/adr/0001-decisiones-base.md).

## Arranque rápido

```bash
cp .env.example .env
docker compose up -d postgres
cd backend && go run ./cmd/api      # :8080, aplica migraciones al arrancar
cd frontend && npm ci && npm start  # :4200
```
