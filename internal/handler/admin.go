package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
	"giodevportal/internal/service/auth"
)

func (a *App) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, _ := a.Q.ListProjects(ctx)
	clients, _ := a.Q.ListClients(ctx)
	totalProjects, _ := a.Q.CountProjects(ctx)
	activeProjects, _ := a.Q.CountActiveProjects(ctx)

	a.page(w, r, http.StatusOK, "admin/dashboard.html", map[string]any{
		"Title":          "Panel Global",
		"Projects":       projects,
		"Clients":        clients,
		"TotalProjects":  totalProjects,
		"ActiveProjects": activeProjects,
		"TotalClients":   len(clients),
	})
}

func (a *App) AdminClients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clients, _ := a.Q.ListClients(ctx)
	projects, _ := a.Q.ListProjects(ctx)

	a.page(w, r, http.StatusOK, "admin/clients.html", map[string]any{
		"Title":    "Clientes",
		"Clients":  clients,
		"Projects": projects,
	})
}

func (a *App) AdminCreateClient(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if name == "" || email == "" {
		redirectErr(w, r, "/admin/clients", "Nombre y correo son obligatorios")
		return
	}
	if _, err := a.Q.GetUserByEmail(r.Context(), email); err == nil {
		redirectErr(w, r, "/admin/clients", "Ya existe un usuario con ese correo")
		return
	}

	tempPass, err := auth.GeneratePassword(12)
	if err != nil {
		redirectErr(w, r, "/admin/clients", "No se pudo generar la contraseña")
		return
	}
	hash, err := a.Auth.HashPassword(tempPass)
	if err != nil {
		redirectErr(w, r, "/admin/clients", "No se pudo crear el cliente")
		return
	}

	now := domain.Now()
	if _, err := a.Q.CreateUser(r.Context(), repo.CreateUserParams{
		Name:               name,
		Email:              email,
		PasswordHash:       hash,
		Role:               domain.RoleClient,
		IsActive:           1,
		MustChangePassword: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		redirectErr(w, r, "/admin/clients", "No se pudo crear el cliente")
		return
	}

	_ = a.Email.SendWelcome(r.Context(), email, name, tempPass, a.Cfg.AppBaseURL)
	redirectMsg(w, r, "/admin/clients", "Cliente creado. Contraseña temporal: "+tempPass)
}

func (a *App) AdminResetClientPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	u, err := a.Q.GetUserByID(r.Context(), id)
	if err != nil || u.Role != domain.RoleClient {
		http.NotFound(w, r)
		return
	}

	tempPass, err := auth.GeneratePassword(12)
	if err != nil {
		redirectErr(w, r, "/admin/clients", "No se pudo generar la contraseña")
		return
	}
	hash, err := a.Auth.HashPassword(tempPass)
	if err != nil {
		redirectErr(w, r, "/admin/clients", "No se pudo restablecer la contraseña")
		return
	}
	if err := a.Q.UpdateUserPassword(r.Context(), repo.UpdateUserPasswordParams{
		PasswordHash:       hash,
		MustChangePassword: 1,
		UpdatedAt:          domain.Now(),
		ID:                 u.ID,
	}); err != nil {
		redirectErr(w, r, "/admin/clients", "No se pudo restablecer la contraseña")
		return
	}
	_ = a.Q.RevokeAllUserRefreshTokens(r.Context(), u.ID)
	_ = a.Email.SendWelcome(r.Context(), u.Email, u.Name, tempPass, a.Cfg.AppBaseURL)

	redirectMsg(w, r, "/admin/clients", "Contraseña restablecida para "+u.Email+". Temporal: "+tempPass)
}

func (a *App) AdminCreateProject(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	clientID := formInt(r, "client_id", 0)
	baseCost := formFloat(r, "base_cost", 0)
	currency := "MXN"
	startDate := strings.TrimSpace(r.FormValue("start_date"))

	if name == "" || clientID == 0 {
		redirectErr(w, r, "/admin", "Nombre del proyecto y cliente son obligatorios")
		return
	}
	client, err := a.Q.GetUserByID(r.Context(), clientID)
	if err != nil || client.Role != domain.RoleClient {
		redirectErr(w, r, "/admin", "El cliente seleccionado no es válido")
		return
	}

	now := domain.Now()
	start := sql.NullString{}
	if startDate != "" {
		start = sql.NullString{String: startDate + "T00:00:00Z", Valid: true}
	}

	p, err := a.Q.CreateProject(r.Context(), repo.CreateProjectParams{
		Name:         name,
		Description:  description,
		ClientID:     clientID,
		Status:       domain.StatusActive,
		StartDate:    start,
		BaseCost:     baseCost,
		Currency:     currency,
		GitRepo:      "",
		PrivateNotes: "",
		ApiKeys:      "",
		ServerAccess: "",
		StagingUrl:   "",
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		redirectErr(w, r, "/admin", "No se pudo crear el proyecto")
		return
	}

	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "project_created", "Proyecto creado: "+p.Name, u.ID)
	redirectMsg(w, r, "/admin/projects/"+strconv.FormatInt(p.ID, 10)+"/srs", "Proyecto creado correctamente")
}

func (a *App) AdminProjectEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, ok := a.projectFor(w, r, id); !ok {
		return
	}
	http.Redirect(w, r, "/admin/projects/"+strconv.FormatInt(id, 10)+"/srs", http.StatusSeeOther)
}
