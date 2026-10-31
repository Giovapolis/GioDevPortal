# GioDevPortal — Reglas de Negocio Críticas (v0.1)

Este documento es la fuente de verdad para el comportamiento funcional y de seguridad.
Si una implementación contradice estas reglas, la implementación es incorrecta.

---

## 1. Roles, vistas y aislamiento

| Rol | Acceso | Ve | No ve nunca |
|-----|--------|----|-------------|
| `admin` | `/admin/**` | Todos los proyectos, ficha técnica, kanban, SRS dual, finanzas editables, tickets, recursos | — |
| `client` | `/client/**` (solo sus proyectos) | Resumen, documentación ejecutiva, entregables, recursos, finanzas de solo lectura | Ficha técnica, kanban, SRS técnico, llaves API, notas privadas, otros proyectos |

Reglas:

1. El rol se guarda en `users.role` y **nunca** se confía en el cliente. Cada request se valida en servidor.
2. El middleware `RequireRole` bloquea `/admin` a clientes y `/client` a administradores (HTTP 403).
3. Toda operación sobre un proyecto recalcula propiedad: `client_id == user.id` para clientes; admin siempre.
4. El portal del cliente nunca renderiza datos de la ficha técnica ni el `dev_content` del SRS.
5. Un usuario que no es dueño del proyecto recibe `403`; un recurso inexistente recibe `404`.

---

## 2. Autenticación, sesión y contraseñas

1. **Access token**: JWT HS256, TTL 15 min (configurable), en cookie `gdp_access` `HttpOnly`, `SameSite=Lax`, `Secure` cuando `ENV=production`.
2. **Refresh token**: 32 bytes aleatorios, se guarda **hasheado (SHA-256)** en `refresh_tokens`. TTL 7 días. Cookie `gdp_refresh` `HttpOnly`.
3. El access token se renueva automáticamente si expiró y el refresh es válido, no revocado y no expirado.
4. **Logout** revoca el refresh token y limpia cookies.
5. **Cambio de contraseña** y **reset** revocan todos los refresh tokens del usuario.
6. Las cuentas de cliente se crean con contraseña temporal y `must_change_password = 1`; al ingresar son redirigidas a `/change-password` hasta cambiarla.
7. La contraseña temporal se muestra **una sola vez** al administrador y se envía por correo. No se almacena en texto plano.
8. **Reset de contraseña**: token aleatorio de un solo uso, guardado hasheado, válido **1 hora**. Se invalida al usarse.
9. La recuperación de contraseña responde siempre el mismo mensaje (no revela si el correo existe).
10. Contraseñas: mínimo **8 caracteres**; hash **bcrypt** (coste por defecto).
11. **CSRF**: patrón double-submit. Todo `POST/PUT/PATCH/DELETE` exige `csrf_token` (form) o `X-CSRF-Token` (header) igual a la cookie `gdp_csrf`. Si falta o no coincide → **403**.
12. **Rate limiting por IP** (token bucket):
    - Login: 10 req/min, burst 5 → exceso **429**.
    - Generación IA (SRS/tareas): 10 req/min, burst 3.
    - Tickets de cliente: 20 req/min, burst 10.

---

## 3. SRS dual asistido por IA

1. El administrador escribe notas en lenguaje natural y solicita generación.
2. La generación crea una **nueva versión** (`srs_versions.version` incremental). Nunca sobrescribe versiones previas.
3. Cada versión contiene:
   - `client_content`: vista ejecutiva (reglas de negocio, casos de uso, alcance). **Única visible al cliente.**
   - `dev_content`: vista técnica (arquitectura, modelo de datos, endpoints, criterios). **Solo admin.**
   - `source_notes`: notas originales de la solicitud.
4. Ambas vistas pueden editarse manualmente y guardarse.
5. **Propuestas de tareas**: la IA lee el SRS y crea filas en `proposed_tasks` con estado `pending`.
6. Una propuesta **no** es una tarea: el admin debe **aprobar** (crea tarea en `backlog`) o **descartar** (estado `rejected`). No hay aprobación automática.
7. Sin `GEMINI_API_KEY`, el sistema usa un generador determinista (mock) para no bloquear el flujo. La UI lo indica como “IA mock/IA activa”.
8. **Solicitudes de actualización del cliente**: el cliente puede proponer cambios de objetivos/casos de uso desde “Documentación”. Quedan en `srs_requests` con estado `pending`.
9. La solicitud **no** modifica el SRS hasta que el administrador la autoriza.
10. Al aprobar una solicitud del cliente se crea una **nueva versión** del SRS con `client_content` = contenido actual + sección “Actualización aprobada”; se conserva `dev_content`; se registra `srs_version_id`. El cliente ya autorizó al solicitarla. Al rechazar no cambia el SRS.
11. El botón **“Generar SRS dual”** del administrador **no publica una versión**: actualiza el SRS actual con las nuevas notas del cuadro de texto y guarda una **propuesta** en `srs_proposals`.
12. La propuesta se publica como nueva versión solo cuando **el administrador y el cliente** la autorizan, en cualquier orden. Mientras tanto, la versión vigente no cambia.
13. Cada propuesta registra `admin_approved` y `client_approved`; al completarse ambas se crea la versión final y la propuesta pasa a `applied`.
14. Generar una nueva propuesta marca las propuestas pendientes anteriores como `superseded`.
15. El cliente ve el contenido ejecutivo propuesto y el estado de ambas autorizaciones; el administrador ve ambas vistas.
16. El cliente ve el estado de sus solicitudes y la nota del administrador.

