package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
)

func (a *App) ClientCreateSRSRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	back := fmt.Sprintf("/client/projects/%d/docs", p.ID)

	content := strings.TrimSpace(r.FormValue("content"))
	if len([]rune(content)) < 10 {
		redirectErr(w, r, back, "Describe la actualización con al menos 10 caracteres")
		return
	}
	if len([]rune(content)) > 4000 {
		redirectErr(w, r, back, "La solicitud supera el máximo de 4000 caracteres")
		return
	}

	u := userFrom(r.Context())
	if _, err := a.Q.CreateSRSRequest(r.Context(), repo.CreateSRSRequestParams{
		ProjectID: p.ID,
		ClientID:  u.ID,
		Content:   content,
		CreatedAt: domain.Now(),
	}); err != nil {
		redirectErr(w, r, back, "No se pudo registrar la solicitud")
		return
	}

	a.logEvent(r.Context(), p.ID, "srs_request_created", "Solicitud de actualización del SRS enviada por el cliente", u.ID)
	go func(project, client, summary string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = a.WhatsApp.SendSRSRequestAlert(ctx, project, client, summary)
	}(p.Name, u.Name, content)

	redirectMsg(w, r, back, "Solicitud enviada. El administrador debe autorizarla para actualizar la versión final del SRS.")
}

func (a *App) AdminApproveSRSRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	requestID, ok := pathID(r, "requestID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	req, err := a.Q.GetSRSRequest(r.Context(), requestID)
	if err != nil || req.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	if req.Status != "pending" {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "Esta solicitud ya fue revisada")
		return
	}

	latest, err := a.Q.GetLatestSRS(r.Context(), p.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			redirectErr(w, r, a.workspacePath(p.ID, "srs"), "Genera primero un SRS antes de aprobar solicitudes")
			return
		}
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo cargar el SRS actual")
		return
	}

	next, err := a.Q.NextSRSVersion(r.Context(), p.ID)
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo calcular la versión del SRS")
		return
	}

	section := fmt.Sprintf("\n\n---\n\n## Actualización aprobada · %s\n\n%s", time.Now().Format("02 Jan 2006"), req.Content)
	clientContent := strings.TrimRight(latest.ClientContent, "\n") + section

	u := userFrom(r.Context())
	version, err := a.Q.CreateSRSVersion(r.Context(), repo.CreateSRSVersionParams{
		ProjectID:     p.ID,
		Version:       next,
		ClientContent: clientContent,
		DevContent:    latest.DevContent,
		SourceNotes:   fmt.Sprintf("Actualización solicitada por el cliente (solicitud #%d)", req.ID),
		CreatedBy:     u.ID,
		CreatedAt:     domain.Now(),
	})
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo crear la nueva versión del SRS")
		return
	}

	_ = a.Q.SetSRSRequestApproved(r.Context(), repo.SetSRSRequestApprovedParams{
		AdminNote:    strings.TrimSpace(r.FormValue("admin_note")),
		SrsVersionID: sql.NullInt64{Int64: version.ID, Valid: true},
		ReviewedBy:   sql.NullInt64{Int64: u.ID, Valid: true},
		ReviewedAt:   sql.NullString{String: domain.Now(), Valid: true},
		ID:           req.ID,
	})
	a.logEvent(r.Context(), p.ID, "srs_request_approved", fmt.Sprintf("Solicitud de actualización aprobada · SRS v%d", next), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "srs"), fmt.Sprintf("Solicitud aprobada. SRS actualizado a la versión %d", next))
}

func (a *App) AdminRejectSRSRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	requestID, ok := pathID(r, "requestID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	req, err := a.Q.GetSRSRequest(r.Context(), requestID)
	if err != nil || req.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	if req.Status != "pending" {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "Esta solicitud ya fue revisada")
		return
	}
	u := userFrom(r.Context())
	_ = a.Q.SetSRSRequestRejected(r.Context(), repo.SetSRSRequestRejectedParams{
		AdminNote:  strings.TrimSpace(r.FormValue("admin_note")),
		ReviewedBy: sql.NullInt64{Int64: u.ID, Valid: true},
		ReviewedAt: sql.NullString{String: domain.Now(), Valid: true},
		ID:         req.ID,
	})
	a.logEvent(r.Context(), p.ID, "srs_request_rejected", "Solicitud de actualización rechazada", u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "srs"), "Solicitud rechazada")
}
