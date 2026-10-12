# Evaluación de la arquitectura (ATAM)

ATAM (*Architecture Tradeoff Analysis Method*, SEI) evalúa una arquitectura contra escenarios de atributos de calidad. El resultado son sus **puntos de sensibilidad**, sus **compromisos** (*tradeoffs*), sus **riesgos** y sus **no-riesgos**.

Este documento reúne los escenarios que cada persona documentó en su módulo y agrega el análisis global. El porqué de cada tecnología está en [justificacion.md](justificacion.md).

## 1. Factores de negocio

- Casa de apuestas con **dinero virtual**: el dinero no es real, pero un doble cobro o un pago doble invalidan el producto.
- **Datos deportivos reales** con un proveedor gratuito (100 llamadas al día).
- **Equipo de 4 personas y entrega en 5 días**: la arquitectura debe permitir trabajar en paralelo y ser simple de operar.

## 2. Árbol de utilidad

Prioridad (importancia para el negocio, dificultad técnica): A = alta, M = media, B = baja.

| Atributo | Refinamiento | Escenario | Prioridad |
|---|---|---|---|
| **Integridad** | Saldo y pagos exactos | **E3** — apuestas simultáneas sobre el mismo saldo | (A, A) |
| | | **E5** — liquidación sin pagos dobles aunque se pierdan avisos | (A, A) |
| **Disponibilidad** | Proveedor externo caído o sin cuota | **E2** — API-Football falla o se agota el presupuesto | (A, M) |
| **Frescura del dato** | Marcador en vivo | **E1** — un gol se ve en la app | (A, M) |
| **Corrección del dominio** | Partidos anulados o suspendidos | **E4** — partido suspendido, aplazado o terminado | (A, M) |
| **Seguridad** | Sesiones y roles | **E6** — token robado o usuario bloqueado | (A, M) |
| **Rendimiento** | Navegar y apostar bajo carga | **E7** — 50 usuarios simultáneos | (M, B) |
| **Modificabilidad** | Cambiar el proveedor de datos | **E8** — reemplazar API-Football | (M, B) |

Los escenarios **(A, A)** son los que más riesgo tenían y los que más pruebas recibieron.

## 3. Escenarios

Formato: fuente → estímulo → artefacto → entorno → respuesta → **medida**.

### E1 — Actualización de un partido en vivo *(Persona 2, detalle en [`persona2-datos.md`](../persona2-datos.md))*

- **Fuente / estímulo:** API-Football registra un gol en un partido destacado.
- **Artefacto / entorno:** Sync Worker, `events`, bus y SSE; operación normal.
- **Respuesta:** el worker guarda marcador y sucesos, publica `MatchUpdated` después del COMMIT y el SSE lo empuja a los navegadores, que actualizan marcador y línea de tiempo sin recargar.
- **Medida:** el dato llega en ≤ `SYNC_LIVE_INTERVAL` (3 min) y la app marca como "desactualizado" un dato de más de 5 min. *Verificado con el replay: la línea de tiempo creció a la par del marcador sin recargar.*

### E2 — Proveedor caído o presupuesto agotado *(Persona 2)*

- **Fuente / estímulo:** API-Football responde error, 429, o se llega al límite diario.
- **Artefacto / entorno:** adapter, decorador de presupuesto y Sync Worker; operación degradada.
- **Respuesta:** el adapter informa el error tipado, el worker lo registra y **no borra** el último dato. Los usuarios siguen navegando y apostando con lo que hay en PostgreSQL. El contador persistido corta las llamadas aunque la API se reinicie.
- **Medida:** 0 errores visibles para el usuario por fallas del proveedor; nunca más de 100 llamadas al día. *El día de pruebas se usaron 58–77 de 100, con reinicios incluidos.*

### E3 — Apuestas simultáneas sobre el mismo saldo *(Persona 3, detalle en [`persona3-transaccional.md`](../persona3-transaccional.md))*

- **Fuente / estímulo:** un usuario lanza varias apuestas a la vez cuya suma supera su saldo.
- **Artefacto / entorno:** `PlaceBet` y `WalletService.Debit`; carga concurrente.
- **Respuesta:** solo se aceptan las que el saldo cubre; el resto recibe `422 INSUFFICIENT_FUNDS` sin dejar rastro.
- **Medida:** saldo nunca negativo; con 20 simultáneas y saldo para 1, **exactamente 1 aceptada** *(medido con k6)*.

