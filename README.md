# GioDevPortal

SaaS self-hosted para gestionar una agencia de desarrollo: panel global de proyectos, workspace técnico para el administrador y portal de solo lectura para el cliente. Venta de producto/licencia (sin entrega de código fuente). Todos los importes se manejan en **MXN** (peso mexicano).

**Stack:** Go · Chi · SQLite + sqlc · HTMX · Alpine.js · Tailwind CLI · Cloudflare R2 · Gemini · WhatsApp Business API · Resend · Docker.

Las reglas de negocio y seguridad están en **[`specifics.md`](./specifics.md)**. El diseño original y el estado de la v0.1 están en **[`Caracteristics.md`](./Caracteristics.md)**.

---

## Requisitos

- Go 1.26+ (el toolchain se descarga automáticamente si hace falta)
- Node 18+ (solo para compilar Tailwind)
- Docker (opcional, para despliegue)
- `sqlc` (solo si modificas `sql/`)

## Inicio rápido

```bash
cp .env.example .env
go mod download
npm install && npm run tailwind:build
make run
```

Abre http://localhost:8080.

Con `SEED_DEMO=true` y base de datos vacía se crean:

| Rol | Correo | Contraseña |
|-----|--------|------------|
| Admin | `admin@giodevportal.local` | `Admin123!` |
| Cliente | `cliente@demo.local` | `Cliente123!` |

> Cambia `JWT_SECRET`, `SEED_ADMIN_PASSWORD` y las credenciales de integraciones antes de producción.

## Variables de entorno

Todas se documentan en `.env.example`. Resumen:

| Variable | Uso |
|----------|-----|
| `ENV` | `development` o `production` (activa cookies `Secure`) |
| `SERVER_ADDR` | dirección HTTP (por defecto `:8080`) |
| `DB_PATH` / `DATA_DIR` | SQLite y archivos locales |
| `JWT_SECRET` | secreto HS256 (mínimo 16 caracteres; usa 32+) |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | duración de tokens |
| `SEED_*` | admin inicial y datos demo |
| `GEMINI_API_KEY` / `GEMINI_MODEL` | IA; sin key usa mock |
| `WHATSAPP_*`, `ADMIN_WHATSAPP_NUMBER` | notificaciones (tickets, recursos, pagos, solicitudes SRS); sin config usa log |
| `R2_*` | Cloudflare R2; sin config usa disco local |
| `RESEND_API_KEY`, `EMAIL_FROM`, `APP_BASE_URL` | correos; sin key escribe el enlace en log |

## Comandos

```bash
make run            # servidor en :8080
make build          # binario en bin/
make tailwind       # compila CSS (requerido tras cambiar plantillas)
make sqlc           # regenera el repositorio desde sql/
make test           # tests de integración (SQLite :memory:)
make test-race      # tests con -race
make docker-build   # imagen Docker
make docker-up      # docker compose up
make fmt            # gofmt
```

## Arquitectura

```
cmd/server/              entrypoint, wiring y graceful shutdown
internal/config/         carga y validación de .env
internal/db/             SQLite (WAL, FK) + migraciones embebidas
internal/domain/         constantes, etiquetas y helpers de formato
internal/repository/     código generado por sqlc (no editar)
internal/service/auth/   JWT, bcrypt, tokens opacos
internal/service/email/  Resend + fallback a log
internal/handler/        rutas, middleware, controladores, seed, tests
internal/template/       renderer html/template + FuncMap
pkg/gemini/              SDK oficial de Google + mock
pkg/whatsapp/            Graph API (plantillas)
pkg/storage/             Store interface + local + R2
sql/                     schema.sql, queries/, migrations/
web/templates/           layouts, páginas y parciales HTMX
web/static/css/          input.css (fuente) y output.css (compilado)
web/static/img/          imágenes de interfaz reemplazables (logo, favicon, estados vacíos)
docker/                  Dockerfile multi-stage + compose
```

### Flujo de petición

1. `middleware`: request id, real IP, logger, recoverer.
2. `CSRF`: asegura cookie `gdp_csrf` y valida mutaciones (double-submit).
3. Rutas públicas: login, recuperación y reset.
4. `Authenticate`: valida access token; si expiró, renueva con refresh token.
5. `RequireRole`: aísla `/admin` y `/client`.
6. Handler: valida propiedad del proyecto, ejecuta consultas y renderiza (página completa o parcial HTMX).

### Base de datos

15 tablas: `users`, `projects`, `srs_versions`, `srs_proposals`, `srs_requests`, `proposed_tasks`, `tasks`, `deliverables`, `tickets_notes`, `resources`, `billings`, `payment_claims`, `password_resets`, `refresh_tokens`, `project_events`.

Regenerar tras cambiar SQL:

```bash
sqlc generate
```

Las migraciones viven embebidas en `internal/db/migrations` y se aplican al arrancar. Para una nueva migración crea `000002_descripcion.up.sql` / `.down.sql` en `sql/migrations` **y** copia a `internal/db/migrations`.

## Rutas principales

