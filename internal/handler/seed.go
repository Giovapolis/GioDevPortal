package handler

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
)

// Seed creates the initial admin (and demo data in dev) when the database is empty.
func (a *App) Seed(ctx context.Context) error {
	count, err := a.Q.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := a.Auth.HashPassword(a.Cfg.SeedAdminPassword)
	if err != nil {
		return err
	}
	now := domain.Now()
	admin, err := a.Q.CreateUser(ctx, repo.CreateUserParams{
		Name:               a.Cfg.SeedAdminName,
		Email:              a.Cfg.SeedAdminEmail,
		PasswordHash:       hash,
		Role:               domain.RoleAdmin,
		IsActive:           1,
		MustChangePassword: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		return err
	}
	log.Printf("[seed] admin created: %s", admin.Email)

	if !a.Cfg.SeedDemo {
		return nil
	}

	demoHash, err := a.Auth.HashPassword("Cliente123!")
	if err != nil {
		return err
	}
	client, err := a.Q.CreateUser(ctx, repo.CreateUserParams{
		Name:               "Cliente Demo",
		Email:              "cliente@demo.local",
		PasswordHash:       demoHash,
		Role:               domain.RoleClient,
		IsActive:           1,
		MustChangePassword: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		return err
	}

	project, err := a.Q.CreateProject(ctx, repo.CreateProjectParams{
		Name:         "Portal de Reservas",
		Description:  "Sistema de reservas para una cadena de restaurantes con panel administrativo.",
		ClientID:     client.ID,
		Status:       domain.StatusActive,
		StartDate:    sql.NullString{String: now, Valid: true},
		BaseCost:     4800,
		Currency:     "MXN",
		GitRepo:      "git@github.com:giodev/portal-reservas.git",
		PrivateNotes: "Servidor staging: 10.0.0.12 (ver ficha técnica privada).",
		ApiKeys:      "GEMINI_KEY=demo; STRIPE_KEY=sk_test_demo",
		ServerAccess: "ssh deploy@10.0.0.12",
		StagingUrl:   "https://staging.demo.local",
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return err
	}

	tasks := []struct {
		title, desc, status string
	}{
		{"Configurar autenticación", "Login con roles admin/cliente y cookies seguras.", domain.TaskDone},
		{"Modelo de reservas", "Tablas, migraciones y validaciones del dominio.", domain.TaskDone},
		{"Pantalla de disponibilidad", "Calendario con horarios disponibles.", domain.TaskTesting},
		{"Integración de pagos", "Stripe checkout y webhooks.", domain.TaskDoing},
		{"Reportes de ocupación", "Dashboard con métricas por sucursal.", domain.TaskBacklog},
	}
	for i, t := range tasks {
		_, _ = a.Q.CreateTask(ctx, repo.CreateTaskParams{
			ProjectID:   project.ID,
			Title:       t.title,
			Description: t.desc,
			Status:      t.status,
			ColumnOrder: int64(i),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	srs, err := a.Q.CreateSRSVersion(ctx, repo.CreateSRSVersionParams{
		ProjectID:     project.ID,
		Version:       1,
		ClientContent: "## Objetivo\nPermitir que los clientes reserven mesa en línea y que el restaurante gestione la ocupación.\n\n## Reglas de negocio\n- Cada reserva requiere nombre, teléfono y número de personas.\n- No se permiten reservas fuera del horario de atención.\n- El administrador puede cancelar reservas.\n\n## Casos de uso\n1. El cliente consulta disponibilidad.\n2. El cliente confirma una reserva.\n3. El restaurante revisa la agenda del día.",
		DevContent:    "## Arquitectura\nGo + Chi + SQLite + HTMX + Tailwind, despliegue Docker.\n\n## Modelo de datos\nreservations, tables, customers, users.\n\n## Endpoints\n- GET /availability\n- POST /reservations\n- GET /admin/reservations\n\n## Criterios de aceptación\n- Reservas sin solapamiento.\n- Validación de horarios.\n- Panel administrativo funcional.",
		SourceNotes:   "El cliente quiere reservas online para 3 sucursales, con panel de administración y reportes básicos.",
		CreatedBy:     admin.ID,
		CreatedAt:     now,
	})
	if err != nil {
		return err
	}

	_ = a.Q.SetProjectKanbanSRSVersion(ctx, repo.SetProjectKanbanSRSVersionParams{
		KanbanSrsVersionID: sql.NullInt64{Int64: srs.ID, Valid: true},
		UpdatedAt:          now,
		ID:                 project.ID,
	})

	_, _ = a.Q.CreateProposedTask(ctx, repo.CreateProposedTaskParams{
		ProjectID:     project.ID,
		SrsVersionID:  sql.NullInt64{Int64: srs.ID, Valid: true},
		AiTitle:       "Validaciones de disponibilidad",
		AiDescription: "Evitar solapamiento de reservas por mesa y franja horaria.",
		CreatedAt:     now,
	})

	_, err = a.Q.CreateDeliverable(ctx, repo.CreateDeliverableParams{
		ProjectID: project.ID,
		SprintNum: 2,
		Title:     "Reservas y disponibilidad",
		UrlDemo:   "https://staging.demo.local/reservas",
		Notes:     "Incluye calendario y flujo de reserva completo. Pendiente aprobación del cliente.",
		CreatedAt: now,
	})
	if err != nil {
		return err
	}

	sprint1, err := a.Q.CreateDeliverable(ctx, repo.CreateDeliverableParams{
		ProjectID: project.ID,
		SprintNum: 1,
		Title:     "Autenticación y base",
		UrlDemo:   "https://staging.demo.local",
		Notes:     "Login, roles y estructura base.",
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	_ = a.Q.AcceptDeliverable(ctx, repo.AcceptDeliverableParams{
		AcceptedAt: sql.NullString{String: now, Valid: true},
		ID:         sprint1.ID,
	})

	proofPayload := []byte("Comprobante de pago inicial (demo)\n")
	proofKey := fmt.Sprintf("payments/%d/seed-comprobante-inicial.txt", project.ID)
	if err := a.Store.Put(ctx, proofKey, bytes.NewReader(proofPayload), int64(len(proofPayload)), "text/plain"); err != nil {
		return err
	}
	billing, err := a.Q.CreateBilling(ctx, repo.CreateBillingParams{
		ProjectID:  project.ID,
		Amount:     2400,
		IsExtra:    0,
		Reason:     "Pago inicial (50%)",
		PaidStatus: "paid",
		PaidAt:     sql.NullString{String: now, Valid: true},
		CreatedAt:  now,
	})
	if err != nil {
		return err
	}
	initialClaim, err := a.Q.CreatePaymentClaim(ctx, repo.CreatePaymentClaimParams{
		ProjectID:        project.ID,
		ClientID:         client.ID,
		Amount:           2400,
		Note:             "Pago inicial (50%)",
		ProofKey:         proofKey,
		ProofName:        "comprobante-inicial.txt",
		ProofContentType: "text/plain",
		ProofSize:        int64(len(proofPayload)),
		CreatedAt:        now,
	})
	if err != nil {
		return err
	}
	_ = a.Q.SetPaymentClaimPaid(ctx, repo.SetPaymentClaimPaidParams{
		AdminNote:  "Pago inicial verificado",
		BillingID:  sql.NullInt64{Int64: billing.ID, Valid: true},
		ReviewedBy: sql.NullInt64{Int64: admin.ID, Valid: true},
		ReviewedAt: sql.NullString{String: now, Valid: true},
		ID:         initialClaim.ID,
	})

	events := []struct{ typ, msg string }{
		{"project_created", "Proyecto creado: " + project.Name},
		{"srs_generated", "SRS v1 generado"},
		{"deliverable_created", "Entregable del Sprint 2 publicado"},
		{"task_moved", "Integración de pagos → En Curso"},
	}
	for _, e := range events {
		a.logEvent(ctx, project.ID, e.typ, e.msg, admin.ID)
	}

	log.Printf("[seed] demo client: cliente@demo.local / Cliente123!")
	return nil
}
