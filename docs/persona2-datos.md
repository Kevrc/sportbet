# Persona 2 — Datos deportivos

**GitHub:** @Carlos2190 · **Módulos:** events, markets, providers

## Adapter: `FixtureProvider` y `LiveScoreProvider`

`providers.New` elige API-Football o ReplayProvider y entrega ambos contratos a
la aplicación. `events` es el único módulo que consulta esos contratos:

- `APIFootball` usa `GET /fixtures?date=...`, `GET /fixtures?id=...` y
  `GET /fixtures?live=all`. Envía la clave en `x-apisports-key`, aplica timeout
  y convierte los estados del proveedor a `contracts.MatchStatus`.
- `ReplayProvider` lee fixtures y frames JSON, avanza según el reloj inyectado y
  `REPLAY_SPEED`; el resultado consultado corresponde al frame actual, no al
  final antes de que termine el replay.
- `ProviderManager` permite que `POST /admin/replay/start` sustituya el adapter
  activo. El archivo solicitado debe ser un archivo regular dentro del
  directorio configurado para los replays.
- El Sync Worker sincroniza fixtures, marcadores y resultados. Persiste los
  cambios en PostgreSQL y publica `MatchUpdated`, `MatchStarted` y
  `MatchFinished` después del commit.

Los resultados de eventos terminados usan exclusivamente `score.fulltime`
(incluye añadido, excluye prórroga y penales). El marcador `goals` se usa solo
para el estado en vivo.

## Fallos externos y presupuesto

- Los errores HTTP, JSON inválido, estados desconocidos, resultados finales sin
  marcador reglamentario y HTTP 429 se devuelven como errores explícitos; no se
  sustituyen por listas vacías.
- `provider_usage` persiste y aumenta atómicamente por proveedor y día UTC antes
  de cada request. Al fallar la llamada externa, el consumo permanece contado,
  evitando superar el límite por reintentos.
- Las consultas de fixtures/resultados se limitan a
  `PROVIDER_DAILY_BUDGET - PROVIDER_RESERVE`; los marcadores en vivo pueden usar
  el presupuesto completo. Replay no consume cuota externa.
- El worker registra errores y vuelve a intentar en el siguiente ciclo. El
  cierre de mercados por reloj y el barrido de settlement siguen funcionando
  aunque falle el proveedor. El endpoint
  `GET /admin/providers/status` expone uso y límite del día.

## Mercados y resolución

Al detectar eventos programados próximos se crean en una transacción los tres
mercados y sus siete selecciones. La cuota se guarda como entero centesimal
(`odd_x100`); las probabilidades Poisson sobre la grilla 0..10 se normalizan
antes de aplicar el margen y limitar el resultado a 1.01–50.00. No se usa
`float64` para almacenar dinero ni cuotas.

Los mercados se cierran al recibir `MatchStarted` y, como respaldo ante pérdida
del bus, por reloj cuando restan 60 segundos para el inicio. Para un resultado
reglamentario, `markets` resuelve HOME/DRAW/AWAY, OVER/UNDER y YES/NO. Los
estados POSTPONED, CANCELLED, ABANDONED y AWARDED anulan las selecciones.

## Escenarios ATAM 1, 2 y 4

**ATAM 1 — Actualización de un partido en vivo.** El estímulo es un cambio de marcador
   externo. El worker consulta según `SYNC_LIVE_INTERVAL`, persiste estado y
   marcador, y publica el evento después del commit; el SSE público lo reenvía.
   Si el bus pierde el aviso, el siguiente polling vuelve a guardar el estado.

**ATAM 2 — Proveedor externo caído o presupuesto agotado.** El adapter informa el
   error y el worker lo registra sin borrar el último dato persistido. El UPSERT
   diario rechaza requests al alcanzar el límite y reserva capacidad para
   marcadores; el worker vuelve a intentar en el ciclo siguiente.

**ATAM 4 — Resultado terminal o partido suspendido/anulado.** El estado no retrocede
   automáticamente desde terminal. FINISHED se resuelve con el marcador de
   tiempo reglamentario; POSTPONED, CANCELLED, ABANDONED y AWARDED producen
   VOID. El resultado queda disponible para el barrido de settlement incluso
   si se perdió `MatchFinished`.

## Verificación

Desde `backend`:

```bash
go test ./internal/events/... ./internal/markets/... ./internal/providers/...
../scripts/check-imports.sh
```
