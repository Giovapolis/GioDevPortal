package handler

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"giodevportal/internal/config"
	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
	"giodevportal/internal/service/auth"
	"giodevportal/internal/service/email"
	render "giodevportal/internal/template"
	"giodevportal/pkg/gemini"
	"giodevportal/pkg/storage"
	"giodevportal/pkg/whatsapp"
)

const (
	accessCookie  = "gdp_access"
	refreshCookie = "gdp_refresh"
	csrfCookie    = "gdp_csrf"
)

type App struct {
	Cfg      *config.Config
	Q        *repo.Queries
	DB       *sql.DB
	Auth     *auth.Service
	Email    *email.Service
	Gemini   *gemini.Client
	WhatsApp *whatsapp.Client
	Store    storage.Store
	Render   *render.Renderer
}

func NewApp(cfg *config.Config, q *repo.Queries, db *sql.DB, authSvc *auth.Service, emailSvc *email.Service, gem *gemini.Client, wa *whatsapp.Client, store storage.Store, rnd *render.Renderer) *App {
	return &App{Cfg: cfg, Q: q, DB: db, Auth: authSvc, Email: emailSvc, Gemini: gem, WhatsApp: wa, Store: store, Render: rnd}
}

type ctxKey string

const (
	userCtxKey ctxKey = "user"
	csrfCtxKey ctxKey = "csrf"
)

func withUser(ctx context.Context, u *repo.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

func userFrom(ctx context.Context) *repo.User {
	u, _ := ctx.Value(userCtxKey).(*repo.User)
	return u
}

func withCSRF(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfCtxKey, token)
}

func csrfFrom(ctx context.Context) string {
	s, _ := ctx.Value(csrfCtxKey).(string)
	return s
}

func (a *App) baseData(r *http.Request) map[string]any {
	return map[string]any{
		"User":            userFrom(r.Context()),
		"CSRF":            csrfFrom(r.Context()),
		"Msg":             r.URL.Query().Get("msg"),
		"Err":             r.URL.Query().Get("err"),
		"Path":            r.URL.Path,
		"GeminiEnabled":   a.Gemini.Enabled(),
		"WhatsAppEnabled": a.Cfg.WhatsAppEnabled(),
		"R2Enabled":       a.Cfg.R2Enabled(),
		"TaskStatuses":    domain.TaskStatuses,
	}
}

func (a *App) page(w http.ResponseWriter, r *http.Request, status int, page string, extra map[string]any) {
	data := a.baseData(r)
	for k, v := range extra {
		data[k] = v
	}
	if err := a.Render.Render(w, status, page, data); err != nil {
		log.Printf("[render] %s: %v", page, err)
		http.Error(w, "Error al renderizar la página", http.StatusInternalServerError)
	}
}

func (a *App) partial(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	base := a.baseData(r)
	for k, v := range data {
		base[k] = v
	}
	if err := a.Render.Partial(w, http.StatusOK, name, base); err != nil {
		log.Printf("[partial] %s: %v", name, err)
		http.Error(w, "Error al renderizar", http.StatusInternalServerError)
	}
}

func redirectMsg(w http.ResponseWriter, r *http.Request, path, msg string) {
	q := url.Values{}
	q.Set("msg", msg)
	http.Redirect(w, r, path+"?"+q.Encode(), http.StatusSeeOther)
}

func redirectErr(w http.ResponseWriter, r *http.Request, path, msg string) {
	q := url.Values{}
	q.Set("err", msg)
	http.Redirect(w, r, path+"?"+q.Encode(), http.StatusSeeOther)
}

func pathID(r *http.Request, name string) (int64, bool) {
	v := chiURLParam(r, name)
	id, err := strconv.ParseInt(v, 10, 64)
	return id, err == nil && id > 0
}

func formInt(r *http.Request, name string, def int64) int64 {
	v := strings.TrimSpace(r.FormValue(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func formFloat(r *http.Request, name string, def float64) float64 {
	v := strings.TrimSpace(r.FormValue(name))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func formBool(r *http.Request, name string) bool {
	v := strings.ToLower(strings.TrimSpace(r.FormValue(name)))
	return v == "1" || v == "true" || v == "on" || v == "yes"
}

func (a *App) logEvent(ctx context.Context, projectID int64, eventType, message string, actorID int64) {
	actor := sql.NullInt64{Int64: actorID, Valid: actorID > 0}
	if _, err := a.Q.CreateEvent(ctx, repo.CreateEventParams{
		ProjectID: projectID,
		EventType: eventType,
		Message:   message,
		ActorID:   actor,
		CreatedAt: domain.Now(),
	}); err != nil {
		log.Printf("[event] %v", err)
	}
}

func (a *App) projectFor(w http.ResponseWriter, r *http.Request, id int64) (*repo.Project, bool) {
	p, err := a.Q.GetProject(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	u := userFrom(r.Context())
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil, false
	}
	if u.Role != domain.RoleAdmin && p.ClientID != u.ID {
		http.Error(w, "No tienes acceso a este proyecto", http.StatusForbidden)
		return nil, false
	}
	return &p, true
}

func (a *App) ownedProjectOr404(w http.ResponseWriter, r *http.Request, projectID int64) (*repo.Project, bool) {
	p, err := a.Q.GetProject(r.Context(), projectID)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	return &p, true
}

func setCookie(w http.ResponseWriter, name, value string, maxAge int, secure bool, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
