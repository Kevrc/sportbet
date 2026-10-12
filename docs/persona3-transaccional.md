# Persona 3 — Núcleo transaccional

**GitHub:** @By-Alvarez06 · **Módulos:** betting, wallet, settlement

## Modelo de dominio

* **Wallet:** Billetera de saldo de los usuarios usando enteros de 64 bits para manejar centavos exactos (`money.Money`). Todos los abonos y retiros quedan registrados en `wallet_transactions` de manera obligatoria y consistente.
* **Betting:** Administra las apuestas de los usuarios validando cuotas, reglas de negocio (sin correlación, rangos permitidos) usando `money.Odds`. Mantiene los comprobantes y selecciones.
* **Settlement:** Proceso orquestador que lee resultados de mercado, re-evalúa apuestas pendientes y emite pagos.

## Flujo de apuesta (débito atómico, idempotencia)

El proceso de colocación de apuestas se divide de la siguiente forma:
1. Validaciones previas y combinación con los mercados (vía `MarketReader`).
2. Se procesan todas las cuotas combinadas truncando a 2 decimales para el pago potencial (`Combine`).
3. Se abre una Transacción (PGX TX).
4. Se debita usando un CTE (`WITH updated AS ... INSERT`) que evita problemas de concurrencia e impide dobles cargos usando la llave de `wallet_transactions`.
5. Se inserta en base de datos la apuesta `bets` protegiendo idempotencia usando la llave `Idempotency-Key` única junto con el `user_id`.

## Flujo de liquidación (WIN/LOSS/VOID, combinadas)

El proceso de `Settlement` se activa cuando un partido acaba (`MatchFinished`) o mediante el Sweep (barrido). Funciona así:
1. Pide al `ResultReader` la resolución de todas las selecciones del partido.
2. Actualiza el resultado individual (`WON`, `LOST`, `VOID`) de manera masiva en todos los detalles (`bet_selections`) donde están pendientes.
3. Encuentra todas las apuestas afectadas y las re-evalúa individualmente en transacciones separadas.
4. Si la apuesta tiene alguna selección LOST, pierde. Si sigue con alguna PENDING, espera. Si todas VOID, se reembolsa (`BET_REFUND`). Caso contrario, gana (`BET_PAYOUT`).
5. En la misma transacción donde actualiza el estado de la apuesta a terminal, hace `wallet.Credit`, impidiendo inconsistencias.

## Escenarios ATAM 3 y 5

Formato de escenario de atributo de calidad: fuente, estímulo, artefacto, entorno, respuesta y medida. La evaluación completa, con el árbol de utilidad y los compromisos, está en [`arquitectura/evaluacion-atam.md`](arquitectura/evaluacion-atam.md).

### ATAM 3 — Apuestas simultáneas sobre el mismo saldo (integridad)

| Elemento | Descripción |
|---|---|
| **Fuente** | Un usuario con dos pestañas, un doble clic o un script |
| **Estímulo** | Varias apuestas a la vez cuya suma supera el saldo disponible |
| **Artefacto** | `PlaceBet` (betting) y `WalletService.Debit` (wallet) sobre `wallets` |
| **Entorno** | Operación normal, con carga concurrente |
| **Respuesta** | Se aceptan solo las apuestas que el saldo cubre; las demás reciben `422 INSUFFICIENT_FUNDS` y no dejan rastro (ni apuesta ni movimiento) |
| **Medida** | Saldo nunca negativo; `wallets.balance_cents = SUM(wallet_transactions.amount_cents)`; con 20 apuestas simultáneas y saldo para una, se acepta exactamente una |

**Decisiones que lo resuelven:**
- **Débito atómico condicional.** `UPDATE wallets SET balance_cents = balance_cents - $1 WHERE user_id = $2 AND balance_cents >= $1` dentro de un CTE que además inserta el movimiento. La comprobación y el débito son una sola sentencia, así que no existe la ventana "leí el saldo, otro debitó, debito yo".
- **Una sola transacción para apuesta y débito.** `WalletService.Debit` recibe el `db.Querier` de `PlaceBet`, así que, si falla el débito, la apuesta no se inserta (y al revés).
- **Idempotencia del cliente.** `UNIQUE(user_id, idempotency_key)` en `bets`. Un reintento con la misma `Idempotency-Key` devuelve `200` con la apuesta original y no debita otra vez.
- **Cuota congelada.** El cliente envía la cuota que vio. Si cambió, se rechaza con `409 ODDS_CHANGED` y la apuesta guarda `odd_x100` del momento.

**Evidencia:** `loadtest/doble-gasto.js` lanza 20 apuestas simultáneas de 2.00 con saldo 3.00. Resultado: **1 aceptada, 19 `INSUFFICIENT_FUNDS`, saldo final 1.00**. La prueba de carga con 50 y 200 usuarios no tuvo errores (ver PASA en `persona1-plataforma.md`).

**Puntos de sensibilidad:** la cláusula `balance_cents >= $1` (sin ella hay doble gasto) y que el débito use la transacción de la apuesta.

