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

_Pendiente._

## Correcciones e Integración (Frontend)

Durante la integración con el Frontend, se implementaron los siguientes ajustes:
* **Enrutamiento:** Se removió el prefijo redundante `/api/v1/` en los métodos de registro de rutas de `wallet`, `betting` y `settlement`, ya que el prefijo ya está inyectado por el enrutador principal en `main.go`.
* **Creación Automática de Billetera:** Si un usuario recién creado (como el admin o un usuario que todavía no ha depositado) intenta consultar su billetera en `GET /wallet` y no existe, se crea automáticamente con saldo inicial de `0` en vez de arrojar un error HTTP 500.
* **Historial de Transacciones Vacío:** Se corrigió el valor de retorno en `GET /wallet/transactions` para que devuelva un array vacío (`[]`) en lugar de `null` en caso de que no haya transacciones.
* **Consulta de Apuestas (`GET /bets`):** Se añadió el endpoint `GET /bets` y su handler correspondiente para devolver el historial de apuestas del usuario, permitiendo al frontend listarlas.