### E4 — Partido suspendido, aplazado o terminado *(Persona 2)*

- **Fuente / estímulo:** el proveedor informa FT, AET, PEN, PST, CANC, ABD o AWD.
- **Artefacto / entorno:** `events`, `markets`, `settlement`.
- **Respuesta:** los estados terminales no retroceden. FINISHED se liquida con el marcador de tiempo reglamentario (sin prórroga ni penales); aplazado, cancelado, abandonado o adjudicado anulan las selecciones (VOID) y la combinada sigue con cuota 1.00 en esa selección.
- **Medida:** 0 apuestas liquidadas con un marcador que incluya prórroga; 100 % de las selecciones de partidos anulados en VOID.

### E5 — Liquidación con avisos perdidos o repetidos *(Persona 3)*

- **Fuente / estímulo:** se pierde `MatchFinished` (bus en memoria, reinicio) o la liquidación corre dos veces a la vez.
- **Artefacto / entorno:** `Settler`, `bets`, `wallet_transactions`.
- **Respuesta:** el barrido cada 5 min recupera lo perdido; `UPDATE … WHERE status = 'PENDING'` y `UNIQUE(reference, type)` impiden pagar dos veces.
- **Medida:** 0 pagos duplicados; ninguna apuesta `PENDING` más de 5 min después de tener resultado. *Verificado: dos liquidaciones seguidas dan un solo pago y un solo anuncio.*

### E6 — Token robado o usuario bloqueado *(Persona 1, detalle en [`persona1-plataforma.md`](../persona1-plataforma.md))*

- **Fuente / estímulo:** un atacante reutiliza un refresh token, o el admin bloquea a un usuario con sesión abierta.
- **Artefacto / entorno:** `auth`, middleware JWT y `users`.
- **Respuesta:** el refresh token rota en cada uso (reusar el viejo da 401; de dos refresh simultáneos solo uno gana). El usuario bloqueado no puede renovar su sesión (403).
- **Medida:** un token robado sirve como máximo hasta su próximo uso legítimo; un usuario bloqueado pierde el acceso en ≤ 15 min (vida del token de acceso). *Verificado con pruebas de integración y manuales.*

### E7 — Carga de 50 usuarios simultáneos *(Persona 1, análisis PASA)*

- **Fuente / estímulo:** 50 usuarios navegan, consultan su billetera y apuestan durante 1 min.
- **Artefacto / entorno:** API completa y PostgreSQL; carga normal.
- **Respuesta:** todas las peticiones se atienden sin errores.
- **Medida:** p95 < 300 ms al leer y < 500 ms al apostar; errores < 1 %. *Medido: **2.6 ms y 8.0 ms**, 0 errores; con 200 usuarios, 2.7 ms y 14.9 ms.*

### E8 — Reemplazar el proveedor de datos *(transversal)*

- **Fuente / estímulo:** el equipo quiere usar otro proveedor (por ejemplo, football-data.org o un plan pago).
- **Artefacto / entorno:** módulo `providers`; tiempo de desarrollo.
- **Respuesta:** se escribe un nuevo adapter que implemente `FixtureProvider` y `LiveScoreProvider`; ningún otro módulo cambia.
- **Medida:** 1 archivo nuevo más su registro en `providers.New`; 0 cambios en `events`, `markets`, `betting`. *Evidencia: API-Football y el replay ya conviven detrás de las mismas interfaces.*

## 4. Enfoques arquitectónicos y su análisis

| Enfoque | Escenarios | Sensibilidad | Compromiso | Riesgo |
|---|---|---|---|---|
| **Débito atómico condicional en una transacción con la apuesta** | E3, E7 | S1 | — | — (verificado) |
| **Sync Worker único + lectura desde la base** | E1, E2, E7 | S2 | C1 | — |
| **Bus de eventos en memoria** | E1, E5 | S3 | C2 | R1, R2 |
| **Liquidación doble (aviso + barrido) e idempotente** | E5 | S4 | — | R3 |
| **Cuotas internas y cierre de mercados al inicio** | E1, E3 | — | C3 | — |
| **JWT corto + refresh rotado** | E6 | S5 | C4 | R4 |
| **Monolito modular con contratos** | E7, E8 | — | C5 | — |
| **Presupuesto del proveedor persistido** | E2 | S6 | C1 | R5 |

