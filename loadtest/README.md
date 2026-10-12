# Pruebas de carga (Persona 1)

Escenarios de [k6](https://k6.io) para el análisis PASA de `docs/persona1-plataforma.md`. Se corren con Docker, sin instalar k6.

| Script | Qué mide | Criterio de aceptación |
|---|---|---|
| `apuestas.js` | 50 usuarios simultáneos durante 1 min: 70 % navega, 20 % consulta billetera e historial, 10 % apuesta | p95 de lecturas < 300 ms, p95 de apostar < 500 ms, errores < 1 % |
| `doble-gasto.js` | 20 apuestas simultáneas de un usuario con saldo para una sola | Exactamente 1 aceptada, 19 `INSUFFICIENT_FUNDS` y saldo final no negativo |

## Cómo correrlas

Usa una base **aparte**: la prueba crea usuarios y apuestas.

1. Levanta la API apuntando a una base vacía, con `PROVIDER=replay` para no gastar llamadas de API-Football. Por ejemplo, con la base de `docker compose`:
   ```bash
   cd backend && DATABASE_URL=postgres://sportbet:sportbet@localhost:5432/sportbet_carga?sslmode=disable PROVIDER=replay HTTP_ADDR=:8082 go run ./cmd/api
   ```
2. Carga los partidos de prueba. El worker de mercados les crea las cuotas en unos 15 s:
   ```bash
   psql postgres://sportbet:sportbet@localhost:5432/sportbet_carga -f loadtest/seed.sql
   ```
3. Corre los escenarios. Dentro del contenedor, la API del host es `host.docker.internal`:
   ```bash
   docker run --rm -v "$PWD/loadtest":/scripts -e BASE_URL=http://host.docker.internal:8082/api/v1 grafana/k6:0.54.0 run /scripts/doble-gasto.js
   docker run --rm -v "$PWD/loadtest":/scripts -e BASE_URL=http://host.docker.internal:8082/api/v1 grafana/k6:0.54.0 run /scripts/apuestas.js
   ```
   `apuestas.js` acepta `-e USERS=200 -e DURATION=30s` para medir con más carga.

k6 termina con código distinto de 0 si no se cumple algún criterio.
