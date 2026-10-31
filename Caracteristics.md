Actúa como un Arquitecto de Software y Desarrollador Full-Stack Senior experto en arquitecturas ligeras, seguras y de alto rendimiento. Tu objetivo es diseñar y generar el código base para un SaaS self-hosted destinado a la gestión integral de una agencia de desarrollo de software (venta de producto/licencia, sin entrega de código fuente).

### Stack Tecnológico Obligatorio
*   **Backend:** Go (Golang) usando el router `chi` o `echo`.
*   **Base de Datos:** SQLite (usando `gorm` o `sqlc`).
*   **Frontend:** Plantillas nativas de Go (`html/template`) renderizadas en el servidor.
*   **Interactividad:** HTMX para peticiones sin recarga (SPA-like) y Alpine.js para interactividad menor (modales, tabs).
*   **Estilos:** Tailwind CSS.
*   **Despliegue:** Docker (Dockerfile multi-stage incluido).
*   **Integraciones:** API de Google (Gemini) para IA, y API de mensajería (ej. WhatsApp/Twilio) para notificaciones.

### Arquitectura de Roles y Vistas

#### 1. Panel Global (Exclusivo Administrador)
*   **Dashboard Global:** Lista de todos los proyectos activos e inactivos. Permite seleccionar un proyecto para entrar a su "Espacio de Trabajo".
*   **Gestión de Accesos:** Panel para crear cuentas de Cliente. Genera contraseñas temporales y tiene un botón para "Restablecer Contraseña" (envía email).

#### 2. Espacio de Trabajo del Proyecto - VISTA ADMINISTRADOR (Control Total)
*   **Ficha Técnica Privada:** Campos para guardar enlace a Git, llaves API, accesos a servidores y fecha de inicio.
*   **Módulo SRS Dual (IA):** Asistido por Google Gemini. Permite generar y editar la "Vista Ejecutiva" (para el cliente) y la "Vista Técnica" (arquitectura, endpoints).
*   **Pantalla de Revisión IA -> Kanban:** La IA lee el SRS y propone tareas. El admin las revisa, une o modifica antes de pasarlas al Backlog.
*   **Tablero Kanban Técnico:** Tablero HTMX (Drag & Drop) de 4 columnas (Backlog, En Curso, Testeo, Terminado).
*   **Gestor de Entregables:** Panel para asociar Sprints completados a un enlace de prueba (Staging). El admin marca manualmente el entregable como "Aceptado" (tras aprobación verbal del cliente).
*   **Recursos:** Visualización y descarga de archivos subidos por el cliente.
*   **Finanzas (Edición):** Panel para definir Costo Base, registrar Pagos y agregar "Ajustes de Costo" con justificación.

#### 3. Portal Simplificado - VISTA CLIENTE (Solo lectura e interacción guiada)
*   *Nota: El cliente NO ve el Kanban técnico ni la ficha técnica privada.*
*   **Resumen del Proyecto:** Línea de tiempo visual, estado actual, fecha de inicio y un historial de texto simple de avances.
*   **Documentación:** Solo lectura del SRS Ejecutivo (Reglas de negocio, casos de uso).
*   **Entregables, Avances y Soporte (Unificado):**
    *   Historial visual de entregables aceptados.
    *   Entregable actual destacado con enlace al entorno de pruebas (Staging).
    *   **Caja de Notas / Tickets:** Un formulario en esta misma vista para dejar feedback o reportar bugs del entregable. **Acción:** Al enviar la nota, dispara un webhook (WhatsApp) al Admin.
*   **Recursos:** Nube simple para subir logos, manuales, etc. **Acción:** Al subir o actualizar un archivo, dispara webhook (WhatsApp/Notificación) al Admin.
*   **Finanzas (Lectura):** Visualización transparente del costo base, historial de pagos y cargos extra.

### Esquema de Base de Datos Sugerido
*   `Users`: id, name, email, password_hash, role (admin|client).
*   `Projects`: id, client_id, status, start_date, base_cost, git_repo, private_notes.
*   `SRS_Versions`: id, project_id, client_content, dev_content.
*   `Proposed_Tasks`: id, project_id, ai_title, ai_description, status.
*   `Tasks`: id, project_id, title, status (backlog|doing|testing|done).
*   `Deliverables`: id, project_id, url_demo, is_accepted.
*   `Tickets_Notes`: id, project_id, deliverable_id, client_message.
*   `Resources`: id, project_id, file_path, uploaded_at.
*   `Billings`: id, project_id, amount, is_extra, reason, paid_status.

### Entregables Requeridos en tu Respuesta
1.  **Estructura de Directorios:** Arquitectura limpia en Go.
2.  **Modelos de Base de Datos:** Código Go para crear las tablas con relaciones.
3.  **Lógica del Core (Controladores):**
    *   Flujo de Autenticación y separación de vistas (Admin vs Cliente).
    *   Flujo del Kanban con HTMX.
