# AGENTS.md — reglas para personas y agentes de código

> CONGELADO después del commit base. Aplica a cualquier agente (Claude Code, Copilot, Cursor…) y a las personas del equipo.

SportBet es una casa de apuestas deportivas con dinero virtual: Angular 22, Go 1.26 y PostgreSQL 17. La arquitectura es por capas con inversión de dependencias, un directorio por módulo, Repository y Adapter. La entrega es el 13 oct 2026.

## Reglas obligatorias para agentes

1. Modifica solo archivos dentro de las carpetas de tu módulo (ver "Dueños").
2. No reformatees ni corrijas archivos ajenos, aunque veas errores: repórtalos.
3. No cambies `backend/internal/shared/`, `backend/openapi.yaml`, `backend/cmd/api/main.go`, `frontend/src/app/app.routes.ts`, `frontend/src/app/core/` ni las dependencias (`go.mod`, `package.json`).
4. Si necesitas algo de otro módulo, usa su interfaz de `shared/contracts` y su fake (`shared/contracts/fakes`).
5. Ejecuta `gofmt`, `scripts/check-imports.sh` y las pruebas antes de cada commit.

## Dueños

| Persona | Backend (`backend/internal/`) | Frontend (`frontend/src/app/features/`) | Documento |
|---|---|---|---|
| 1. Plataforma y Auth | `auth/`, `users/`, `admin/` | `auth/`, `admin/` | `docs/persona1-plataforma.md` |
| 2. Datos deportivos | `events/`, `markets/`, `providers/` (+ `testdata/replay/`) | — | `docs/persona2-datos.md` |
| 3. Núcleo transaccional | `betting/`, `wallet/`, `settlement/` | — | `docs/persona3-transaccional.md` |
| 4. Frontend | — | `events/`, `betting/`, `wallet/` | `docs/persona4-frontend.md` |

`auth` y `users` comparten la tabla `users`; las dos son de la Persona 1.

## Archivos compartidos

| Archivo | Regla |
|---|---|
| `main.go`, `app.routes.ts`, `docker-compose.yml`, `AGENTS.md` | Congelados |
| `shared/`, `openapi.yaml`, `core/` del frontend | Solo cambian en un PR pequeño y aparte, avisando al grupo. Se fusiona de inmediato y todos hacen rebase |
| Migraciones nuevas | Prefijo por persona: `1xxx_`, `2xxx_`, `3xxx_`. Nadie edita `0001_init.sql`. goose acepta migraciones fuera de orden |
| `go.mod`, `package.json` | Una dependencia nueva va en un PR aparte de una línea |
| Documentación | Un archivo por persona en `docs/`; el README se arma el lunes |
| Código generado (Swagger, clientes) | No se sube al repositorio |

## Estructura de un módulo del backend

```
betting/
├── domain/          entidades y reglas; sin pgx, chi ni net/http
├── application/     casos de uso; define los puertos (interfaces de repositorio)
├── infrastructure/  repositorios concretos (pgx) y adapters
├── presentation/    handlers HTTP y DTOs
└── module.go        func Register(r chi.Router, d app.Deps)
```

- `internal/<X>` solo importa `internal/<X>/...`, `internal/shared/...`, la librería estándar y lo que está en `go.mod`. Nunca otro módulo. CI lo verifica con `scripts/check-imports.sh`.
- Solo `cmd/api/main.go` importa módulos.
- `Register` usa rutas completas (`r.Get("/events/{id}", h)`, `r.With(d.Auth.RequireAuth).Post("/bets", h)`). Nunca `r.Route` ni `r.Mount` sobre un prefijo que otro módulo también use, porque chi entra en pánico.
- Los constructores de contratos que llama `main.go` (`auth.NewAuthenticator`, `wallet.NewService`, `markets.NewMarketReader`, `markets.NewResultReader`, `providers.New`) tienen la firma congelada. Hoy devuelven el fake; el dueño solo cambia el cuerpo.
- Los workers de fondo (Sync Worker, barrido de settlement) se registran con `d.Lifecycle.Go(nombre, fn)`.
- La hora se lee de `d.Clock()`, no de `time.Now()`, para que el ReplayProvider acelerado y las pruebas funcionen.

## Contratos entre módulos (`shared/contracts`)

