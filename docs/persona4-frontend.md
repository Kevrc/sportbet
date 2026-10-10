# Persona 4 — Frontend

**GitHub:** @jeimytorresg · **Módulos:** features/events, features/betting, features/wallet

Este documento cubre tres partes: [usabilidad](#usabilidad), [casos de uso](#casos-de-uso) y [manual de usuario](#manual-de-usuario). Lo que está medido o probado se dice como tal; lo que falta por ejecutar aparece marcado como pendiente.

## Usabilidad

### Objetivo

La propuesta fija un requisito de usabilidad: **encontrar un evento y realizar una apuesta en pocos pasos**. Se tradujo a una meta concreta de diseño:

> Desde la lista de eventos, una apuesta simple se hace en **4 acciones**: (1) abrir el partido, (2) tocar una cuota, (3) escribir el monto, (4) pulsar "Apostar".

Esa cifra sale del diseño del recorrido, no de una prueba con usuarios. Sin sesión iniciada se añade el registro o el login, y el boleto se conserva mientras tanto (ver CU-05).

### Heurísticas aplicadas

| Heurística (Nielsen)                    | Cómo se aplicó en la interfaz                                                                                                                                                                                                                                                                                                             |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Visibilidad del estado del sistema      | El boleto muestra al instante la cuota total y el pago potencial. El botón dice "Apostando…" mientras se envía. Aviso "Sin conexión en tiempo real" si se cae el SSE y etiqueta "Desactualizado" si un partido en vivo lleva más de 5 minutos sin datos. Estados de carga (`aria-busy`) en listas                                         |
| Correspondencia con el mundo real       | Cuotas con dos decimales (`2.14`), montos con separador de miles (`1 000.00`), estados traducidos ("Ganada", "Anulada", "En vivo"), mercados con nombre legible ("Resultado final (1X2)")                                                                                                                                                 |
| Control y libertad del usuario          | Cada selección se quita con ✕ o tocando la cuota otra vez, "Vaciar" limpia el boleto y el monto, "Cerrar" descarta el mensaje de resultado. Los filtros tienen "Limpiar filtros"                                                                                                                                                          |
| Consistencia y estándares               | Mismas pantallas de error con "Reintentar", mismos estados vacíos y misma paginación en billetera, historial y eventos. En las pantallas de esta persona, los botones que vuelven a pedir datos se llaman "Actualizar" para no confundirlos con "recargar saldo"                                                                          |
| Prevención de errores                   | El botón "Apostar" está deshabilitado si el monto es inválido, supera el saldo o el pago pasa del máximo. El boleto rechaza dos selecciones del mismo partido y más de 5 selecciones antes de llegar al servidor. Un doble clic envía una sola apuesta. Un reintento tras un corte de red no duplica la apuesta (misma `Idempotency-Key`) |
| Reconocer antes que recordar            | El saldo disponible está junto al campo de monto. Cada selección del boleto muestra partido, mercado y opción. Las reglas (1.00 a 500.00) aparecen como ayuda bajo el campo                                                                                                                                                               |
| Flexibilidad y eficiencia               | Se aceptan `10`, `10.5` y `10,50` como monto. Filtros combinables por fecha, liga, estado y destacados                                                                                                                                                                                                                                    |
| Diseño estético y minimalista           | Una acción principal por pantalla; el boleto solo muestra los datos de decisión                                                                                                                                                                                                                                                           |
| Ayuda para reconocer y corregir errores | Mensajes en español que dicen qué pasó y qué hacer (por ejemplo, ante una cuota cambiada se muestra la cuota anterior tachada y la nueva, y se pide confirmar). Los errores del servidor por campo se muestran junto al campo                                                                                                             |
| Ayuda y documentación                   | Este manual. Cada pantalla explica su regla principal en una línea ("Saldo virtual: no se maneja dinero real")                                                                                                                                                                                                                            |

### Accesibilidad

- Cada cuota es un `<button>` con `aria-label` ("Local, cuota 2.14") y `aria-pressed` cuando está en el boleto.
- Los errores usan `role="alert"` y los avisos de estado `role="status"`. El campo de monto indica su error con `aria-invalid` y `aria-describedby`.
- El estado de una apuesta se comunica con **texto** además de color (Ganada, Perdida, Anulada, Pendiente).
- Foco visible en botones y enlaces, formularios con `<label>` asociado y tablas con `<caption>` oculto.
- Diseño adaptable: el detalle pasa de dos columnas (mercados y boleto) a una sola por debajo de 900 px, con una barra fija que lleva al boleto, y las cuotas pasan a mostrarse en columna por debajo de 480 px. Las filas de la lista se reacomodan por debajo de 560 px.

### Tolerancia a fallos (disponibilidad)

Alineado con el requisito de tolerar fallos del proveedor externo:

- Si falla la carga de una lista, se muestra el error y "Reintentar"; nunca se presenta un fallo como "no hay datos". Esto incluye el saldo de la billetera.
- Si se cae el SSE, la pantalla sigue funcionando con los datos cargados, lo avisa, y al reconectar vuelve a pedir `GET /events/live` y los datos de la página.
- Si el servidor no responde al apostar (red o 5xx), el boleto se conserva y el reintento es seguro.

### Pruebas automáticas

El frontend completo tiene 161 pruebas con Vitest (incluye las de login, administración y SSE de otras personas). Las de esta persona cubren:

| Área                           | Qué verifican                                                                                                                                                                                                                          |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `money`, `odds`                | Formato y conversión de dinero sin coma flotante; cuota total con truncado paso a paso, tope de 500.00 y el ejemplo del ADR (10.00 × 2.14 × 1.97 = 42.10)                                                                              |
| `BetSlipStore`                 | Añadir, quitar y reemplazar; rechazo del mismo partido y más de 5 selecciones; validación del monto; doble clic; clave de idempotencia (se reutiliza ante error de red y cambia si el boleto cambia); cuota cambiada y mercado cerrado |
| `bet-errors`, `BettingService` | Cada código de error del contrato se traduce al aviso correcto; URL, parámetros y cabecera `Idempotency-Key`                                                                                                                           |
| `LiveScoresStore`              | Mensajes del SSE aplicados a los partidos; se ignoran los mensajes viejos y no se revive un partido terminado; recarga al reconectar                                                                                                   |
| Páginas y componentes          | Lista de eventos y filtros, detalle con mercados cerrados, panel del boleto, historial y billetera: estados vacío, de error con reintento y paginación                                                                                 |

### Pruebas manuales

Plan de escenarios para ejecutar contra el backend real (`npm start`) o contra un mock. La columna de cobertura indica si ya hay una prueba automática del comportamiento; la última columna se completa al ejecutar cada escenario.

| #   | Escenario                                       | Resultado esperado                                            | Cobertura automática                                                                                                                                                                                                                                     | Resultado manual                              |
| --- | ----------------------------------------------- | ------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| M1  | Encontrar un evento y apostar                   | 4 acciones desde la lista de eventos                          | Parcial (boleto y apuesta por separado)                                                                                                                                                                                                                  | Pendiente                                     |
| M2  | Apostar sin saldo suficiente                    | Aviso claro; saldo y boleto intactos                          | Sí                                                                                                                                                                                                                                                       | Pendiente                                     |
| M3  | La cuota cambia con el boleto abierto           | Se muestra la cuota nueva y se pide confirmar                 | Sí                                                                                                                                                                                                                                                       | Pendiente (requiere backend que devuelva 409) |
| M4  | Combinada con dos selecciones del mismo partido | Se impide en el boleto, con aviso                             | Sí                                                                                                                                                                                                                                                       | Pendiente                                     |
| M5  | Cortar la red durante el SSE                    | "Sin conexión en tiempo real"; al volver se recarga el estado | Sí                                                                                                                                                                                                                                                       | Pendiente                                     |
| M6  | Partido que empieza con el boleto abierto       | La selección queda como no disponible al intentar apostar     | Parcial                                                                                                                                                                                                                                                  | Pendiente                                     |
| M7  | Doble clic en "Apostar"                         | Una sola apuesta                                              | Sí                                                                                                                                                                                                                                                       | Pendiente                                     |
| M8  | Pantalla de 360 px                              | Sin desplazamiento horizontal; boleto accesible               | Medido con Chrome automatizado (2026-10-10): sin desplazamiento horizontal en login, registro, eventos, detalle, boleto, historial y billetera                                                                                                           | Ejecutado (ver nota)                          |
| M9  | Recorrido solo con teclado                      | Se puede apostar completo                                     | Ejecutado con Chrome automatizado contra el backend real (2026-10-10): Tab hasta la cuota, Enter la selecciona, Tab hasta el monto, se escribe y Tab hasta "Apostar" + Enter registra la apuesta; todo elemento enfocado tiene indicador de foco visible | Ejecutado (ver nota)                          |
| M10 | Ir a iniciar sesión con el boleto armado        | Se vuelve a la pantalla anterior con el boleto intacto        | Sí (el botón lleva a login con `returnUrl`; el boleto vive en un store de sesión)                                                                                                                                                                        | Pendiente                                     |

Nota sobre M8 y M9: se ejecutaron con Chrome controlado por script (protocolo DevTools), no a mano en un celular real. Verifican desbordamiento horizontal, orden de tabulación, activación con teclado e indicadores de foco; no sustituyen una revisión visual en un dispositivo ni una prueba con lector de pantalla. En la barra superior (archivo compartido, fuera de este módulo) los enlaces miden unos 21 px de alto, menos que lo recomendable para el tacto.

### Limitaciones conocidas

- La lista de eventos **no muestra cuotas**: el contrato (`GET /events`) devuelve eventos sin mercados y pedir los de cada partido multiplicaría las peticiones. Las cuotas y el boleto están en el detalle del evento.
- El boleto solo aparece en la pantalla de detalle. En pantallas angostas queda debajo de los mercados, por eso una barra fija con el resumen (cantidad de selecciones y cuota) lleva hasta él; no hay un contador en la barra superior.
- Un partido que empieza con el boleto abierto no se marca como cerrado hasta que el servidor rechaza la apuesta.
- El historial de apuestas depende de `GET /bets`, que aún no está disponible en el backend; la pantalla muestra un error con "Reintentar" mientras tanto.
- No se hicieron pruebas de usabilidad con usuarios reales.

## Casos de uso

**Actores:** _Visitante_ (sin sesión), _Usuario_ (con sesión; hereda lo del visitante) y _Sistema_ (backend y proveedor deportivo). Requisitos de la propuesta cubiertos: RF04, RF05, RF06, RF09, RF10, RF11 y RF12.

### CU-01 Consultar eventos

- **Actor:** Visitante o Usuario.
- **Precondición:** ninguna.
- **Flujo principal:**
  1. El actor abre "Eventos".
  2. El sistema muestra los partidos agrupados por liga (destacadas primero), con hora, equipos, estado y marcador si existe.
  3. El actor puede filtrar por fecha, liga, estado (programados, en vivo, finalizados) o solo destacados.
- **Alternos:** sin resultados, se avisa que no hay eventos (con o sin filtros). Si falla la carga, se muestra el error con "Reintentar".
- **Resultado:** el actor identifica el partido de interés.

### CU-02 Ver el detalle de un evento y sus mercados

- **Actor:** Visitante o Usuario.
- **Precondición:** el evento existe.
- **Flujo principal:**
  1. El actor elige un partido de la lista.
  2. El sistema muestra equipos, liga, fecha, estado y marcador, y los mercados (Resultado final 1X2, Total de goles 2.5, Ambos equipos anotan) con sus cuotas.
- **Alternos:** si el mercado está cerrado, liquidado o anulado, o el partido ya empezó, las cuotas se deshabilitan y se indica el motivo. Si el evento no existe, se informa. Si falla la carga, "Reintentar".
- **Resultado:** el actor conoce las cuotas vigentes.

### CU-03 Seguir el marcador en vivo

- **Actor:** Visitante o Usuario. **Secundario:** Sistema (SSE).
- **Precondición:** hay partidos en juego.
- **Flujo principal:**
  1. Con la lista o el detalle abiertos, el sistema recibe `match.started`, `match.updated` y `match.finished`.
  2. La pantalla actualiza estado, marcador y minuto sin recargar. Al empezar el partido, sus cuotas se deshabilitan.
- **Alternos:** si se cae la conexión se avisa y, al volver, se recargan los datos. Si el dato en vivo tiene más de 5 minutos, se marca "Desactualizado". Los mensajes más viejos que el estado conocido se ignoran y un partido terminado no vuelve a "en vivo".
- **Resultado:** el actor ve el estado del partido casi en tiempo real.

### CU-04 Armar el boleto

- **Actor:** Visitante o Usuario.
- **Precondición:** un mercado abierto en el detalle de un evento (CU-02).
- **Flujo principal:**
  1. El actor toca una cuota; el sistema la añade al boleto y la marca.
  2. Con una selección, la apuesta es **simple**; con 2 a 5, **combinada**. El sistema calcula la cuota total y el pago potencial.
- **Alternos:**
  - Tocar de nuevo la misma cuota la quita.
  - Elegir otra opción del mismo mercado reemplaza a la anterior.
  - Elegir otro mercado del mismo partido se rechaza con aviso (no hay combinadas correlacionadas).
  - Pasar de 5 selecciones se rechaza con aviso.
- **Resultado:** boleto con 1 a 5 selecciones de partidos distintos.

### CU-05 Realizar una apuesta

- **Actor:** Usuario. **Secundario:** Sistema.
- **Precondición:** boleto con al menos una selección y sesión iniciada. Sin sesión, el botón lleva a iniciar sesión y el boleto se conserva.
- **Flujo principal:**
  1. El actor escribe el monto (entre 1.00 y 500.00, sin superar su saldo).
  2. El sistema valida el monto y muestra el pago potencial (máximo 10 000.00).
  3. El actor pulsa "Apostar". El sistema envía el monto y las cuotas que el actor vio, con una clave de idempotencia.
  4. El sistema confirma. El boleto se vacía, se muestra la confirmación (monto, cuota total y pago potencial) y el saldo se actualiza.
- **Alternos:**

| Situación                                                   | Comportamiento                                                                                                                                                                |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Cuota cambiada (409 `ODDS_CHANGED`)                         | El sistema relee las cuotas vigentes, muestra la anterior tachada y la nueva, y bloquea "Apostar" hasta que el actor acepta las cuotas nuevas. Nunca se reenvía por su cuenta |
| Mercado cerrado                                             | La selección se marca como cerrada y hay que quitarla para apostar                                                                                                            |
| Saldo insuficiente                                          | Aviso; el boleto y el monto se conservan y el saldo se vuelve a leer                                                                                                          |
| Selecciones del mismo partido (422 `CORRELATED_SELECTIONS`) | Aviso; el boleto no se borra                                                                                                                                                  |
| Datos inválidos (400)                                       | Aviso con los errores por campo                                                                                                                                               |
| Sin conexión o error del servidor                           | Aviso; el boleto se conserva y el reintento usa la misma clave, por lo que no duplica la apuesta                                                                              |
| Sesión vencida (401)                                        | Aviso con enlace para iniciar sesión                                                                                                                                          |
| Doble clic                                                  | Se envía una sola apuesta                                                                                                                                                     |

- **Resultado:** la apuesta queda registrada con la cuota congelada y el monto se descuenta del saldo.

### CU-06 Consultar el historial de apuestas

- **Actor:** Usuario.
- **Precondición:** sesión iniciada.
- **Flujo principal:**
  1. El actor abre "Mis apuestas".
  2. El sistema lista sus apuestas con tipo, fecha, estado, selecciones (con la cuota congelada y el resultado de cada una), monto, cuota total y pago.
  3. El actor puede filtrar por estado (pendientes, ganadas, perdidas, anuladas) y paginar.
- **Alternos:** sin apuestas se invita a ver eventos. Si falla la carga, error con "Reintentar". Una selección anulada cuenta como cuota 1.00.
- **Resultado:** el actor conoce el estado y el pago de cada apuesta.

### CU-07 Consultar la billetera

- **Actor:** Usuario.
- **Precondición:** sesión iniciada.
- **Flujo principal:**
  1. El actor abre "Billetera".
  2. El sistema muestra el saldo disponible y los movimientos (fecha, tipo, referencia, monto con signo y saldo posterior), paginados.
- **Alternos:** si falla el saldo se muestra "No disponible" con aviso y "Reintentar" (no se presenta como billetera vacía). Si no hay movimientos se informa.
- **Resultado:** el actor ve cómo se movió su saldo. No hay depósitos ni retiros: el saldo es virtual.

### CU-08 Registrarse e iniciar sesión

Estas pantallas pertenecen a la Persona 1 y se documentan en su archivo; aquí se mencionan porque CU-05, CU-06 y CU-07 las requieren. Al volver de iniciar sesión, el actor regresa a la pantalla donde estaba.

## Manual de usuario

SportBet es una plataforma de **apuestas simuladas con saldo virtual**: no se usa dinero real. Funciona en el navegador, en computador y en celular.


### 1. Crear una cuenta e iniciar sesión

1. Abre la aplicación y pulsa **Entrar** (arriba a la derecha).
2. Si no tienes cuenta, elige crear una: escribe tu email, un nombre visible y una contraseña de **mínimo 8 caracteres**.
3. Al terminar entras con tu sesión iniciada y recibes un saldo virtual inicial (en desarrollo, 1 000.00).

Para salir, pulsa **Salir** en la barra superior.


### 2. Ver los eventos

La pantalla **Eventos** muestra los partidos agrupados por liga.

- Cada fila indica la hora, los equipos, el estado (Programado, En vivo con el minuto, Finalizado…) y el marcador si ya lo hay.
- Usa los filtros para acotar: **Fecha**, **Liga**, **Estado** y **Solo destacados**. Pulsa **Limpiar filtros** para quitarlos.
- **Actualizar** vuelve a pedir la lista.
- Si ves "Sin conexión en tiempo real", los marcadores pueden no estar al día; se actualizan solos cuando vuelva la conexión.


### 3. Ver los mercados de un partido

Pulsa un partido para abrir su detalle. Verás hasta tres mercados:

| Mercado               | Qué apuestas                                                 |
| --------------------- | ------------------------------------------------------------ |
| Resultado final (1X2) | Quién gana: local, empate o visitante (tiempo reglamentario) |
| Total de goles (2.5)  | Si habrá más o menos de 2.5 goles                            |
| Ambos equipos anotan  | Si los dos equipos marcan o no                               |

La **cuota** es lo que recibes por cada 1.00 apostado si aciertas: con cuota 2.14 y un monto de 10.00, el pago es 21.40. Cada mercado se cierra poco antes de que empiece el partido: no hay apuestas en vivo. Un mercado cerrado se ve deshabilitado y dice el motivo.

### 4. Armar el boleto

1. Toca una cuota: se añade al **Boleto** (a la derecha, o debajo en el celular) y se resalta.
2. Para quitarla, toca la cuota otra vez o pulsa ✕. **Vaciar** limpia todo.

Reglas:

- **Simple:** una selección. **Combinada:** de 2 a 5 selecciones; las cuotas se multiplican y debes acertarlas todas.
- No puedes combinar dos selecciones del **mismo partido**.
- Si eliges otra opción del mismo mercado, reemplaza a la anterior.
- La cuota total tiene un máximo de 500.00 y el pago potencial, de 10 000.00.


### 5. Apostar

1. Escribe el **monto** (entre 1.00 y 500.00 y sin superar tu saldo, que verás debajo del campo). Puedes usar punto o coma: `10`, `10.50`, `10,50`.
2. Revisa la **cuota total** y el **pago potencial**.
3. Pulsa **Apostar**. Si no has iniciado sesión, el botón te lleva a hacerlo y tu boleto se conserva.
4. Verás "¡Apuesta realizada!" con el resumen. Tu saldo se actualiza.


### 6. Avisos que puedes ver al apostar

| Aviso                                                  | Qué significa                        | Qué hacer                                                        |
| ------------------------------------------------------ | ------------------------------------ | ---------------------------------------------------------------- |
| "La cuota cambió" (la anterior aparece tachada)        | La cuota se movió desde que la viste | Revisa la nueva y pulsa **Aceptar cuotas nuevas**; luego apuesta |
| "Mercado cerrado: quítala para apostar"                | Ese mercado ya no admite apuestas    | Quita esa selección con ✕                                        |
| "Saldo insuficiente" / "No tienes saldo suficiente"    | El monto supera tu saldo             | Baja el monto                                                    |
| "No puedes combinar dos selecciones del mismo partido" | Regla de las combinadas              | Deja una sola selección por partido                              |
| "No hay conexión con el servidor…"                     | Se cortó la conexión                 | Pulsa **Apostar** otra vez: no se duplicará la apuesta           |
| "Tu sesión venció"                                     | Debes iniciar sesión de nuevo        | Pulsa **Iniciar sesión**; tu boleto se conserva                  |

### 7. Ver tus apuestas

**Mis apuestas** lista tus apuestas. De cada una verás:

- El tipo (Simple o Combinada), la fecha y el **estado**:
  - **Pendiente:** el partido aún no termina.
  - **Ganada:** acertaste; se te pagó.
  - **Perdida:** no acertaste (se pierde el monto apostado).
  - **Anulada:** el partido se suspendió o canceló; se te devuelve el monto.
- Cada selección, con la **cuota que tenía cuando apostaste** (no cambia aunque la cuota se mueva después) y su resultado.
- El monto, la cuota total y el pago potencial o recibido.

Filtra por estado con el selector y usa **Actualizar** para volver a pedir la lista.


### 8. Tu billetera

**Billetera** muestra el **saldo disponible** y la lista de **movimientos**: saldo inicial, apuestas (resta), premios y reembolsos (suma) y ajustes del administrador. Cada movimiento indica el monto con su signo y el saldo que quedó después.

### 9. Preguntas frecuentes

**¿Puedo recargar o retirar dinero?** No. El saldo es virtual y no hay depósitos ni retiros.

**¿Por qué no puedo apostar en un partido que ya empezó?** Los mercados se cierran poco antes del inicio; no hay apuestas en vivo.

**¿Por qué dice "Sin conexión en tiempo real"?** Perdió la conexión que actualiza los marcadores. Los datos mostrados pueden estar atrasados y se corrigen solos al volver.

**¿Por qué una apuesta combinada puede salir "Anulada" en parte?** Si una selección se anula, cuenta como cuota 1.00; si todas se anulan, se devuelve el monto.

**Pagué una apuesta y no veo el saldo cambiar.** Pulsa **Actualizar** en la billetera. Si el saldo muestra "No disponible", usa **Reintentar**.