---

## 4. Kanban

1. Cuatro columnas fijas y únicas: `backlog`, `doing`, `testing`, `done`.
2. Solo el administrador puede crear, mover o eliminar tareas.
3. El orden dentro de una columna se persiste en `tasks.column_order`.
4. Un estado inválido es rechazado con **400**.
5. Mover una tarea entre columnas registra un evento en el timeline del proyecto (solo si cambia de estado).
6. Las tareas no se muestran nunca al cliente.
7. El administrador **asigna una versión del SRS al tablero** (`projects.kanban_srs_version_id`); el Kanban lee esa versión como referencia.
8. La acción **“Validar backlog”** compara la versión asignada con las tareas actuales y genera propuestas IA en `proposed_tasks`.
9. El administrador aprueba o descarta las propuestas; al aprobar pasan al Backlog.
10. El administrador puede **eliminar tareas en cualquier columna**, incluyendo En Curso, Testeo y Terminado.
11. No existe edición inline de tareas ni el panel “Refinar Backlog” en v0.1.2.

---

## 5. Entregables

1. El administrador publica un entregable con: sprint, título, URL de staging y notas. La **URL de staging es obligatoria**.
2. El cliente ve como “entregable actual” el **último entregable no aceptado** (por sprint descendente).
3. El historial muestra únicamente entregables **aceptados**.
4. **Solo el administrador acepta** un entregable (después de aprobación verbal del cliente). El cliente no tiene esa acción.
5. Eliminar un entregable es solo del administrador.
6. Aceptar un entregable registra `accepted_at` y un evento en el timeline.

---

## 6. Tickets / notas del cliente

1. El formulario de nota está disponible en la vista de entregables **solo si existe un entregable actual**.
2. El mensaje es obligatorio y tiene un máximo de **2000 caracteres**.
3. Crear un ticket:
   - persiste en `tickets_notes` con estado `open`,
   - registra un evento en el timeline,
   - dispara notificación **asíncrona** por WhatsApp al administrador (no bloquea la respuesta).
4. El cliente ve sus notas y su estado; el administrador puede cambiar `open | in_progress | closed` y dejar una respuesta interna.
5. Si WhatsApp no está configurado, la notificación se registra en logs (degradación controlada), sin fallar la operación.

---

## 7. Recursos (archivos)

1. Tamaño máximo por archivo: **25 MB**. Archivos vacíos se rechazan.
2. La clave de almacenamiento es `projects/{project_id}/{timestamp}-{slug}{ext}` (slug del nombre original).
3. Con Cloudflare R2 el bucket es **privado**; la descarga se hace con URL prefirmada de **1 hora**.
4. Sin R2 configurado, se usa disco local en `DATA_DIR/uploads` y la descarga pasa por `/files/{key}`.
5. La descarga exige autenticación y **verifica propiedad del proyecto** (admin o cliente dueño).
6. **Solo el cliente sube archivos.** El administrador no puede subir: la ruta POST de admin no existe y el servidor rechaza la operación. El admin puede revisar, descargar y eliminar.
7. Cada recurso muestra el usuario que lo subió (`uploaded_by`) para mantener el control de versiones.
8. Al eliminar un recurso, el admin borra el objeto de almacenamiento y el registro. El cliente no puede eliminar.
9. Cuando el **cliente** sube un archivo, se notifica al administrador por WhatsApp.
10. El nombre original se conserva para mostrar y descargar; la clave se sanea.

---

## 8. Finanzas