4.  **Servicios Externos:** Archivo `services.go` con las llamadas simuladas a Google Gemini (generación SRS/Tareas) y notificaciones de WhatsApp.
5.  **Vistas HTMX (Plantillas):** Un ejemplo funcional de la "Vista Entregables del Cliente" con el formulario que dispara la notificación.
6.  **Dockerfile:** Configuración óptima para producción.

---

# Estado de Implementación — v0.1

> Las reglas de negocio críticas y de seguridad están documentadas en **`specifics.md`** (fuente de verdad funcional).

## Decisiones técnicas confirmadas

| Área | Decisión |
|------|----------|
| Router | `chi` v5 |
| Persistencia | SQLite + `sqlc` (driver puro Go `modernc.org/sqlite`, sin CGO) |
| Migraciones | `golang-migrate` embebidas, se aplican al arrancar |
| Auth | JWT HS256 (access 15 min) + refresh rotativo hasheado en DB (7 días), cookies HttpOnly |
| IA | SDK oficial `google.golang.org/genai` (Gemini); mock determinista si no hay API key |
| WhatsApp | WhatsApp Business API (Graph API v19), plantillas configurables; log si no está configurado |
| Almacenamiento | Cloudflare R2 (S3 compatible, URLs prefirmadas); disco local como fallback |
| Email | Resend (API HTTP); log en desarrollo si no hay API key |
| Frontend | `html/template` + HTMX 1.9 + Alpine.js 3 + Tailwind CLI (build step) |
| Drag & Drop | SortableJS |
| Rate limiting | Token bucket por IP (`golang.org/x/time/rate`) |
| Tests | Integración con SQLite `:memory:` + `httptest` |

## Funcionalidades entregadas

- **Panel global admin**: métricas, listado de proyectos, creación de proyectos, gestión de clientes, contraseñas temporales y restablecimiento.
- **Workspace admin**: ficha técnica privada (Git, llaves, servidores, notas), SRS dual (ejecutivo/técnico) con generación IA y edición, revisión de propuestas IA → aprobar/descartar, Kanban de 4 columnas con drag & drop persistente, entregables (publicar/aceptar/eliminar), tickets con respuesta y estado, recursos (R2/local), finanzas editables.
- **Portal cliente**: resumen con timeline de avances, documentación ejecutiva de solo lectura, entregable actual destacado con enlace a staging, historial de aceptados, formulario de notas/bugs que dispara WhatsApp, carga de recursos que notifica al admin, finanzas transparentes de solo lectura.
- **Transversal**: autenticación por roles, refresh automático, CSRF double-submit, rate limiting, auditoría en `project_events`, degradación elegante de integraciones, seed de demo, Docker multi-stage, tests de integración.

## Cómo ejecutar

```bash
# 1. Dependencias
go mod download
npm install && npm run tailwind:build

# 2. Configuración
cp .env.example .env   # ajusta secretos e integraciones

# 3. Ejecutar
make run               # http://localhost:8080
```

Credenciales del seed de demo (`SEED_DEMO=true`, solo en base vacía):

- Admin: `admin@giodevportal.local` / `Admin123!`
- Cliente: `cliente@demo.local` / `Cliente123!`

## Docker

```bash
make docker-build
make docker-up     # http://localhost:8080
```

## Tests

```bash
make test          # integración con SQLite :memory:
go vet ./...
```

## Estructura

```
cmd/server/            entrypoint + wiring
internal/config/       carga y validación de entorno
internal/db/           apertura SQLite + migraciones embebidas
internal/repository/   código generado por sqlc
internal/service/      auth (JWT/bcrypt), email (Resend)
internal/handler/      rutas, middleware, controladores, seed, tests
internal/template/     renderer de plantillas y helpers
pkg/gemini/            cliente Gemini + mock
pkg/whatsapp/          cliente WhatsApp Business API
pkg/storage/           interfaz + local + Cloudflare R2
sql/                   schema, queries (sqlc) y migraciones
web/                   plantillas, CSS/JS (Tailwind, Sortable, HTMX)
docker/                Dockerfile multi-stage + compose
```

## Definición de terminado v0.1

- Login/logout/refresh y separación estricta de vistas por rol.
- Kanban funcional con persistencia de estado y orden.
- SRS dual generado por IA (o mock) y versionado.
- Entregables con aceptación manual del admin y vista cliente.
- Tickets del cliente → notificación WhatsApp (degradable a log).
- Recursos con R2 (o local) y descarga autorizada.
- Finanzas transparentes para el cliente y editables por el admin.
- CSRF, rate limiting, contraseñas temporales, reset de un solo uso.
- Migraciones automáticas, Docker y tests de integración en verde.
---

## Actualización v0.1.1 — Pagos, solicitudes SRS y ajustes visuales

