# SportBet

Casa de apuestas deportivas con **dinero virtual**. Proyecto del curso de Arquitectura de Software; entrega el 13 de octubre de 2026.

> **Borrador.** Lo armó la Persona 1 con lo que hay en `main` el lunes 12; revísenlo y completen lo que falte antes de la entrega.

Partidos reales de Premier League, LaLiga y Liga 1 de Perú (vía API-Football) o un partido grabado (replay) para la demo sin conexión. Cada usuario empieza con 1 000.00 de saldo virtual, apuesta con cuotas que calcula el propio sistema y ve el marcador y la línea de tiempo del partido en vivo.

## Qué hace

- **Cuentas:** registro, inicio de sesión, sesión que se renueva sola y cierre de sesión.
- **Partidos:** lista por liga con escudos, filtros por fecha, liga y estado; detalle con jornada, estadio y **línea de tiempo** (goles, tarjetas, cambios, VAR).
- **En vivo:** marcador y sucesos se actualizan solos por SSE, sin recargar la página.
- **Apuestas:** simples y combinadas en 3 mercados (1X2, más/menos de 2.5 goles, ambos marcan). Las cuotas salen de un modelo de Poisson con la fuerza de cada equipo, calculada a partir de la tabla de posiciones.
- **Billetera:** saldo, movimientos e historial de apuestas. Las apuestas se liquidan solas al terminar el partido (ganada, perdida o anulada).
- **Administración:** estadísticas (apuestas, total apostado y pagado, resultado de la casa), actividad reciente y gestión de usuarios (bloquear o desbloquear, dar o quitar admin).

## Arquitectura

**Capas con inversión de dependencias**, un directorio por módulo y cada módulo con sus cuatro capas (`domain`, `application`, `infrastructure`, `presentation`). Los módulos no se importan entre sí: se comunican por las interfaces de `shared/contracts` y por un bus de eventos en memoria. CI verifica estas reglas.

```mermaid
flowchart LR
    subgraph Navegador
        NG[Angular 22]
    end
    subgraph API Go
        direction TB
        REST[REST /api/v1]
        SSE[SSE /events/stream]
        MOD[auth · users · events · markets<br/>betting · wallet · settlement]
        SW[Sync Worker]
        BUS([bus de eventos])
    end
    PG[(PostgreSQL 17)]
    AF[API-Football<br/>o replay]

    NG -- HTTP + JWT --> REST
    SSE -- match.updated --> NG
    REST --> MOD
    MOD --> PG
    SW -- única que consulta --> AF
    SW --> PG
    SW --> BUS
    BUS --> SSE
    BUS --> MOD
```

Decisiones clave (la justificación completa de la arquitectura y de cada tecnología está en [`docs/arquitectura/justificacion.md`](docs/arquitectura/justificacion.md)):

| Decisión | Por qué |
|---|---|
| Solo el Sync Worker consulta al proveedor; los usuarios leen de la base | El plan gratuito da 100 llamadas al día: la carga de usuarios no gasta llamadas |
| Débito atómico (`UPDATE … WHERE balance_cents >= monto`) en la misma transacción que la apuesta | Evita el doble gasto; verificado con 20 apuestas simultáneas |
| Dinero en centavos y cuotas en centésimas, siempre enteros | Sin errores de redondeo de `float` |
| Cuotas propias y congeladas al apostar | No dependen del proveedor; si cambian antes de confirmar, la apuesta se rechaza con `ODDS_CHANGED` |
| Mercados cerrados al empezar el partido | El dato en vivo llega con hasta 3 minutos de retraso |
| Proveedor detrás de interfaces (Adapter) | API-Football y el replay son intercambiables con `PROVIDER` |

## Estructura y dueños

```
backend/
  cmd/api/main.go           punto de entrada (congelado)
  internal/shared/          contratos, bus, dinero, errores, base de datos (compartido)
  internal/auth, users, admin             Persona 1
  internal/events, markets, providers     Persona 2
  internal/betting, wallet, settlement    Persona 3
  migrations/               0001 base; 1xxx/2xxx/3xxx por persona
  openapi.yaml              contrato REST (compartido)
frontend/src/app/
  core/                     cliente API, sesión, interceptores, SSE (compartido)
  features/auth, admin                    Persona 1
  features/events, betting, wallet        Persona 4
loadtest/                   pruebas de carga k6
docs/                       un documento por persona + ADR
```

Las reglas para trabajar en el repo (también para agentes de IA) están en [`AGENTS.md`](AGENTS.md).

## Cómo correrlo

**Requisitos:** Go 1.26, Node 24, Docker.

### 1. Base de datos y configuración

```bash
cp .env.example .env
docker compose up -d postgres
```

En `.env`, define `ADMIN_SEED_PASSWORD` (mínimo 8 caracteres) para que se cree el usuario admin `admin@sportbet.local` al arrancar.

### 2. API

```bash
cd backend
set -a && . ../.env && set +a   # go run no lee .env por sí solo
go run ./cmd/api                # http://localhost:8080, aplica las migraciones al arrancar
```

O con Docker, que sí lee `.env`: `docker compose up -d api`.

### 3. Frontend

```bash
cd frontend
npm ci
npm start                       # http://localhost:4200, con proxy a la API en :8080
```

Para trabajar solo con el frontend contra respuestas simuladas del contrato: `npm run mock` en una terminal y `npm run start:mock` en otra.

### Datos: replay o API-Football

