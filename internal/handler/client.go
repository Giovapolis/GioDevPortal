package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
)

func (a *App) ClientHome(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	projects, _ := a.Q.ListProjectsByClient(r.Context(), u.ID)
	if len(projects) == 1 {
		http.Redirect(w, r, "/client/projects/"+strconv.FormatInt(projects[0].ID, 10), http.StatusSeeOther)
		return
	}
	a.page(w, r, http.StatusOK, "client/home.html", map[string]any{
		"Title":    "Mis proyectos",
		"Projects": projects,
	})
}

func (a *App) ClientDashboard(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	events, _ := a.Q.ListEvents(r.Context(), repo.ListEventsParams{ProjectID: p.ID, Limit: 20})
	accepted, _ := a.Q.ListAcceptedDeliverables(r.Context(), p.ID)
	current, err := a.Q.GetCurrentDeliverable(r.Context(), p.ID)
	hasCurrent := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		hasCurrent = false
	}
	sums, _ := a.Q.SumBillings(r.Context(), p.ID)
	openTickets, _ := a.Q.CountOpenTickets(r.Context(), p.ID)

	var currentPtr *repo.Deliverable
	if hasCurrent {
		currentPtr = &current
	}

	a.page(w, r, http.StatusOK, "client/dashboard.html", map[string]any{
		"Title":         "Resumen · " + p.Name,
		"Project":       p,
		"Events":        events,
		"Accepted":      accepted,
		"Current":       currentPtr,
		"Totals":        sumsMap(sums),
		"OpenTickets":   openTickets,
		"ActiveSection": "resumen",
	})
}

func (a *App) ClientDocs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	latest, err := a.Q.GetLatestSRS(r.Context(), p.ID)
	hasSRS := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		hasSRS = false
	}
	requests, _ := a.Q.ListSRSRequestsByProject(r.Context(), p.ID)
	pendingProposal, perr := a.Q.GetPendingSRSProposal(r.Context(), p.ID)
	var latestPtr *repo.SrsVersion
	if hasSRS {
		latestPtr = &latest
	}
	var proposalPtr *repo.SrsProposal
	if perr == nil {
		proposalPtr = &pendingProposal
	}
	a.page(w, r, http.StatusOK, "client/docs.html", map[string]any{
		"Title":           "Documentación · " + p.Name,
		"Project":         p,
		"Latest":          latestPtr,
		"HasSRS":          hasSRS,
		"Requests":        requests,
		"PendingProposal": proposalPtr,
		"ActiveSection":   "docs",
	})
}

func (a *App) ClientDeliverables(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	current, err := a.Q.GetCurrentDeliverable(r.Context(), p.ID)
	hasCurrent := err == nil && current.ID > 0
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		hasCurrent = false
	}
	accepted, _ := a.Q.ListAcceptedDeliverables(r.Context(), p.ID)
	tickets, _ := a.Q.ListTicketsByProject(r.Context(), p.ID)

	var currentPtr *repo.Deliverable
	if hasCurrent {
		currentPtr = &current
	}

	a.page(w, r, http.StatusOK, "client/deliverables.html", map[string]any{
		"Title":         "Entregables · " + p.Name,
		"Project":       p,
		"Current":       currentPtr,
		"Accepted":      accepted,
		"Tickets":       tickets,
		"ActiveSection": "entregables",
	})
}

func (a *App) ClientCreateTicket(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		a.partial(w, r, "ticket_sent", map[string]any{"TicketError": "Escribe un mensaje antes de enviar"})
		return
	}
	if len([]rune(message)) > 2000 {
		a.partial(w, r, "ticket_sent", map[string]any{"TicketError": "El mensaje es demasiado largo (máximo 2000 caracteres)"})
		return
	}

	var deliverableID sql.NullInt64
	if did := formInt(r, "deliverable_id", 0); did > 0 {
		if d, err := a.Q.GetDeliverable(r.Context(), did); err == nil && d.ProjectID == p.ID {
			deliverableID = sql.NullInt64{Int64: d.ID, Valid: true}
		}
	}

	u := userFrom(r.Context())
	ticket, err := a.Q.CreateTicket(r.Context(), repo.CreateTicketParams{
		ProjectID:     p.ID,
		DeliverableID: deliverableID,
		ClientID:      u.ID,
		ClientMessage: message,
		CreatedAt:     domain.Now(),
	})
	if err != nil {
		a.partial(w, r, "ticket_sent", map[string]any{"TicketError": "No se pudo registrar la nota"})
		return
	}

	a.logEvent(r.Context(), p.ID, "ticket_created", "Nueva nota del cliente: "+domain.Truncate(message, 80), u.ID)

	go func(project, client, msg string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = a.WhatsApp.SendTicketAlert(ctx, project, client, msg)
	}(p.Name, u.Name, message)

	_ = ticket
	a.partial(w, r, "ticket_sent", map[string]any{"TicketOK": "Nota enviada. El administrador fue notificado."})
}

func (a *App) ClientResources(w http.ResponseWriter, r *http.Request) {
	a.resourcesPage(w, r, "client/resources.html")
}
