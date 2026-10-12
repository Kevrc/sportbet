# ADR 0002 — Decisiones de integración con datos reales

- **Fecha:** 9 a 12 de octubre de 2026
- **Estado:** aceptada
- **Contexto general:** el ADR 0001 fijó la arquitectura base. Al conectar API-Football con una clave real aparecieron restricciones del plan gratuito que el diseño no contemplaba, y al integrar los módulos aparecieron huecos (avisos del bus que nadie publicaba, errores sin código). Este ADR registra cada decisión con su contexto y sus consecuencias. El detalle de implementación está en los PRs indicados.

## D1. Ajustarse a las restricciones del plan gratuito de API-Football

**Contexto.** Comprobado con la clave real (#11, #12):

| Restricción | Efecto |
|---|---|
| 100 llamadas al día y 10 por minuto | Límite duro de todo el sistema |
| `fixtures?date=` solo de ayer a mañana | Pedir pasado mañana falla y gasta una llamada |
| `standings` solo temporadas 2022–2024 | No hay tabla de la temporada actual |
| `fixtures?ids=` (varios por llamada) no permitido | Cada consulta por ID es una llamada |
| La respuesta trae `"errors": []` si va bien y un objeto si falla | El adapter lo leía como error en toda respuesta correcta |

**Decisión.** El calendario se pide solo para hoy y mañana. El adapter acepta `errors` como lista o como objeto. Cada uso de la API se diseña contando llamadas (ver D2, D3, D4 y D5).

**Consecuencias.** El sistema funciona con datos reales sin pagar, a costa de estrategias más elaboradas para no pasar de 100 llamadas. Pasar a un plan pago no cambia el código: solo `PROVIDER_DAILY_BUDGET`.

## D2. Marcador en vivo solo con partidos destacados en juego

**Contexto.** El Sync Worker consultaba `fixtures?live=all` cada 3 minutos siempre: unas 480 llamadas al día. En unas horas se agotaba el presupuesto y dejaban de confirmarse resultados, y las apuestas no se liquidaban.

**Decisión.** Solo se consulta en vivo si hay algún partido **destacado** en juego o a punto de empezar (`HasFeaturedInPlay`). El admin marca los destacados desde el panel. Es la regla que el plan ya pedía: "cero llamadas cuando no hay partidos destacados en juego".

**Consecuencias.** El gasto del vivo queda acotado por la decisión del admin, no por el reloj. Los partidos no destacados no tienen minuto a minuto, pero igual se actualizan al confirmar resultados (D3). Una sola consulta en vivo trae todos los partidos en juego, así que los demás partidos de nuestras ligas que estén en juego también se actualizan.

## D3. Confirmar resultados por fecha y no por partido

**Contexto.** La confirmación pedía cada partido por ID cada pocos minutos; con 10 partidos pendientes eran 10 llamadas por ciclo y se pasaba el límite de 10 por minuto.

**Decisión.** Con API-Football se pide **una vez por fecha** (que trae todos los partidos del día), como mucho cada 10 minutos. Las fechas anteriores a ayer, que el plan no deja consultar por fecha, se piden por ID.

**Consecuencias.** De N llamadas por ciclo a 1 o 2. El resultado llega con hasta 10 minutos de retraso, lo que es aceptable porque la liquidación no es en tiempo real.

## D4. Fuerza de los equipos desde la tabla de posiciones

**Contexto.** El modelo de cuotas (ADR 0001) usa una fuerza por equipo, pero todos entraban con 1.00 y todas las cuotas salían iguales (2.14 / 3.62 / 3.14).

**Decisión.**
- **Fórmula:** fuerza = √((GF + 3) / (GC + 3)), acotada a [0.60, 1.60], con los goles de la tabla. Se pide una llamada por liga cada 12 h.
- **Temporada:** como el plan gratuito no da la temporada actual, el adapter usa la última que permite (2024) como **fuerza histórica** y recuerda ese límite para no gastar otra llamada.
- **Recálculo:** las cuotas de los mercados abiertos se recalculan cuando cambia la fuerza.

**Consecuencias.**
- Las cuotas reflejan la calidad relativa de los equipos (Levante vs Sevilla: 1.80 / 3.84 / 4.06).
- Los recién ascendidos quedan en 1.00 porque no estaban en 2024.
- Las apuestas ya hechas conservan su cuota congelada; quien vea una cuota vieja recibe `ODDS_CHANGED`.

## D5. Línea de tiempo del partido (goles, tarjetas, cambios, VAR)

**Contexto.** `fixtures?live=all` ya traía los sucesos de cada partido y se descartaban. La consulta por fecha no los trae; la consulta por ID sí.

**Decisión.**
- **En vivo:** se guardan los sucesos que trae la consulta en vivo, sin costo extra (#13).
- **Al terminar:** cada partido terminado se pide **una vez** por ID para completar su línea de tiempo, como mucho 3 por ciclo de 10 minutos. `events.timeline_fetched_at` evita repetirlo aunque el partido no haya tenido sucesos (#14).
- **Actualización:** el frontend recarga la línea de tiempo cuando el SSE avisa un cambio del partido. No cambió el bus compartido.

**Consecuencias.** Unas 10–15 llamadas al día con 3 ligas. El flujo Sync Worker → PostgreSQL → bus → SSE → Angular queda visible de punta a punta en la demo.

## D6. Datos de presentación que ya venían gratis

**Decisión.** Se guardan los escudos de los equipos, el logo y la bandera de la liga, la jornada y el estadio (#11, #12). Solo se aceptan URLs `https`, porque terminan como `src` de una imagen.

**Consecuencias.** Mejor experiencia sin gastar llamadas. Las columnas son nullables: el replay no tiene estos datos y el frontend muestra iniciales en su lugar.

## D7. Auditoría y estadísticas alimentadas por el bus

**Contexto.** El plan definió los avisos `BetPlaced` y `BetSettled` para el panel de admin, pero nadie los publicaba, y el admin no puede leer las tablas de betting (regla de módulos).

**Decisión.**
- **Publicación:** betting y settlement publican los avisos **después del COMMIT**: solo apuestas nuevas y solo liquidaciones efectivas.
- **Consumo:** el módulo admin los guarda en `audit_log`, y de ahí salen `/admin/stats` y `/admin/audit` (#21).

**Consecuencias.** El admin no depende del esquema de betting. Pero el bus es en memoria: si la API se reinicia en el instante de una apuesta, ese aviso se pierde en la auditoría, aunque la apuesta y su pago quedan bien. Las estadísticas cuentan desde que se activó esta función.

## D8. Errores de dominio con el código del contrato

**Contexto.** Los errores de dominio de betting (mercado cerrado, monto fuera de rango, combinada con dos selecciones del mismo partido…) eran `errors.New` y salían como **500**.

**Decisión.** El handler los traduce a los códigos de `openapi.yaml`: `422 MARKET_CLOSED`, `422 CORRELATED_SELECTIONS` y `400 VALIDATION_ERROR` con el campo (#21).

**Consecuencias.** El boleto del frontend muestra mensajes útiles en lugar de "error inesperado".

## D9. Endurecimiento de la autenticación

Surgió de la revisión de código (#3):
- **Contraseña:** mínimo 8 **caracteres** y máximo 72 **bytes** (límite de bcrypt). Si se excede responde 400, no 500.
- **Refresh token:** rotación atómica. Un solo `UPDATE … WHERE revoked_at IS NULL … RETURNING`, así que de dos refresh simultáneos con el mismo token solo uno gana.
- **Un solo reloj:** el vencimiento del JWT, del refresh token y de la cookie salen del mismo reloj inyectado (`app.Deps.Clock`).
- **Gestión de usuarios:** `/admin/users` vive en el módulo `users`, que es el dueño de la tabla, y no en `admin`. Un admin no puede quitarse el rol ni bloquearse a sí mismo.