### ATAM 5 — Liquidación correcta aunque se pierdan avisos o haya reintentos (recuperabilidad)

| Elemento | Descripción |
|---|---|
| **Fuente** | El bus en memoria o el reinicio del servidor |
| **Estímulo** | Se pierde el aviso `MatchFinished`, llega dos veces, o el barrido y el aviso liquidan la misma apuesta al mismo tiempo |
| **Artefacto** | `Settler` (settlement), `WalletService.Credit`, tablas `bets` y `wallet_transactions` |
| **Entorno** | Reinicio de la API, carga normal o partido que termina mientras la API está apagada |
| **Respuesta** | Toda apuesta de un partido terminado se liquida una sola vez (ganada, perdida o anulada) y se paga una sola vez |
| **Medida** | 0 pagos duplicados; ninguna apuesta queda `PENDING` más de un ciclo de barrido (5 min) después de que el resultado esté disponible |

**Decisiones que lo resuelven:**
- **Doble camino de activación.** El aviso `MatchFinished` del bus liquida al instante. Además, un barrido cada 5 minutos busca partidos terminados en las últimas 48 h con apuestas pendientes, así que un aviso perdido solo atrasa la liquidación.
- **Transición condicional.** `UPDATE bets … WHERE id = $3 AND status = 'PENDING'`. Si otra ejecución ya la liquidó, afecta 0 filas y no se paga.
- **Segunda barrera en el libro contable.** `UNIQUE(reference, type)` en `wallet_transactions`, con referencia `bet:<id>:v<n>`. Un crédito repetido devuelve `ErrAlreadyProcessed` y no suma saldo.
- **Todo en una transacción.** El cambio de estado de la apuesta y el crédito van juntos; no puede quedar "ganada sin pagar" ni "pagada y pendiente".
- **Anuncio después del COMMIT.** `BetSettled` se publica solo si esta ejecución liquidó la apuesta, para que la auditoría del admin no la cuente dos veces.

**Evidencia:** la prueba `TestSettleBetPublishesBetSettledOnce` llama dos veces a `SettleBet` sobre la misma apuesta contra PostgreSQL: un solo pago y un solo anuncio. En la prueba de punta a punta, dos barridos seguidos dejaron el saldo igual y la invariante del libro contable se cumplió.

**Riesgo conocido:** el barrido corre cada 5 minutos pero no al arrancar la API, así que tras un reinicio la primera recuperación puede tardar hasta 5 minutos.

## Correcciones e Integración (Frontend)

Durante la integración con el Frontend, se implementaron los siguientes ajustes:
* **Enrutamiento:** Se removió el prefijo redundante `/api/v1/` en los métodos de registro de rutas de `wallet`, `betting` y `settlement`, ya que el prefijo ya está inyectado por el enrutador principal en `main.go`.
* **Creación Automática de Billetera:** Si un usuario recién creado (como el admin o un usuario que todavía no ha depositado) intenta consultar su billetera en `GET /wallet` y no existe, se crea automáticamente con saldo inicial de `0` en vez de arrojar un error HTTP 500.
* **Historial de Transacciones Vacío:** Se corrigió el valor de retorno en `GET /wallet/transactions` para que devuelva un array vacío (`[]`) en lugar de `null` en caso de que no haya transacciones.
* **Consulta de Apuestas (`GET /bets`):** Se añadió el endpoint `GET /bets` y su handler correspondiente para devolver el historial de apuestas del usuario, permitiendo al frontend listarlas.

## Correcciones e Integración (Frontend) - Fase 2

Luego del rebase y validaciones adicionales del Frontend, se corrigieron los siguientes puntos para ajustar el contrato REST `openapi.yaml`:
* **Respuesta GET /bets y POST /bets:** Se ajustaron los handlers para retornar todos los campos requeridos en el contrato (`placedAt`, `selections` con la información del evento, etc.), utilizando una subconsulta `json_agg` directamente en PostgreSQL para recolectar las opciones y haciendo un `JOIN` a las tablas `selections`, `markets` y `events` a nivel base de datos.
* **Reintento Idempotente:** Si se intenta crear una apuesta con el mismo `Idempotency-Key`, la API devuelve un código HTTP `200 OK` con la apuesta original recuperada desde la base de datos, manejando correctamente la violación del *Unique Constraint* devolviendo el error `errs.ErrAlreadyProcessed`.
* **Manejo de Errores Específicos:** Se sustituyeron los errores base del dominio (como `ErrOddsChanged` o un genérico 500) por los errores estructurados definidos en `shared/errs` (como `INSUFFICIENT_FUNDS` o `ODDS_CHANGED`), lo que permite que el middleware responda con los códigos `422` o `409` adecuados.
* **Billetera al vuelo:** Se mantiene la creación de billetera al vuelo con `0` de saldo cuando el usuario (como el administrador sembrado) no la tiene.
* **Limpieza:** Se ha deshecho la edición accidental del contrato de tests de `wallet` para mantener las reglas de `AGENTS.md`, esto se mandará en un PR distinto si es necesario.