| Método | Ruta | Rol |
|--------|------|-----|
| GET/POST | `/login`, `/forgot-password`, `/reset-password` | público |
| POST | `/logout`, `/change-password` | autenticado |
| GET | `/admin` | admin |
| GET/POST | `/admin/clients`, `/admin/projects` | admin |
| GET/POST | `/admin/projects/{id}/technical-sheet` | admin |
| GET/POST | `/admin/projects/{id}/srs`, `/srs/generate`, `/srs/save`, `/srs/tasks/generate` | admin |
| POST | `/admin/projects/{id}/srs/proposals/{proposalID}/approve` | admin (autoriza propuesta SRS) |
| POST | `/admin/projects/{id}/proposed/{taskID}/approve\|reject` | admin |
| GET/POST | `/admin/projects/{id}/kanban[/srs][/validate][/tasks][/tasks/{id}/move][/delete]` | admin |
| GET/POST | `/admin/projects/{id}/deliverables[/{id}/accept]` | admin |
| GET/POST | `/admin/projects/{id}/tickets[/{id}/status]` | admin |
| GET | `/admin/projects/{id}/resources` | admin (solo gestiona/elimina) |
| GET | `/admin/projects/{id}/finances` | admin (solo pagos del cliente) |
| GET | `/client`, `/client/projects/{id}[/docs\|/deliverables\|/resources\|/finances]` | client |
| POST | `/client/projects/{id}/deliverables/{id}/tickets` | client |
| POST | `/client/projects/{id}/resources` | client (único rol que sube archivos) |
| POST | `/client/projects/{id}/payments` | client (reporta pago + comprobante) |
| POST | `/client/projects/{id}/requests` | client (solicita actualización SRS) |
| POST | `/client/projects/{id}/srs/proposals/{proposalID}/approve` | client (autoriza propuesta SRS) |
| GET | `/files/{key}` | admin/cliente dueño |
| GET | `/payment-proofs/{claimID}` | admin/cliente dueño |

### Nuevas rutas de gestión (admin)

| Método | Ruta | Acción |
|--------|------|--------|
| POST | `/admin/projects/{id}/payments/{claimID}/paid` | Marca pagado: suma al Pagado, descuenta saldo y crea movimiento |
| POST | `/admin/projects/{id}/payments/{claimID}/pending` | Marca pendiente: retira el movimiento y restaura el saldo |
| POST | `/admin/projects/{id}/srs/proposals/{proposalID}/approve` | Autoriza la propuesta como administrador |
| POST | `/admin/projects/{id}/srs/requests/{requestID}/approve` | Aplica una solicitud del cliente creando una nueva versión |
| POST | `/admin/projects/{id}/srs/requests/{requestID}/reject` | Rechaza la solicitud del cliente |
| POST | `/admin/projects/{id}/kanban/srs` | Asigna/desvincula una versión del SRS al tablero |
| POST | `/admin/projects/{id}/kanban/validate` | Valida el backlog contra la versión asignada (IA) |
| POST | `/admin/projects/{id}/resources/{resourceID}/delete` | Elimina recurso del cliente |

## Integraciones

### Gemini
Si `GEMINI_API_KEY` está presente se usa el SDK oficial (`google.golang.org/genai`) con salida JSON estructurada. `UpdateSRS` fusiona el SRS vigente con las nuevas notas y `ValidateBacklog` compara la versión asignada con las tareas actuales. Sin key, el sistema usa un generador determinista para que el flujo sea probable sin dependencias externas.

### WhatsApp Business API
Se envían plantillas (`ticket_alert`, `resource_alert`, `payment_alert`, `srs_request_alert`) a `ADMIN_WHATSAPP_NUMBER` vía Graph API. Las plantillas deben aprobarse en Meta Business Manager. Sin configuración, las notificaciones se registran en el log.

### Cloudflare R2
Bucket privado. Subida server-side mediante AWS SDK v2 (endpoint R2) y descarga con URL prefirmada de 1 hora. Sin credenciales, se usa `DATA_DIR/uploads`.

### Resend
Correos de bienvenida y reset vía API HTTP. Sin key, el enlace/credenciales se escriben en el log (útil en desarrollo).

## Tests

```bash
make test
```

Los tests levantan SQLite en memoria (`:memory:`), aplican migraciones, siembran datos y ejercitan el router con `httptest` y cookies reales:

- login por rol y aislamiento admin/cliente,
- movimiento y reordenamiento de tareas en Kanban,
- CSRF obligatorio,
- ticket de cliente persistido,
- generación de propuesta SRS sin publicar versión,
- doble autorización admin + cliente para publicar la versión,
- solicitud de actualización del SRS + aprobación creando nueva versión,
- asignación de versión SRS al Kanban + validación de backlog,
- rate limiting de login,
- pago reportado por cliente + toggle pagado/pendiente,
- moneda forzada a MXN,
- admin no puede subir recursos; sí puede eliminarlos.

**14 tests de integración** y **33 pruebas E2E** con Chrome headless (login, tema morado, orden SRS→Kanban, propuesta SRS con doble autorización, Kanban con versión asignada, finanzas MXN, pagos, recursos y control de acceso).

## Despliegue

```bash
cp .env.example .env   # configura producción
make docker-build
make docker-up
```

- El contenedor corre como usuario no root.
- `/app/data` es volumen persistente (SQLite + uploads locales).
- En producción usa HTTPS y `ENV=production` (cookies `Secure`).
- Respaldar el volumen `/app/data`.

## Recursos visuales

Las imágenes de la interfaz están en `web/static/img/` y se pueden reemplazar sin tocar código manteniendo el nombre:

- `logo.svg` y `favicon.svg`
- `empty-projects.svg`, `empty-docs.svg`, `empty-deliverables.svg`, `empty-files.svg`

La paleta primaria es morada (`primary-600 = #6d28d9`). Ver `web/static/img/README.md`.

## Seguridad

- Cookies HttpOnly + SameSite=Lax; `Secure` en producción.
- CSRF double-submit en todas las mutaciones.
- Rate limiting por IP en login, IA y tickets.
- Contraseñas bcrypt; tokens de reset/refresh hasheados y revocables.
- Ownership de proyecto verificado en cada operación.
- Secretos solo por entorno; `.env` está en `.gitignore`.

Consulta **[`specifics.md`](./specifics.md)** para el detalle completo.
