# ADR 0001 — Decisiones del commit base

- **Fecha:** 8 oct 2026
- **Estado:** aceptada
- **Fuente:** plan de trabajo del equipo, revisado por un arquitecto de software y un experto en apuestas deportivas

## Arquitectura

| Tema | Decisión |
|---|---|
| Capas | Capas con inversión de dependencias. Las interfaces viven en dominio/aplicación y la infraestructura las implementa |
| Organización | Un directorio por módulo con las cuatro capas dentro. Solo `main.go` importa módulos |
| Router / driver | chi v5 y pgx v5 (pgxpool) |
| Migraciones | goose v3 embebidas, con `WithAllowOutofOrder(true)`, porque cada persona usa su propio prefijo. golang-migrate ignoraría migraciones de número menor |
| Transacción entre módulos | `WalletService` recibe `db.Querier`, así PlaceBet debita e inserta la apuesta en una sola transacción, y el registro crea la billetera en la misma transacción que el usuario |
| Qué selección gana | Lo decide markets (`ResultReader.ResolveSelections`), no settlement |
| Auth | JWT HS256 de 15 min en memoria. Refresh opaco de 32 bytes, guardado como sha256 en `refresh_tokens`, rotado en cada uso, en cookie httpOnly `SameSite=Strict` con `Path=/api/v1/auth` |
| SSE | `GET /events/stream` es público, porque EventSource no envía cabeceras, y solo emite datos de partidos. Sin `WriteTimeout`, `middleware.Timeout` ni `Compress` |
| Proveedores | `FixtureProvider` y `LiveScoreProvider`. El Sync Worker es el único que consulta al proveedor, con un presupuesto diario persistido en `provider_usage` |
| Mocks del frontend | Prism sobre `openapi.yaml` (`npm run mock`). Los tipos TS están escritos a mano en `core/api/models.ts`, porque openapi-typescript aún no soporta TypeScript 6 |

## Mercados (versión mínima)

Se resuelven con el marcador de **tiempo reglamentario** (`score.fulltime`: 90 min + añadido). Nunca se usa `goals`, porque en AET/PEN incluye la prórroga.

| Mercado | Selección | Gana si (H-A) |
|---|---|---|
| `MATCH_RESULT` | `HOME` / `DRAW` / `AWAY` | H > A / H = A / H < A |
| `TOTAL_GOALS_2_5` | `OVER` / `UNDER` | H + A ≥ 3 / H + A ≤ 2 |
| `BOTH_TEAMS_TO_SCORE` | `YES` / `NO` | H ≥ 1 y A ≥ 1 / alguno en 0 |

Las 7 selecciones se crean juntas al importar el evento.

## Modelo de cuotas (0 requests)

- Cada equipo tiene `teams.strength` entre 0.60 y 1.60 (1.00 por defecto).
- λH = 1.45 · sH / sA y λA = 1.15 · sA / sH.
- P(i, j) = Pois(i; λH) · Pois(j; λA) sobre una grilla de 0..10 goles. Se suman las celdas según cada mercado y se normaliza.
- Margen: 6 % en 1X2 y 5 % en los mercados de dos vías.
- `odd_x100 = max(101, min(5000, floor(100 / (p · (1 + m)))))`.
- Se calcula una vez al importar el evento y no se recalcula, porque los mercados cierran al inicio.

Con ambos equipos en s = 1.00:

| Mercado | Cuotas |
|---|---|
| 1X2 | 2.14 / 3.62 / 3.14 |
| Over/Under 2.5 | 1.97 / 1.83 |
| Ambos marcan (Sí/No) | 1.82 / 1.99 |

**Plan B si falta tiempo:** usar estas cuotas fijas para todos los partidos.

## Reglas de apuesta (centavos)

| Regla | Valor |
|---|---|
| Saldo inicial (`DEPOSIT_INITIAL`) | 100 000 (1 000.00), *pendiente de confirmar*, en `WALLET_INITIAL_BALANCE_CENTS` |
| Stake mínimo / máximo | 100 / 50 000 |
| Selecciones | Simple: 1. Combinada: 2 a 5. Nunca dos del mismo partido |
| Cuota total máxima | 500.00, truncando a centésimas en cada multiplicación |
| Pago máximo | 1 000 000. Se rechaza al apostar si se supera |
| Cuota cambiada | 409 `ODDS_CHANGED`, con las cuotas nuevas para que el usuario confirme |

## Liquidación

- **Estado de la apuesta:**
  1. Alguna selección LOST: la apuesta es LOST, aunque otras sigan PENDING.
  2. Si no, alguna PENDING: sigue PENDING.
  3. Si no, todas VOID: es VOID y se reembolsa el stake (`BET_REFUND`).
  4. En cualquier otro caso: WON, con las selecciones VOID a cuota 1.00.
- **Pago:** `payout = floor(stake × Combine(cuotas no VOID) / 100)`, con tope de 1 000 000. Ejemplo: 10.00 × (2.14 × VOID × 1.97) = 10.00 × 4.21 = **42.10**.
- **La selección es VOID cuando el evento termina** en POSTPONED, CANCELLED, ABANDONED o AWARDED, cuando no tiene final 48 h después de `start_at`, o cuando la apuesta se aceptó después del inicio real (`kicked_off_at`).
- **Idempotencia:** `WHERE status='PENDING'` más `UNIQUE(reference, type)`. Las referencias son `bet:<id>:v<settlement_version>`.
- **Corrección de resultado (solo admin):**
  1. Revertir el movimiento anterior con `BET_PAYOUT_REVERSAL` o `BET_REFUND_REVERSAL`.
  2. Incrementar `settlement_version`.
  3. Volver a liquidar. La reversión puede dejar el saldo negativo.
- **Cuándo liquidar con API-Football:** 10 min después de FT, o al confirmar el resultado. Con Replay, de inmediato.

## Estados del evento (API-Football → interno)

| API-Football | Interno | Efecto |
|---|---|---|
| TBD, NS | SCHEDULED | Mercados abiertos |
| 1H, HT, 2H, ET, BT, P, LIVE | LIVE | La primera transición emite `MatchStarted` |
| SUSP, INT | INTERRUPTED | Esperar; a las 48 h sin final, VOID |
| FT, AET, PEN | FINISHED | `MatchFinished`; se liquida con `score.fulltime` |
| PST / CANC / ABD / AWD, WO | POSTPONED / CANCELLED / ABANDONED / AWARDED | `MatchFinished` con ese `Status`; anula las selecciones |

Los estados terminales no retroceden, salvo por acción del admin.

## wallet_transactions

`amount_cents` lleva signo: positivo acredita y negativo debita. Invariante: `wallets.balance_cents = SUM(amount_cents)`.

| type | Signo | reference |
|---|---|---|
| `DEPOSIT_INITIAL` | + | `user:<id>` |
| `BET_STAKE` | − | `bet:<id>` |
| `BET_PAYOUT`, `BET_REFUND` | + | `bet:<id>:v<n>` |
| `BET_PAYOUT_REVERSAL`, `BET_REFUND_REVERSAL` | − | `bet:<id>:v<n>` |
| `ADMIN_ADJUSTMENT` | ± | `admin:<uuid>` |

Una apuesta LOST no genera movimiento, porque el stake ya se debitó.
