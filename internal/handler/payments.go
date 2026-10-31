package handler

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
)

const maxProofSize = 25 << 20

func (a *App) ClientCreatePaymentClaim(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	back := "/client/projects/" + fmt.Sprintf("%d", p.ID) + "/finances"

	amount := formFloat(r, "amount", 0)
	if amount <= 0 {
		redirectErr(w, r, back, "El monto del pago debe ser mayor a cero")
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))

	paidBase, _ := a.Q.SumPaidBase(r.Context(), p.ID)
	outstanding := p.BaseCost - asFloat(paidBase)
	if outstanding < 0 {
		outstanding = 0
	}
	if amount > outstanding+0.005 {
		redirectErr(w, r, back, fmt.Sprintf("El monto supera el saldo pendiente del costo base (%.2f %s)", outstanding, p.Currency))
		return
	}

	file, header, err := r.FormFile("proof")
	if err != nil {
		redirectErr(w, r, back, "Adjunta el comprobante del pago")
		return
	}
	defer file.Close()
	if header.Size <= 0 {
		redirectErr(w, r, back, "El comprobante está vacío")
		return
	}
	if header.Size > maxProofSize {
		redirectErr(w, r, back, "El comprobante supera el límite de 25 MB")
		return
	}

	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(filepath.Base(header.Filename), ext)
	key := fmt.Sprintf("payments/%d/%d-%s%s", p.ID, time.Now().UnixNano(), domain.Slugify(base), ext)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			contentType = ct
		} else {
			contentType = "application/octet-stream"
		}
	}
	if err := a.Store.Put(r.Context(), key, file, header.Size, contentType); err != nil {
		redirectErr(w, r, back, "No se pudo subir el comprobante: "+err.Error())
		return
	}

	u := userFrom(r.Context())
	if _, err := a.Q.CreatePaymentClaim(r.Context(), repo.CreatePaymentClaimParams{
		ProjectID:        p.ID,
		ClientID:         u.ID,
		Amount:           amount,
		Note:             note,
		ProofKey:         key,
		ProofName:        header.Filename,
		ProofContentType: contentType,
		ProofSize:        header.Size,
		CreatedAt:        domain.Now(),
	}); err != nil {
		_ = a.Store.Delete(r.Context(), key)
		redirectErr(w, r, back, "No se pudo registrar el pago")
		return
	}

	a.logEvent(r.Context(), p.ID, "payment_claimed", fmt.Sprintf("Pago reportado por %s: %.2f %s", u.Name, amount, p.Currency), u.ID)
	amountLabel := fmt.Sprintf("%.2f %s", amount, p.Currency)
	go func(project, client, amountLabel string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = a.WhatsApp.SendPaymentAlert(ctx, project, client, amountLabel)
	}(p.Name, u.Name, amountLabel)

	redirectMsg(w, r, back, "Pago reportado. Queda pendiente de autorización del administrador.")
}

func (a *App) AdminMarkPaymentPaid(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	claimID, ok := pathID(r, "claimID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	claim, err := a.Q.GetPaymentClaim(r.Context(), claimID)
	if err != nil || claim.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	if claim.Status == "paid" {
		redirectMsg(w, r, a.workspacePath(p.ID, "finances"), "Este pago ya está marcado como pagado")
		return
	}

	paidBase, _ := a.Q.SumPaidBase(r.Context(), p.ID)
	outstanding := p.BaseCost - asFloat(paidBase)
	if outstanding < 0 {
		outstanding = 0
	}
	if claim.Amount > outstanding+0.005 {
		redirectErr(w, r, a.workspacePath(p.ID, "finances"), fmt.Sprintf("El pago supera el saldo pendiente actual (%.2f %s)", outstanding, p.Currency))
		return
	}

	now := domain.Now()
	reason := "Pago reportado por el cliente"
	if claim.Note != "" {
		reason += ": " + claim.Note
	}
	billing, err := a.Q.CreateBilling(r.Context(), repo.CreateBillingParams{
		ProjectID:  p.ID,
		Amount:     claim.Amount,
		IsExtra:    0,
		Reason:     reason,
		PaidStatus: "paid",
		PaidAt:     sql.NullString{String: now, Valid: true},
		CreatedAt:  now,
	})
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "finances"), "No se pudo registrar el pago")
		return
	}

	u := userFrom(r.Context())
	_ = a.Q.SetPaymentClaimPaid(r.Context(), repo.SetPaymentClaimPaidParams{
		AdminNote:  strings.TrimSpace(r.FormValue("admin_note")),
		BillingID:  sql.NullInt64{Int64: billing.ID, Valid: true},
		ReviewedBy: sql.NullInt64{Int64: u.ID, Valid: true},
		ReviewedAt: sql.NullString{String: now, Valid: true},
		ID:         claim.ID,
	})
	a.logEvent(r.Context(), p.ID, "payment_paid", fmt.Sprintf("Pago marcado como pagado: %.2f %s", claim.Amount, p.Currency), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "finances"), "Pago marcado como pagado: se agregó al Pagado y se descontó del saldo del costo base")
}

func (a *App) AdminMarkPaymentPending(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	claimID, ok := pathID(r, "claimID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	claim, err := a.Q.GetPaymentClaim(r.Context(), claimID)
	if err != nil || claim.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	if claim.Status == "pending" {
		redirectMsg(w, r, a.workspacePath(p.ID, "finances"), "Este pago ya está pendiente")
		return
	}

	u := userFrom(r.Context())
	if claim.BillingID.Valid {
		_ = a.Q.DeleteBilling(r.Context(), claim.BillingID.Int64)
	}
	_ = a.Q.SetPaymentClaimPending(r.Context(), repo.SetPaymentClaimPendingParams{
		AdminNote:  strings.TrimSpace(r.FormValue("admin_note")),
		ReviewedBy: sql.NullInt64{Int64: u.ID, Valid: true},
		ReviewedAt: sql.NullString{String: domain.Now(), Valid: true},
		ID:         claim.ID,
	})
	a.logEvent(r.Context(), p.ID, "payment_pending", fmt.Sprintf("Pago devuelto a pendiente: %.2f %s", claim.Amount, p.Currency), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "finances"), "Pago marcado como pendiente y retirado del Pagado")
}

func (a *App) ServePaymentProof(w http.ResponseWriter, r *http.Request) {
	claimID, ok := pathID(r, "claimID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	claim, err := a.Q.GetPaymentClaim(r.Context(), claimID)
	if err != nil || claim.ProofKey == "" {
		http.NotFound(w, r)
		return
	}
	if _, ok := a.projectFor(w, r, claim.ProjectID); !ok {
		return
	}

	url, err := a.Store.URL(r.Context(), claim.ProofKey)
	if err != nil {
		http.Error(w, "No se pudo generar el enlace del comprobante", http.StatusInternalServerError)
		return
	}
	if strings.HasPrefix(url, "http") {
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return
	}
	rc, err := a.Store.Open(r.Context(), claim.ProofKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()

	if claim.ProofContentType != "" {
		w.Header().Set("Content-Type", claim.ProofContentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", claim.ProofName))
	_, _ = io.Copy(w, rc)
}
