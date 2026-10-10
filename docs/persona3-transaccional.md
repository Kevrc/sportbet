# Persona 3 — Núcleo transaccional

**GitHub:** @By-Alvarez06 · **Módulos:** betting, wallet, settlement

## Modelo de dominio

* **Wallet:** Billetera de saldo de los usuarios usando enteros de 64 bits para manejar centavos exactos (`money.Money`). Todos los abonos y retiros quedan registrados en `wallet_transactions` de manera obligatoria y consistente.
* **Betting:** Administra las apuestas de los usuarios validando cuotas, reglas de negocio (sin correlación, rangos permitidos) usando `money.Odds`. Mantiene los comprobantes y selecciones.

## Flujo de apuesta (débito atómico, idempotencia)

El proceso de colocación de apuestas se divide de la siguiente forma:
1. Validaciones previas y combinación con los mercados (vía `MarketReader`).
2. Se procesan todas las cuotas combinadas truncando a 2 decimales para el pago potencial (`Combine`).
3. Se abre una Transacción (PGX TX).
4. Se debita usando un CTE (`WITH updated AS ... INSERT`) que evita problemas de concurrencia e impide dobles cargos usando la llave de `wallet_transactions`.
5. Se inserta en base de datos la apuesta `bets` protegiendo idempotencia usando la llave `Idempotency-Key` única junto con el `user_id`.

## Flujo de liquidación (WIN/LOSS/VOID, combinadas)

_Pendiente (módulo settlement)._

## Escenarios ATAM 3 y 5

_Pendiente._