### Finanzas del cliente
- El cliente puede **reportar un pago** con monto, nota y **comprobante obligatorio** (máx. 25 MB).
- El pago queda **pendiente de autorización**; solo el administrador puede autorizarlo o rechazarlo tras verificar el comprobante.
- Al autorizar se crea un movimiento pagado, se **suma al Pagado** y se **descuenta del saldo del costo base**; se muestra el saldo restante en ambas vistas.
- Se valida que el monto no supere el saldo pendiente del costo base. El comprobante se sirve con control de acceso.

### Recursos (control de versiones)
- **Solo el cliente sube archivos.** El administrador no tiene formulario ni ruta de subida; únicamente revisa, descarga y elimina.
- Se muestra el usuario que subió cada archivo (`uploaded_by`) y la fecha.
- Al eliminar, el admin borra el objeto del almacenamiento y el registro.

### Documentación / SRS
- El cliente puede **proponer actualizaciones** de objetivos/casos de uso desde Documentación.
- Las solicitudes quedan pendientes y **no modifican el SRS** hasta que el administrador las autoriza.
- Al autorizar se crea una **nueva versión** del SRS con la sección “Actualización aprobada”, preservando historial y vista técnica.
- *(v0.1.2 eliminó el refinamiento manual del Backlog; ver más abajo.)*

### Visual
- El orden del workspace es **SRS + IA primero, Kanban después**; el ingreso al proyecto y el botón del dashboard apuntan a SRS.
- Tema **morado más fuerte** (`primary-600 = #6d28d9`), sidebar morado profundo y acentos coherentes.
- Nueva carpeta **`web/static/img/`** con imágenes reemplazables: logo, favicon y estados vacíos. Ver `web/static/img/README.md`.

### Integraciones
- Nuevas plantillas WhatsApp configurables: `WHATSAPP_TEMPLATE_PAYMENT_ALERT` y `WHATSAPP_TEMPLATE_SRS_ALERT` (con degradación a log si no están configuradas).
- Migración `000002_finance_requests` añade `payment_claims` y `srs_requests`.

### Validación (v0.1.1 · histórica)
- `go vet ./...` limpio.
- 13 tests de integración con SQLite `:memory:` en verde.
- 24/24 pruebas E2E con Chrome headless.

---

## Actualización v0.1.2 — MXN, doble autorización SRS y Kanban por versión

### Moneda
- **Todos los importes son MXN (peso mexicano).** Se eliminaron los selectores de moneda del alta de proyecto y de la ficha técnica.
- Formato visual con prefijo **`MX$`**. El backend fuerza `currency = "MXN"` y la migración `000003` actualiza proyectos existentes.

### SRS + IA
- **“Generar SRS dual”** ahora **actualiza el SRS vigente** con lo propuesto en el cuadro de texto y guarda una **propuesta** (`srs_proposals`), sin publicar versión.
- La nueva versión se crea **solo cuando el administrador y el cliente autorizan** la propuesta (en cualquier orden).
- Se muestra el estado de ambas autorizaciones, el contenido propuesto y un historial de propuestas (`pending | applied | superseded`).
- Generar una nueva propuesta reemplaza (supersede) las pendientes anteriores.
- Las solicitudes del cliente siguen aplicándose al aprobarlas el admin (el cliente ya autorizó al solicitarlas).

### Kanban
- El administrador **asigna una versión del SRS al tablero** (`projects.kanban_srs_version_id`).
- **“Validar backlog”** compara la versión asignada con las tareas actuales y genera propuestas IA; el admin las aprueba o descarta desde el propio Kanban.
- Se eliminó el panel **“Refinar Backlog (manual)”** y la edición inline de tareas.
- El admin puede **eliminar tareas en cualquier columna**, incluidas En Curso, Testeo y Terminado.

### Finanzas del administrador
- Se eliminó la sección **“Registrar movimiento manual”** y la tabla de movimientos manuales.
- Solo se listan los **pagos dados de alta por el cliente**, con acceso al **comprobante**.
- El admin cambia el estatus entre **Pendiente** y **Pagado**:
  - Pagado: crea el movimiento, suma al Pagado y descuenta el saldo.
  - Pendiente: retira el movimiento, resta del Pagado y restaura el saldo.
- `payment_claims.status` pasa a `pending | paid` (migración `000003`).

### Técnico
- Migración `000003_mxn_srs_proposals_kanban`: MXN, columna `projects.kanban_srs_version_id`, tabla `srs_proposals` y nuevo CHECK de `payment_claims`.
- Nuevas rutas: propuestas SRS (admin/cliente), asignación/validación Kanban, pagos `paid`/`pending`.

### Validación v0.1.2
- `go vet ./...` limpio.
- **14 tests de integración** con SQLite `:memory:` en verde.
- **33/33 pruebas E2E** con Chrome headless: tema morado, orden SRS→Kanban, propuesta SRS sin publicar, doble autorización, versión publicada, Kanban vinculado/validado, MXN, pagos pagado/pendiente, recursos y control de acceso.
- Imagen Docker construida y contenedor probado (login, finanzas, SRS).
