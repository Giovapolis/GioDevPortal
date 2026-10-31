package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
)

func (a *App) AdminApproveSRSProposal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	proposalID, ok := pathID(r, "proposalID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	prop, err := a.Q.GetSRSProposal(r.Context(), proposalID)
	if err != nil || prop.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	if prop.Status != "pending" {
		redirectMsg(w, r, a.workspacePath(p.ID, "srs"), "Esta propuesta ya fue procesada")
		return
	}

	u := userFrom(r.Context())
	if prop.AdminApproved == 0 {
		_ = a.Q.SetSRSProposalAdminApproved(r.Context(), repo.SetSRSProposalAdminApprovedParams{
			AdminApprovedAt: sql.NullString{String: domain.Now(), Valid: true},
			ID:              prop.ID,
		})
		prop.AdminApproved = 1
	}

	if prop.ClientApproved == 0 {
		a.logEvent(r.Context(), p.ID, "srs_proposal_admin_approved", fmt.Sprintf("Propuesta de SRS #%d autorizada por el administrador; falta el cliente", prop.ID), u.ID)
		redirectMsg(w, r, a.workspacePath(p.ID, "srs"), "Autorizada por el administrador. Falta la autorización del cliente para publicar la nueva versión.")
		return
	}

	version, err := a.applySRSProposal(r.Context(), p, &prop)
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo publicar la nueva versión del SRS")
		return
	}
	redirectMsg(w, r, a.workspacePath(p.ID, "srs"), fmt.Sprintf("Ambas partes autorizaron. SRS v%d publicado.", version.Version))
}

func (a *App) ClientApproveSRSProposal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	proposalID, ok := pathID(r, "proposalID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	prop, err := a.Q.GetSRSProposal(r.Context(), proposalID)
	if err != nil || prop.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	if prop.Status != "pending" {
		redirectMsg(w, r, fmt.Sprintf("/client/projects/%d/docs", p.ID), "Esta propuesta ya fue procesada")
		return
	}

	u := userFrom(r.Context())
	if prop.ClientApproved == 0 {
		_ = a.Q.SetSRSProposalClientApproved(r.Context(), repo.SetSRSProposalClientApprovedParams{
			ClientApprovedAt: sql.NullString{String: domain.Now(), Valid: true},
			ID:               prop.ID,
		})
		prop.ClientApproved = 1
	}

	if prop.AdminApproved == 0 {
		a.logEvent(r.Context(), p.ID, "srs_proposal_client_approved", fmt.Sprintf("Propuesta de SRS #%d autorizada por el cliente; falta el administrador", prop.ID), u.ID)
		redirectMsg(w, r, fmt.Sprintf("/client/projects/%d/docs", p.ID), "Autorizada. Falta la autorización del administrador para publicar la nueva versión.")
		return
	}

	version, err := a.applySRSProposal(r.Context(), p, &prop)
	if err != nil {
		redirectErr(w, r, fmt.Sprintf("/client/projects/%d/docs", p.ID), "No se pudo publicar la nueva versión del SRS")
		return
	}
	redirectMsg(w, r, fmt.Sprintf("/client/projects/%d/docs", p.ID), fmt.Sprintf("Ambas partes autorizaron. SRS v%d publicado.", version.Version))
}

func (a *App) applySRSProposal(ctx context.Context, p *repo.Project, prop *repo.SrsProposal) (*repo.SrsVersion, error) {
	if prop.Status == "applied" {
		return nil, fmt.Errorf("proposal already applied")
	}
	next, err := a.Q.NextSRSVersion(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	version, err := a.Q.CreateSRSVersion(ctx, repo.CreateSRSVersionParams{
		ProjectID:     p.ID,
		Version:       next,
		ClientContent: prop.ClientContent,
		DevContent:    prop.DevContent,
		SourceNotes:   prop.SourceNotes,
		CreatedBy:     prop.CreatedBy,
		CreatedAt:     domain.Now(),
	})
	if err != nil {
		return nil, err
	}
	if err := a.Q.MarkSRSProposalApplied(ctx, repo.MarkSRSProposalAppliedParams{
		AppliedVersionID: sql.NullInt64{Int64: version.ID, Valid: true},
		ID:               prop.ID,
	}); err != nil {
		return nil, err
	}
	a.logEvent(ctx, p.ID, "srs_published", fmt.Sprintf("SRS v%d publicado tras autorización de administrador y cliente", next), prop.CreatedBy)
	return &version, nil
}
