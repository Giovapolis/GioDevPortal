package handler

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
	"giodevportal/internal/service/auth"
)

func (a *App) Index(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if u.Role == domain.RoleAdmin {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/client", http.StatusSeeOther)
}

func (a *App) LoginPage(w http.ResponseWriter, r *http.Request) {
	a.page(w, r, http.StatusOK, "auth/login.html", map[string]any{"Title": "Ingresar"})
}

func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	if email == "" || password == "" {
		redirectErr(w, r, "/login", "Ingresa tu correo y contraseña")
		return
	}

	u, err := a.Q.GetUserByEmail(r.Context(), email)
	if err != nil || !a.Auth.CheckPassword(u.PasswordHash, password) {
		redirectErr(w, r, "/login", "Credenciales inválidas")
		return
	}
	if u.IsActive != 1 {
		redirectErr(w, r, "/login", "Tu cuenta está desactivada")
		return
	}

	access, err := a.Auth.IssueAccessToken(u.ID, u.Role)
	if err != nil {
		redirectErr(w, r, "/login", "No se pudo iniciar sesión")
		return
	}

	raw, err := auth.GenerateToken()
	if err != nil {
		redirectErr(w, r, "/login", "No se pudo iniciar sesión")
		return
	}
	if _, err := a.Q.CreateRefreshToken(r.Context(), repo.CreateRefreshTokenParams{
		UserID:    u.ID,
		TokenHash: auth.HashToken(raw),
		ExpiresAt: time.Now().Add(a.Auth.RefreshTTL()).UTC().Format(time.RFC3339),
		CreatedAt: domain.Now(),
	}); err != nil {
		redirectErr(w, r, "/login", "No se pudo iniciar sesión")
		return
	}

	a.setAccessCookie(w, access)
	a.setRefreshCookie(w, raw)

	if u.MustChangePassword == 1 {
		redirectMsg(w, r, "/change-password", "Por seguridad debes cambiar tu contraseña temporal")
		return
	}
	if u.Role == domain.RoleAdmin {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/client", http.StatusSeeOther)
}

func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(refreshCookie); err == nil && c.Value != "" {
		if row, err := a.Q.GetRefreshTokenByHash(r.Context(), auth.HashToken(c.Value)); err == nil {
			_ = a.Q.RevokeRefreshToken(r.Context(), row.ID)
		}
	}
	a.clearAuthCookies(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) ForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	a.page(w, r, http.StatusOK, "auth/forgot-password.html", map[string]any{"Title": "Recuperar contraseña"})
}

func (a *App) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if email != "" {
		if u, err := a.Q.GetUserByEmail(r.Context(), email); err == nil && u.IsActive == 1 {
			raw, err := auth.GenerateToken()
			if err == nil {
				_, err = a.Q.CreatePasswordReset(r.Context(), repo.CreatePasswordResetParams{
					UserID:    u.ID,
					TokenHash: auth.HashToken(raw),
					ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
					CreatedAt: domain.Now(),
				})
				if err == nil {
					link := a.Cfg.AppBaseURL + "/reset-password?token=" + raw
					_ = a.Email.SendPasswordReset(r.Context(), u.Email, u.Name, link)
				}
			}
		}
	}
	redirectMsg(w, r, "/login", "Si el correo existe, enviamos las instrucciones para restablecer la contraseña")
}

func (a *App) ResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	a.page(w, r, http.StatusOK, "auth/reset-password.html", map[string]any{
		"Title": "Restablecer contraseña",
		"Token": token,
	})
}

func (a *App) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.FormValue("token"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")
	if token == "" || len(password) < 8 || password != confirm {
		redirectErr(w, r, "/reset-password?token="+token, "La contraseña debe tener al menos 8 caracteres y coincidir")
		return
	}

	row, err := a.Q.GetPasswordResetByHash(r.Context(), auth.HashToken(token))
	if err != nil || row.Used == 1 {
		redirectErr(w, r, "/reset-password?token="+token, "El enlace es inválido o ya fue utilizado")
		return
	}
	exp, ok := domain.ParseTime(row.ExpiresAt)
	if !ok || time.Now().After(exp) {
		redirectErr(w, r, "/reset-password?token="+token, "El enlace expiró")
		return
	}

	hash, err := a.Auth.HashPassword(password)
	if err != nil {
		redirectErr(w, r, "/reset-password?token="+token, "No se pudo actualizar la contraseña")
		return
	}
	if err := a.Q.UpdateUserPassword(r.Context(), repo.UpdateUserPasswordParams{
		PasswordHash:       hash,
		MustChangePassword: 0,
		UpdatedAt:          domain.Now(),
		ID:                 row.UserID,
	}); err != nil {
		redirectErr(w, r, "/reset-password?token="+token, "No se pudo actualizar la contraseña")
		return
	}
	_ = a.Q.UsePasswordReset(r.Context(), row.ID)
	_ = a.Q.RevokeAllUserRefreshTokens(r.Context(), row.UserID)

	redirectMsg(w, r, "/login", "Contraseña actualizada. Ya puedes ingresar.")
}

func (a *App) ChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	a.page(w, r, http.StatusOK, "auth/change-password.html", map[string]any{"Title": "Cambiar contraseña"})
}

func (a *App) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	current := r.FormValue("current_password")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	if !a.Auth.CheckPassword(u.PasswordHash, current) {
		redirectErr(w, r, "/change-password", "La contraseña actual no es correcta")
		return
	}
	if len(password) < 8 || password != confirm {
		redirectErr(w, r, "/change-password", "La nueva contraseña debe tener al menos 8 caracteres y coincidir")
		return
	}

	hash, err := a.Auth.HashPassword(password)
	if err != nil {
		redirectErr(w, r, "/change-password", "No se pudo actualizar la contraseña")
		return
	}
	if err := a.Q.UpdateUserPassword(r.Context(), repo.UpdateUserPasswordParams{
		PasswordHash:       hash,
		MustChangePassword: 0,
		UpdatedAt:          domain.Now(),
		ID:                 u.ID,
	}); err != nil {
		redirectErr(w, r, "/change-password", "No se pudo actualizar la contraseña")
		return
	}
	_ = a.Q.RevokeAllUserRefreshTokens(r.Context(), u.ID)

	if u.Role == domain.RoleAdmin {
		redirectMsg(w, r, "/admin", "Contraseña actualizada")
		return
	}
	redirectMsg(w, r, "/client", "Contraseña actualizada")
}

func nullID(id int64) sql.NullInt64 {
	return sql.NullInt64{Int64: id, Valid: id > 0}
}