| Contrato | Lo implementa | Lo usa | Qué ofrece |
|---|---|---|---|
| `Authenticator` / `CurrentUser` | Persona 1 | Todos | Middleware `RequireAuth`/`RequireRole`; `contracts.MustCurrentUser(ctx)` |
| `MarketReader` | Persona 2 | Persona 3 | Selección, cuota actual, `IsOpen(now)` |
| `ResultReader` | Persona 2 | Persona 3 | Resultado final y `ResolveSelections` (WON/LOST/VOID por selección) |
| `WalletService` | Persona 3 | Personas 1 y 3 | `CreateWallet`, `Debit`, `Credit`, `Balance`; reciben `db.Querier` para participar en la transacción del llamador |
| `FixtureProvider`, `LiveScoreProvider` | Persona 2 | Persona 2 | Calendario, resultados y marcador en vivo |
| API REST (`openapi.yaml`) | Personas 1, 2 y 3 | Persona 4 | El frontend trabaja contra Prism (`npm run mock` + `npm run start:mock`) |

Cada contrato tiene un fake en `contracts/fakes` y debe tener una suite en `contracts/contracttest` que pase contra el fake y contra la implementación real. Hoy existe la de `WalletService`; las demás las agrega el dueño en un PR aparte de `shared/`.

## Bus de eventos (`shared/eventbus`)

| Evento | Lo publica | Lo consume |
|---|---|---|
| `MatchUpdated` | Sync Worker (P2) | SSE hacia el frontend |
| `MatchStarted` | Sync Worker (P2) | markets: cierra los mercados del evento |
| `MatchFinished` | Sync Worker (P2) | settlement (P3): liquida; el `Status` decide si se anula |
| `BetPlaced`, `BetSettled` | P3 | admin (P1): auditoría y estadísticas |

- Se publica solo después del COMMIT.
- El bus puede perder eventos: los consumidores son idempotentes y settlement tiene un barrido de respaldo al arrancar y cada `SETTLEMENT_SWEEP_INTERVAL`.

## Reglas de dominio (no negociables)

- **Dinero:** `money.Money` en centavos (`int64`). **Cuotas:** `money.Odds` en centésimas (`214` = 2.14). Nunca `float64` para dinero ni cuotas, ni en Go ni en JSON.
- **Débito atómico:** `UPDATE wallets SET balance_cents = balance_cents - $1 WHERE user_id = $2 AND balance_cents >= $1`. Con 0 filas afectadas se devuelve `ErrInsufficientFunds`. Nunca "leer saldo y luego debitar".
- **Idempotencia:** `UNIQUE(user_id, idempotency_key)` en `bets`, que llega en el header `Idempotency-Key`. `UNIQUE(reference, type)` en `wallet_transactions`, para que liquidar dos veces no pague dos veces.
- **Liquidar:** `UPDATE bets SET status=$1 ... WHERE id=$2 AND status='PENDING'` en la misma transacción que el crédito. Con 0 filas, no se hace nada.
- **Mercados:** se cierran al inicio del partido, por `MatchStarted` y también por reloj (`SelectionInfo.IsOpen` exige `now < start_at − 60 s`). No hay apuestas en vivo.
- **Cuota cambiada:** el cliente envía la `oddX100` que vio. Si no coincide, se responde 409 `ODDS_CHANGED`. La apuesta se liquida con la cuota congelada en `bet_selections.odd_x100`.
- **Combinadas:** de 2 a 5 selecciones y nunca dos del mismo partido (422 `CORRELATED_SELECTIONS`). Una selección VOID cuenta como cuota 1.00. Si alguna es LOST, la apuesta es LOST. Si todas son VOID, se reembolsa.
- **Resultado:** se usa el marcador de tiempo reglamentario (`score.fulltime`), nunca `goals` de API-Football, porque incluye la prórroga.

Los detalles y valores numéricos están en [docs/adr/0001-decisiones-base.md](docs/adr/0001-decisiones-base.md).

## Flujo de Git

- Un branch por persona. `main` solo recibe PRs con CI en verde.
- PRs pequeños, al menos dos fusiones por persona al día.
- `git rebase origin/main` cada mañana y antes de abrir cada PR.
- Antes de abrir un PR, revisar con `git diff --stat origin/main` que solo aparezcan carpetas propias.

## Comandos

```bash
cp .env.example .env
docker compose up -d postgres
cd backend && go run ./cmd/api                 # API en :8080, aplica migraciones al arrancar
cd backend && go test ./... && ../scripts/check-imports.sh
cd frontend && npm start                       # Angular en :4200 con proxy a :8080
cd frontend && npm run mock & npm run start:mock   # frontend contra mocks del contrato
```