1. **Moneda única: MXN (peso mexicano).** No hay selector de moneda ni soporte para otras divisas en la UI. Todos los importes se muestran con prefijo `MX$`.
2. El administrador define el **costo base** del proyecto desde la ficha técnica.
3. El administrador **no registra movimientos manuales**. Su sección de Finanzas muestra únicamente los **pagos dados de alta por el cliente**.
4. **Pago reportado por el cliente**: requiere monto > 0, nota opcional y **comprobante obligatorio** (máximo 25 MB).
5. El monto **no puede superar el saldo del costo base** (costo base menos pagos base pagados). Se valida al reportar y al marcar pagado.
6. Todo pago reportado inicia en estado `pending` y queda a la espera de revisión.
7. El administrador puede **ver el comprobante** y cambiar el estatus entre `pending` y `paid` (pendiente/pagado).
8. Al **marcar pagado**: se crea un movimiento base `paid` (suma al Pagado) y se descuenta el saldo del costo base.
9. Al **marcar pendiente**: se elimina el movimiento generado, se resta del Pagado y se restaura el saldo. El pago permanece visible para el cliente.
10. El cliente tiene **lectura total y transparente**: costo base, pagado, saldo del costo base, extras, historial y pagos reportados.
11. El cliente **no puede** crear, editar ni eliminar movimientos contables directamente. Solo puede **reportar un pago** con comprobante.
12. Totales mostrados: pagado, saldo del costo base, pendiente de movimientos y suma de extras.
13. El comprobante se sirve por `/payment-proofs/{id}` con verificación de propiedad (administrador o cliente dueño).
14. **Saldo del costo base** = costo base − pagos base pagados (nunca negativo).

---

## 9. Auditoría y datos

1. `project_events` registra acciones clave: proyecto creado, ficha actualizada, SRS generado/editado, propuestas SRS creadas/autorizadas/publicadas, tareas propuestas/creadas/movidas/eliminadas, validación de backlog, entregable publicado/aceptado, ticket creado, recurso subido/eliminado, pago reportado/pagado/pendiente.
2. El cliente ve el timeline como “Historial de avances” (texto simple).
3. Todas las fechas se guardan en **UTC RFC3339** y se muestran en hora local.
4. Borrar un proyecto elimina en cascada SRS, tareas, propuestas, entregables, tickets, recursos y movimientos.
5. Los archivos en R2 no se borran automáticamente al borrar filas (v0.1.2); requiere limpieza operativa.

---

## 10. Integraciones y degradación

| Integración | Configurada | Sin configurar |
|-------------|-------------|----------------|
| Google Gemini | Generación real con SDK oficial | Mock determinista |
| WhatsApp Business API | Envía plantillas aprobadas | Log de la notificación |
| Cloudflare R2 | Subida/descarga vía S3 API + presigned | Disco local |
| Resend | Envía correo HTML | Log con enlace/credenciales |

Reglas:

1. Ninguna integración externa puede dejar la app inoperante si falta configuración.
2. Las plantillas de WhatsApp (`ticket_alert`, `resource_alert`, `payment_alert`, `srs_request_alert`) deben estar aprobadas en Meta Business; los nombres son configurables por entorno.
3. Los secretos viven solo en `.env` / variables de entorno. **Nunca** se commitean.

---

## 11. Seguridad operativa

1. En producción: HTTPS obligatorio y `ENV=production` para cookies `Secure`.
2. SQLite con WAL, `foreign_keys=ON`, `busy_timeout=5s` y pool limitado a 1 conexión (escritor único).
3. El contenedor corre como usuario no root y persiste `/app/data` en volumen.
4. Backups: respaldar el volumen de datos (SQLite + uploads locales).
5. Las migraciones se aplican automáticamente al arrancar (`golang-migrate`, embebidas).

---

## 12. Alcance y límites conocidos de la v0.1.2

Implementado: autenticación JWT con refresh, roles, panel global, clientes, ficha técnica, **SRS dual con actualización sobre el contenido vigente y doble autorización admin/cliente**, propuestas IA → Kanban, **Kanban con versión SRS asignada y validación de backlog**, drag & drop, eliminación de tareas en cualquier columna, entregables, tickets con notificación, **pagos reportados por el cliente con comprobante y estatus pendiente/pagado controlado por el admin**, solicitudes de actualización del SRS, recursos R2/local con **subida exclusiva del cliente** y gestión/eliminación del admin, **finanzas en MXN con saldo del costo base**, timeline, rate limiting, CSRF, migraciones, Docker, tests de integración y E2E.

Pendiente / fuera de alcance v0.1.2:

- 2FA, SSO, multi-tenant real, múltiples clientes por proyecto.
- Edición inline de tareas, subtareas/checklists y dependencias.
- Multimoneda (solo MXN).
- Movimientos financieros manuales del administrador (solo pagos reportados por el cliente).
- WebSockets o notificaciones en tiempo real.
- Paginación y búsqueda avanzada.
- Borrado de objetos en R2 al eliminar registros.
- Facturación/PDF, integración contable.
- Aprobación del cliente desde el portal (la aceptación es manual del admin).


---

## 13. Recursos visuales

1. Las imágenes de la interfaz viven en `web/static/img/` y se sirven desde `/static/img/...`.
2. Se pueden reemplazar sin tocar código manteniendo el nombre: `logo.svg`, `favicon.svg`, `empty-projects.svg`, `empty-docs.svg`, `empty-deliverables.svg`, `empty-files.svg`.
3. La paleta primaria es morada (`primary-600 = #6d28d9`); cualquier recurso nuevo debe respetar esa paleta para mantener coherencia.
4. El orden visual del workspace es **SRS + IA primero, Kanban después**.