| | Replay (por defecto) | API-Football |
|---|---|---|
| Configuración | `PROVIDER=replay` | `PROVIDER=apifootball`, `APIFOOTBALL_KEY=…`, `FEATURED_LEAGUE_IDS=39,140,281` |
| Partidos | Uno grabado, que empieza al arrancar la API | Los de hoy y mañana de las ligas elegidas |
| Llamadas | Ninguna | Hasta 100 al día (plan gratuito); el contador está en `provider_usage` |
| Uso | Desarrollo y demo sin conexión | Demo con datos reales |

Con API-Football, **el marcador en vivo solo se consulta si hay un partido destacado en juego**: márcalo como destacado desde el panel de admin antes de que empiece. Los demás partidos se actualizan cada 10 minutos al confirmar resultados.

### Variables principales

| Variable | Valor por defecto | Para qué |
|---|---|---|
| `DATABASE_URL` | `postgres://sportbet:sportbet@localhost:5432/sportbet` | Conexión a PostgreSQL |
| `JWT_SECRET` | — | Firma de los tokens; mínimo 32 caracteres fuera de desarrollo |
| `ADMIN_SEED_PASSWORD` | vacío | Si tiene valor, crea el admin inicial |
| `WALLET_INITIAL_BALANCE_CENTS` | `100000` | Saldo inicial (1 000.00) |
| `PROVIDER` | `replay` | `replay` o `apifootball` |
| `FEATURED_LEAGUE_IDS` | vacío | IDs de API-Football; vacío = todas las ligas del día |
| `REPLAY_SPEED` | `60` | Aceleración del replay (60 = un partido en 1 min 45 s) |
| `SYNC_LIVE_INTERVAL` | `3m` | Cada cuánto se consulta el marcador en vivo |

La lista completa está en [`.env.example`](.env.example).

## Pruebas

```bash
cd backend && go test ./...          # las de integración necesitan TEST_DATABASE_URL
../scripts/check-imports.sh          # reglas de dependencias entre módulos
cd frontend && npm test
```

- **Integración:** con `TEST_DATABASE_URL=postgres://…/sportbet_test` las pruebas de repositorios corren contra PostgreSQL, cada una en su propio schema; sin la variable se saltan.
- **CI (GitHub Actions):** en cada PR corre `gofmt`, `go mod tidy`, `go vet`, las reglas de importación, las pruebas del backend con PostgreSQL y el build y las pruebas del frontend. `main` solo acepta PRs con CI en verde.
- **Carga:** [`loadtest/`](loadtest/README.md). Con 50 usuarios simultáneos, p95 de 2.6 ms al leer y 8.0 ms al apostar, sin errores; de 20 apuestas simultáneas con saldo para una, se acepta exactamente una.

## Documentación

| Documento | Contenido |
|---|---|
| [`docs/arquitectura/justificacion.md`](docs/arquitectura/justificacion.md) | **Por qué** este estilo arquitectónico, estos patrones y cada tecnología, con las alternativas descartadas |
| [`docs/arquitectura/evaluacion-atam.md`](docs/arquitectura/evaluacion-atam.md) | Evaluación ATAM: árbol de utilidad, 8 escenarios, puntos de sensibilidad, compromisos y riesgos |
| [`docs/adr/0001-decisiones-base.md`](docs/adr/0001-decisiones-base.md) | Decisiones de arquitectura, mercados, modelo de cuotas, liquidación |
| [`docs/adr/0002-decisiones-integracion.md`](docs/adr/0002-decisiones-integracion.md) | Decisiones al integrar datos reales: plan gratuito de API-Football, vivo con destacados, fuerza de equipos, línea de tiempo, auditoría |
| [`docs/persona1-plataforma.md`](docs/persona1-plataforma.md) | Diagrama de capas, seguridad, descripción en Acme, análisis PASA |
| [`docs/persona2-datos.md`](docs/persona2-datos.md) | Adapter de proveedores, fallos externos y presupuesto, escenarios ATAM 1, 2 y 4 |
| [`docs/persona3-transaccional.md`](docs/persona3-transaccional.md) | Modelo de dominio, flujos de apuesta y liquidación, escenarios ATAM 3 y 5 |
| [`docs/persona4-frontend.md`](docs/persona4-frontend.md) | Usabilidad, casos de uso, manual de usuario |
| [`backend/openapi.yaml`](backend/openapi.yaml) | Contrato de la API REST |

## Limitaciones conocidas

- **Plan gratuito de API-Football:** solo da fechas de ayer a mañana y tablas de posiciones hasta la temporada 2024. Por eso la fuerza de los equipos es histórica, y los recién ascendidos quedan en 1.00.
- **Una sola instancia de la API:** el bus de eventos es en memoria. Para escalar a varias instancias habría que pasarlo a Postgres `LISTEN/NOTIFY` o Redis sin cambiar su interfaz.
- **Estadísticas del panel de admin:** cuentan las apuestas desde que el módulo admin escucha el bus. El bus es en memoria, así que un anuncio perdido (por ejemplo, si la API se reinicia en ese instante) no se recupera en la auditoría, aunque la apuesta y su liquidación sí quedan bien.

## Equipo

| Persona | GitHub | Responsabilidad |
|---|---|---|
| 1 · Plataforma y Auth | [@Kevrc](https://github.com/Kevrc) | Autenticación, usuarios, administración, CI, pruebas de carga |
| 2 · Datos deportivos | [@Carlos2190](https://github.com/Carlos2190) | Eventos, mercados y cuotas, proveedores, Sync Worker, SSE |
| 3 · Núcleo transaccional | [@By-Alvarez06](https://github.com/By-Alvarez06) | Apuestas, billetera, liquidación |
| 4 · Frontend | [@jeimytorresg](https://github.com/jeimytorresg) | Eventos, boleto de apuestas, billetera, historial |
