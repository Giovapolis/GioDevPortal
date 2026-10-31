package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (a *App) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeaders)
	r.Use(LimitBody(30 << 20))
	r.Use(a.CSRF)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	r.Get("/", a.Index)
	r.Get("/login", a.LoginPage)
	r.With(a.RateLimit(10, 5)).Post("/login", a.Login)
	r.Get("/forgot-password", a.ForgotPasswordPage)
	r.With(a.RateLimit(5, 3)).Post("/forgot-password", a.ForgotPassword)
	r.Get("/reset-password", a.ResetPasswordPage)
	r.With(a.RateLimit(10, 5)).Post("/reset-password", a.ResetPassword)

	r.Group(func(pr chi.Router) {
		pr.Use(a.Authenticate)

		pr.Post("/logout", a.Logout)
		pr.Get("/change-password", a.ChangePasswordPage)
		pr.Post("/change-password", a.ChangePassword)
		pr.Get("/files/*", a.ServeFile)
		pr.Get("/payment-proofs/{claimID}", a.ServePaymentProof)

		pr.Route("/admin", func(ar chi.Router) {
			ar.Use(a.RequireRole("admin"))
			ar.Get("/", a.AdminDashboard)
			ar.Get("/clients", a.AdminClients)
			ar.Post("/clients", a.AdminCreateClient)
			ar.Post("/clients/{id}/reset-password", a.AdminResetClientPassword)
			ar.Post("/projects", a.AdminCreateProject)
			ar.Get("/projects/{id}", a.AdminProjectEntry)
			ar.Get("/projects/{id}/technical-sheet", a.TechnicalSheetPage)
			ar.Post("/projects/{id}/technical-sheet", a.TechnicalSheetSave)
			ar.Get("/projects/{id}/srs", a.SRSPage)
			ar.With(a.RateLimit(10, 3)).Post("/projects/{id}/srs/generate", a.SRSGenerate)
			ar.Post("/projects/{id}/srs/save", a.SRSSave)
			ar.Post("/projects/{id}/srs/requests/{requestID}/approve", a.AdminApproveSRSRequest)
			ar.Post("/projects/{id}/srs/requests/{requestID}/reject", a.AdminRejectSRSRequest)
			ar.Post("/projects/{id}/srs/proposals/{proposalID}/approve", a.AdminApproveSRSProposal)
			ar.With(a.RateLimit(10, 3)).Post("/projects/{id}/srs/tasks/generate", a.GenerateProposedTasks)
			ar.Post("/projects/{id}/proposed/{taskID}/approve", a.ApproveProposedTask)
			ar.Post("/projects/{id}/proposed/{taskID}/reject", a.RejectProposedTask)
			ar.Get("/projects/{id}/kanban", a.KanbanPage)
			ar.Post("/projects/{id}/kanban/srs", a.KanbanSetSRSVersion)
			ar.With(a.RateLimit(10, 3)).Post("/projects/{id}/kanban/validate", a.KanbanValidateBacklog)
			ar.Post("/projects/{id}/kanban/tasks", a.TaskCreate)
			ar.Post("/projects/{id}/kanban/tasks/{taskID}/move", a.TaskMove)
			ar.Post("/projects/{id}/kanban/tasks/{taskID}/delete", a.TaskDelete)
			ar.Get("/projects/{id}/deliverables", a.AdminDeliverables)
			ar.Post("/projects/{id}/deliverables", a.AdminCreateDeliverable)
			ar.Post("/projects/{id}/deliverables/{deliverableID}/accept", a.AdminAcceptDeliverable)
			ar.Post("/projects/{id}/deliverables/{deliverableID}/delete", a.AdminDeleteDeliverable)
			ar.Get("/projects/{id}/resources", a.AdminResources)
			ar.Post("/projects/{id}/resources/{resourceID}/delete", a.AdminDeleteResource)
			ar.Get("/projects/{id}/finances", a.AdminFinances)
			ar.Post("/projects/{id}/payments/{claimID}/paid", a.AdminMarkPaymentPaid)
			ar.Post("/projects/{id}/payments/{claimID}/pending", a.AdminMarkPaymentPending)
			ar.Get("/projects/{id}/tickets", a.AdminTickets)
			ar.Post("/projects/{id}/tickets/{ticketID}/status", a.AdminUpdateTicketStatus)
		})

		pr.Route("/client", func(cr chi.Router) {
			cr.Use(a.RequireRole("client"))
			cr.Get("/", a.ClientHome)
			cr.Get("/projects/{id}", a.ClientDashboard)
			cr.Get("/projects/{id}/docs", a.ClientDocs)
			cr.With(a.RateLimit(10, 5)).Post("/projects/{id}/requests", a.ClientCreateSRSRequest)
			cr.Post("/projects/{id}/srs/proposals/{proposalID}/approve", a.ClientApproveSRSProposal)
			cr.Get("/projects/{id}/deliverables", a.ClientDeliverables)
			cr.With(a.RateLimit(20, 10)).Post("/projects/{id}/deliverables/{deliverableID}/tickets", a.ClientCreateTicket)
			cr.Get("/projects/{id}/resources", a.ClientResources)
			cr.Post("/projects/{id}/resources", a.ResourceUpload)
			cr.Get("/projects/{id}/finances", a.ClientFinances)
			cr.With(a.RateLimit(10, 5)).Post("/projects/{id}/payments", a.ClientCreatePaymentClaim)
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		a.page(w, r, http.StatusNotFound, "public/error.html", map[string]any{
			"Title":  "Página no encontrada",
			"Status": http.StatusNotFound,
			"Detail": "La página que buscas no existe.",
		})
	})

	return r
}