### Puntos de sensibilidad

| ID | Punto | Por qué es sensible |
|---|---|---|
| S1 | La condición `balance_cents >= monto` del débito y que use la transacción de la apuesta | Sin ella hay doble gasto (E3) |
| S2 | `SYNC_LIVE_INTERVAL` | Define a la vez la frescura del dato (E1) y el gasto de llamadas (E2) |
| S3 | Tamaño del búfer de cada suscriptor del bus (256) | Si se llena, se descartan avisos (E1, E5) |
| S4 | Intervalo del barrido de liquidación (5 min) | Tiempo máximo de recuperación ante un aviso perdido (E5) |
| S5 | Vida del token de acceso (15 min) | Ventana en que un usuario bloqueado o un token robado siguen sirviendo (E6) |
| S6 | Reserva del presupuesto (13 llamadas) y frecuencia de confirmación de resultados (10 min) | Reparto de las 100 llamadas entre vivo, calendario y resultados (E2) |

### Compromisos (*tradeoffs*)

| ID | Compromiso | Qué se ganó | Qué se cedió |
|---|---|---|---|
| C1 | **Frescura vs presupuesto**: vivo cada 3 min y solo con destacados en juego; resultados cada 10 min | Cumplir 100 llamadas al día | Minuto a minuto solo para partidos destacados; resultados con hasta 10 min de retraso |
| C2 | **Bus en memoria vs durabilidad** | Simplicidad: sin broker que operar | Los avisos se pueden perder (mitigado con barrido y SSE de reconexión); una sola instancia de la API |
| C3 | **Cerrar mercados al inicio** | Nadie apuesta con información que la casa todavía no tiene (dato en vivo con hasta 3 min de retraso) | No hay apuestas en vivo |
| C4 | **JWT sin estado vs revocación inmediata** | El middleware no consulta la base en cada request | Un usuario bloqueado conserva hasta 15 min su token de acceso |
| C5 | **Monolito modular vs microservicios** | Transacciones ACID locales, un despliegue, trabajo en paralelo con dueños claros | No se escalan módulos por separado (no hace falta según E7) |

### Riesgos

| ID | Riesgo | Mitigación actual | Qué haría falta |
|---|---|---|---|
| R1 | El bus en memoria pierde avisos si la API se reinicia o un búfer se llena | Barrido de liquidación; recarga de `/events/live` al reconectar el SSE | Bus durable (Postgres `LISTEN/NOTIFY` + outbox, o Redis) |
| R2 | La auditoría del admin puede perder una apuesta si se pierde `BetPlaced` | Las apuestas y los pagos quedan bien; solo afecta las estadísticas | Outbox transaccional para los avisos de apuestas |
| R3 | El barrido no corre al arrancar: tras un reinicio, hasta 5 min sin recuperar | El aviso en vivo cubre el caso normal | Ejecutar el barrido también al iniciar |
| R4 | Sin límite de intentos de login | bcrypt encarece cada intento | Rate limiting por IP y por cuenta |
| R5 | Dependencia del plan gratuito: sin temporada actual en tablas; fuerza histórica de 2024 | Fuerza acotada a [0.60, 1.60]; los recién ascendidos quedan en 1.00 | Plan pago o un segundo proveedor detrás del mismo Adapter |

### No-riesgos (decisiones verificadas)

- **Doble gasto:** imposible con el débito atómico (k6: 1 de 20).
- **Pagos duplicados:** imposible con la transición condicional más `UNIQUE(reference, type)` (prueba de integración).
- **Errores de redondeo:** no ocurren; dinero y cuotas son enteros.
- **Acoplamiento entre módulos:** imposible de introducir sin que falle el CI (`check-imports.sh`).
- **Rendimiento:** dos órdenes de magnitud por debajo de los objetivos con 4× la carga esperada.

## 5. Temas de riesgo

1. **Durabilidad de la comunicación asíncrona (R1, R2, R3).** El bus en memoria es el punto débil de la arquitectura. Está mitigado para lo que importa (el dinero), no para lo accesorio (estadísticas). Es la primera mejora si el sistema creciera.
2. **Dependencia de un proveedor gratuito (R5, C1).** La arquitectura la aísla bien (Adapter + Decorador de presupuesto), pero la calidad del dato depende del plan contratado.
