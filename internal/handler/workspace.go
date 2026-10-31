package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
)

// ---------- Technical sheet ----------

func (a *App) TechnicalSheetPage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	a.page(w, r, http.StatusOK, "admin/workspace/technical-sheet.html", map[string]any{
		"Title":   "Ficha Técnica · " + p.Name,
		"Project": p,
	})
}

func (a *App) TechnicalSheetSave(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = p.Name
	}
	status := r.FormValue("status")
	if _, exists := domain.ProjectStatusLabels[status]; !exists {
		status = p.Status
	}
	startDate := strings.TrimSpace(r.FormValue("start_date"))
	start := sql.NullString{}
	if startDate != "" {
		start = sql.NullString{String: startDate + "T00:00:00Z", Valid: true}
	}

	err := a.Q.UpdateProjectTechnicalSheet(r.Context(), repo.UpdateProjectTechnicalSheetParams{
		Name:         name,
		Description:  strings.TrimSpace(r.FormValue("description")),
		Status:       status,
		StartDate:    start,
		BaseCost:     formFloat(r, "base_cost", p.BaseCost),
		Currency:     "MXN",
		GitRepo:      strings.TrimSpace(r.FormValue("git_repo")),
		PrivateNotes: strings.TrimSpace(r.FormValue("private_notes")),
		ApiKeys:      strings.TrimSpace(r.FormValue("api_keys")),
		ServerAccess: strings.TrimSpace(r.FormValue("server_access")),
		StagingUrl:   strings.TrimSpace(r.FormValue("staging_url")),
		UpdatedAt:    domain.Now(),
		ID:           p.ID,
	})
	if err != nil {
		redirectErr(w, r, "/admin/projects/"+strconv.FormatInt(p.ID, 10)+"/technical-sheet", "No se pudo guardar la ficha técnica")
		return
	}
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "technical_sheet_updated", "Ficha técnica actualizada", u.ID)
	redirectMsg(w, r, "/admin/projects/"+strconv.FormatInt(p.ID, 10)+"/technical-sheet", "Ficha técnica guardada")
}

// ---------- SRS + AI ----------

func (a *App) SRSPage(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Error al cargar SRS", http.StatusInternalServerError)
		return
	}
	versions, _ := a.Q.ListSRSVersions(r.Context(), p.ID)
	pending, _ := a.Q.ListPendingProposedTasks(r.Context(), p.ID)
	allProposed, _ := a.Q.ListProposedTasks(r.Context(), p.ID)
	pendingRequests, _ := a.Q.ListPendingSRSRequests(r.Context(), p.ID)
	allRequests, _ := a.Q.ListSRSRequestsByProject(r.Context(), p.ID)
	proposals, _ := a.Q.ListSRSProposals(r.Context(), p.ID)
	pendingProposal, perr := a.Q.GetPendingSRSProposal(r.Context(), p.ID)

	var latestPtr *repo.SrsVersion
	if hasSRS {
		latestPtr = &latest
	}
	var proposalPtr *repo.SrsProposal
	if perr == nil {
		proposalPtr = &pendingProposal
	}

	a.page(w, r, http.StatusOK, "admin/workspace/srs.html", map[string]any{
		"Title":           "SRS Dual · " + p.Name,
		"Project":         p,
		"Latest":          latestPtr,
		"Versions":        versions,
		"PendingTasks":    pending,
		"AllProposed":     allProposed,
		"PendingRequests": pendingRequests,
		"AllRequests":     allRequests,
		"Proposals":       proposals,
		"PendingProposal": proposalPtr,
		"HasSRS":          hasSRS,
	})
}

func (a *App) SRSGenerate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	notes := strings.TrimSpace(r.FormValue("notes"))
	if notes == "" {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "Escribe las nuevas notas o cambios para actualizar el SRS")
		return
	}

	latest, err := a.Q.GetLatestSRS(r.Context(), p.ID)
	hasSRS := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo cargar el SRS actual")
		return
	}

	currentClient, currentDev := "", ""
	if hasSRS {
		currentClient = latest.ClientContent
		currentDev = latest.DevContent
	}

	out, err := a.Gemini.UpdateSRS(r.Context(), currentClient, currentDev, notes)
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "La IA no pudo actualizar el SRS: "+err.Error())
		return
	}

	_ = a.Q.SupersedePendingSRSProposals(r.Context(), p.ID)
	baseVersionID := sql.NullInt64{}
	if hasSRS {
		baseVersionID = sql.NullInt64{Int64: latest.ID, Valid: true}
	}

	u := userFrom(r.Context())
	proposal, err := a.Q.CreateSRSProposal(r.Context(), repo.CreateSRSProposalParams{
		ProjectID:     p.ID,
		BaseVersionID: baseVersionID,
		ClientContent: out.ClientContent,
		DevContent:    out.DevContent,
		SourceNotes:   notes,
		CreatedBy:     u.ID,
		CreatedAt:     domain.Now(),
	})
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo guardar la propuesta de SRS")
		return
	}
	a.logEvent(r.Context(), p.ID, "srs_proposal_created", fmt.Sprintf("Propuesta de SRS #%d generada con IA", proposal.ID), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "srs"), "Propuesta de SRS generada. Se publicará como nueva versión cuando el administrador y el cliente la autoricen.")
}

func (a *App) SRSSave(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	versionID := formInt(r, "version_id", 0)
	if versionID == 0 {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "Versión de SRS inválida")
		return
	}
	if err := a.Q.UpdateSRSContent(r.Context(), repo.UpdateSRSContentParams{
		ClientContent: r.FormValue("client_content"),
		DevContent:    r.FormValue("dev_content"),
		ID:            versionID,
	}); err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo guardar el SRS")
		return
	}
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "srs_updated", "SRS editado manualmente", u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "srs"), "SRS actualizado")
}

func (a *App) GenerateProposedTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	srs, err := a.Q.GetLatestSRS(r.Context(), p.ID)
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "Genera primero un SRS antes de proponer tareas")
		return
	}

	tasks, err := a.Gemini.ProposeTasks(r.Context(), srs.DevContent+"\n\n"+srs.ClientContent)
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "La IA no pudo proponer tareas: "+err.Error())
		return
	}

	for _, t := range tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			continue
		}
		_, _ = a.Q.CreateProposedTask(r.Context(), repo.CreateProposedTaskParams{
			ProjectID:     p.ID,
			SrsVersionID:  sql.NullInt64{Int64: srs.ID, Valid: true},
			AiTitle:       title,
			AiDescription: strings.TrimSpace(t.Description),
			CreatedAt:     domain.Now(),
		})
	}
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "tasks_proposed", fmt.Sprintf("%d tareas propuestas por IA", len(tasks)), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "srs"), fmt.Sprintf("%d tareas propuestas. Revísalas antes de enviarlas al backlog.", len(tasks)))
}

func (a *App) ApproveProposedTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	taskID, ok := pathID(r, "taskID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pt, err := a.Q.GetProposedTask(r.Context(), taskID)
	if err != nil || pt.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	order, _ := a.Q.MaxOrderInColumn(r.Context(), repo.MaxOrderInColumnParams{ProjectID: p.ID, Status: domain.TaskBacklog})
	var nextOrder int64
	if v, ok := order.(int64); ok {
		nextOrder = v + 1
	}
	now := domain.Now()
	_, err = a.Q.CreateTask(r.Context(), repo.CreateTaskParams{
		ProjectID:   p.ID,
		Title:       pt.AiTitle,
		Description: pt.AiDescription,
		Status:      domain.TaskBacklog,
		ColumnOrder: nextOrder,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "srs"), "No se pudo crear la tarea")
		return
	}
	_ = a.Q.SetProposedTaskStatus(r.Context(), repo.SetProposedTaskStatusParams{Status: "approved", ID: pt.ID})
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "task_added", "Tarea enviada al backlog: "+pt.AiTitle, u.ID)
	redirectMsg(w, r, a.proposalReturnPath(r, p.ID), "Tarea añadida al backlog")
}

func (a *App) RejectProposedTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	taskID, ok := pathID(r, "taskID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pt, err := a.Q.GetProposedTask(r.Context(), taskID)
	if err != nil || pt.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	_ = a.Q.SetProposedTaskStatus(r.Context(), repo.SetProposedTaskStatusParams{Status: "rejected", ID: pt.ID})
	redirectMsg(w, r, a.proposalReturnPath(r, p.ID), "Propuesta descartada")
}

// ---------- Kanban ----------

func (a *App) KanbanPage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	tasks, _ := a.Q.ListTasksByProject(r.Context(), p.ID)
	pending, _ := a.Q.ListPendingProposedTasks(r.Context(), p.ID)
	versions, _ := a.Q.ListSRSVersions(r.Context(), p.ID)

	var assigned *repo.SrsVersion
	if p.KanbanSrsVersionID.Valid {
		if v, err := a.Q.GetSRSVersion(r.Context(), p.KanbanSrsVersionID.Int64); err == nil {
			assigned = &v
		}
	}

	a.page(w, r, http.StatusOK, "admin/workspace/kanban.html", map[string]any{
		"Title":         "Kanban · " + p.Name,
		"Project":       p,
		"TasksByStatus": groupTasks(tasks),
		"AllTasks":      tasks,
		"PendingTasks":  pending,
		"SRSVersions":   versions,
		"AssignedSRS":   assigned,
	})
}

func (a *App) TaskCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "El título de la tarea es obligatorio")
		return
	}
	status := r.FormValue("status")
	if !domain.IsValidTaskStatus(status) {
		status = domain.TaskBacklog
	}
	maxOrder, _ := a.Q.MaxOrderInColumn(r.Context(), repo.MaxOrderInColumnParams{ProjectID: p.ID, Status: status})
	var nextOrder int64
	if v, ok := maxOrder.(int64); ok {
		nextOrder = v + 1
	}
	now := domain.Now()
	_, err := a.Q.CreateTask(r.Context(), repo.CreateTaskParams{
		ProjectID:   p.ID,
		Title:       title,
		Description: strings.TrimSpace(r.FormValue("description")),
		Status:      status,
		ColumnOrder: nextOrder,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "No se pudo crear la tarea")
		return
	}
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "task_created", "Tarea creada: "+title, u.ID)
	a.renderKanban(w, r, p)
}

func (a *App) TaskMove(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	taskID, ok := pathID(r, "taskID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	task, err := a.Q.GetTask(r.Context(), taskID)
	if err != nil || task.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	status := r.FormValue("status")
	if !domain.IsValidTaskStatus(status) {
		http.Error(w, "Estado inválido", http.StatusBadRequest)
		return
	}
	order := formInt(r, "order", 0)
	oldStatus := task.Status
	if err := a.Q.MoveTask(r.Context(), repo.MoveTaskParams{
		Status:      status,
		ColumnOrder: order,
		UpdatedAt:   domain.Now(),
		ID:          task.ID,
	}); err != nil {
		http.Error(w, "No se pudo mover la tarea", http.StatusInternalServerError)
		return
	}
	if err := a.renumberColumn(r.Context(), p.ID, status, task.ID, int(order)); err != nil {
		http.Error(w, "No se pudo reordenar la columna", http.StatusInternalServerError)
		return
	}
	if oldStatus != status {
		_ = a.renumberColumn(r.Context(), p.ID, oldStatus, 0, 0)
		u := userFrom(r.Context())
		a.logEvent(r.Context(), p.ID, "task_moved", fmt.Sprintf("%s → %s", task.Title, domain.TaskStatusLabels[status]), u.ID)
	}
	a.renderKanban(w, r, p)
}

func (a *App) TaskDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	taskID, ok := pathID(r, "taskID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	task, err := a.Q.GetTask(r.Context(), taskID)
	if err != nil || task.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	_ = a.Q.DeleteTask(r.Context(), taskID)
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "task_deleted", "Tarea eliminada: "+task.Title, u.ID)
	a.renderKanban(w, r, p)
}

func (a *App) KanbanSetSRSVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	versionID := formInt(r, "srs_version_id", 0)
	u := userFrom(r.Context())

	if versionID == 0 {
		_ = a.Q.SetProjectKanbanSRSVersion(r.Context(), repo.SetProjectKanbanSRSVersionParams{
			KanbanSrsVersionID: sql.NullInt64{},
			UpdatedAt:          domain.Now(),
			ID:                 p.ID,
		})
		a.logEvent(r.Context(), p.ID, "kanban_srs_unassigned", "Versión SRS desvinculada del tablero", u.ID)
		redirectMsg(w, r, a.workspacePath(p.ID, "kanban"), "Versión SRS desvinculada del tablero")
		return
	}

	v, err := a.Q.GetSRSVersion(r.Context(), versionID)
	if err != nil || v.ProjectID != p.ID {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "La versión seleccionada no pertenece al proyecto")
		return
	}
	if err := a.Q.SetProjectKanbanSRSVersion(r.Context(), repo.SetProjectKanbanSRSVersionParams{
		KanbanSrsVersionID: sql.NullInt64{Int64: v.ID, Valid: true},
		UpdatedAt:          domain.Now(),
		ID:                 p.ID,
	}); err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "No se pudo vincular la versión al tablero")
		return
	}
	a.logEvent(r.Context(), p.ID, "kanban_srs_assigned", fmt.Sprintf("SRS v%d vinculado al tablero", v.Version), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "kanban"), fmt.Sprintf("SRS v%d vinculado al tablero", v.Version))
}

func (a *App) KanbanValidateBacklog(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	if !p.KanbanSrsVersionID.Valid {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "Asigna una versión del SRS al tablero antes de validar el backlog")
		return
	}
	v, err := a.Q.GetSRSVersion(r.Context(), p.KanbanSrsVersionID.Int64)
	if err != nil || v.ProjectID != p.ID {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "La versión vinculada no es válida")
		return
	}

	tasks, _ := a.Q.ListTasksByProject(r.Context(), p.ID)
	existing := make([]string, 0, len(tasks))
	seen := map[string]bool{}
	for _, t := range tasks {
		existing = append(existing, t.Title)
		seen[strings.ToLower(strings.TrimSpace(t.Title))] = true
	}
	if proposed, err := a.Q.ListProposedTasks(r.Context(), p.ID); err == nil {
		for _, pt := range proposed {
			if pt.Status == "pending" {
				seen[strings.ToLower(strings.TrimSpace(pt.AiTitle))] = true
			}
		}
	}

	out, err := a.Gemini.ValidateBacklog(r.Context(), v.DevContent+"\n\n"+v.ClientContent, existing)
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "kanban"), "La IA no pudo validar el backlog: "+err.Error())
		return
	}

	created := 0
	for _, pt := range out {
		title := strings.TrimSpace(pt.Title)
		if title == "" {
			continue
		}
		key := strings.ToLower(title)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, err := a.Q.CreateProposedTask(r.Context(), repo.CreateProposedTaskParams{
			ProjectID:     p.ID,
			SrsVersionID:  sql.NullInt64{Int64: v.ID, Valid: true},
			AiTitle:       title,
			AiDescription: strings.TrimSpace(pt.Description),
			CreatedAt:     domain.Now(),
		}); err != nil {
			continue
		}
		created++
	}

	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "kanban_backlog_validated", fmt.Sprintf("Backlog validado contra SRS v%d: %d propuestas", v.Version, created), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "kanban"), fmt.Sprintf("Backlog validado contra SRS v%d. %d tareas propuestas para revisión.", v.Version, created))
}

func (a *App) renderKanban(w http.ResponseWriter, r *http.Request, p *repo.Project) {
	tasks, _ := a.Q.ListTasksByProject(r.Context(), p.ID)
	a.partial(w, r, "kanban_board", map[string]any{
		"Project":       p,
		"TasksByStatus": groupTasks(tasks),
	})
}

// renumberColumn keeps column_order contiguous. When movedID > 0 it is inserted at "at".
func (a *App) renumberColumn(ctx context.Context, projectID int64, status string, movedID int64, at int) error {
	tasks, err := a.Q.ListTasksByStatus(ctx, repo.ListTasksByStatusParams{ProjectID: projectID, Status: status})
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(tasks)+1)
	for _, t := range tasks {
		if movedID != 0 && t.ID == movedID {
			continue
		}
		ids = append(ids, t.ID)
	}
	if movedID != 0 {
		if at < 0 {
			at = 0
		}
		if at > len(ids) {
			at = len(ids)
		}
		ids = append(ids, 0)
		copy(ids[at+1:], ids[at:])
		ids[at] = movedID
	}
	now := domain.Now()
	for i, id := range ids {
		if err := a.Q.MoveTask(ctx, repo.MoveTaskParams{
			Status:      status,
			ColumnOrder: int64(i),
			UpdatedAt:   now,
			ID:          id,
		}); err != nil {
			return err
		}
	}
	return nil
}

func groupTasks(tasks []repo.Task) map[string][]repo.Task {
	out := map[string][]repo.Task{}
	for _, s := range domain.TaskStatuses {
		out[s] = []repo.Task{}
	}
	for _, t := range tasks {
		out[t.Status] = append(out[t.Status], t)
	}
	return out
}

// ---------- Deliverables (admin) ----------

func (a *App) AdminDeliverables(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	deliverables, _ := a.Q.ListDeliverables(r.Context(), p.ID)
	tickets, _ := a.Q.ListTicketsByProject(r.Context(), p.ID)

	a.page(w, r, http.StatusOK, "admin/workspace/deliverables.html", map[string]any{
		"Title":        "Entregables · " + p.Name,
		"Project":      p,
		"Deliverables": deliverables,
		"Tickets":      tickets,
	})
}

func (a *App) AdminCreateDeliverable(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	urlDemo := strings.TrimSpace(r.FormValue("url_demo"))
	if urlDemo == "" {
		redirectErr(w, r, a.workspacePath(p.ID, "deliverables"), "El enlace de staging es obligatorio")
		return
	}
	sprint := formInt(r, "sprint_num", 1)
	_, err := a.Q.CreateDeliverable(r.Context(), repo.CreateDeliverableParams{
		ProjectID: p.ID,
		SprintNum: sprint,
		Title:     strings.TrimSpace(r.FormValue("title")),
		UrlDemo:   urlDemo,
		Notes:     strings.TrimSpace(r.FormValue("notes")),
		CreatedAt: domain.Now(),
	})
	if err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "deliverables"), "No se pudo crear el entregable")
		return
	}
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "deliverable_created", fmt.Sprintf("Entregable del Sprint %d publicado", sprint), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "deliverables"), "Entregable publicado")
}

func (a *App) AdminAcceptDeliverable(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	deliverableID, ok := pathID(r, "deliverableID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	d, err := a.Q.GetDeliverable(r.Context(), deliverableID)
	if err != nil || d.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	_ = a.Q.AcceptDeliverable(r.Context(), repo.AcceptDeliverableParams{
		AcceptedAt: sql.NullString{String: domain.Now(), Valid: true},
		ID:         d.ID,
	})
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "deliverable_accepted", fmt.Sprintf("Entregable del Sprint %d aceptado", d.SprintNum), u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "deliverables"), "Entregable marcado como aceptado")
}

func (a *App) AdminDeleteDeliverable(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	deliverableID, ok := pathID(r, "deliverableID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	d, err := a.Q.GetDeliverable(r.Context(), deliverableID)
	if err != nil || d.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	_ = a.Q.DeleteDeliverable(r.Context(), d.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "deliverables"), "Entregable eliminado")
}

// ---------- Resources (shared) ----------

func (a *App) AdminResources(w http.ResponseWriter, r *http.Request) {
	a.resourcesPage(w, r, "admin/workspace/resources.html")
}

func (a *App) resourcesPage(w http.ResponseWriter, r *http.Request, tmpl string) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	resources, _ := a.Q.ListResourcesWithUploader(r.Context(), p.ID)
	u := userFrom(r.Context())
	a.page(w, r, http.StatusOK, tmpl, map[string]any{
		"Title":         "Recursos · " + p.Name,
		"Project":       p,
		"Resources":     resources,
		"CanUpload":     true,
		"IsAdmin":       u.Role == domain.RoleAdmin,
		"ActiveSection": "recursos",
	})
}

func (a *App) ResourceUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	back := "/client/projects/" + strconv.FormatInt(p.ID, 10) + "/resources"
	if userFrom(r.Context()).Role == domain.RoleAdmin {
		redirectErr(w, r, a.workspacePath(p.ID, "resources"), "El administrador no puede subir archivos; solo gestiona los del cliente")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		redirectErr(w, r, back, "Selecciona un archivo para subir")
		return
	}
	defer file.Close()

	const maxSize = 25 << 20
	if header.Size <= 0 {
		redirectErr(w, r, back, "El archivo está vacío")
		return
	}
	if header.Size > maxSize {
		redirectErr(w, r, back, "El archivo supera el límite de 25 MB")
		return
	}

	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(filepath.Base(header.Filename), ext)
	key := fmt.Sprintf("projects/%d/%d-%s%s", p.ID, time.Now().UnixNano(), domain.Slugify(base), ext)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			contentType = ct
		} else {
			contentType = "application/octet-stream"
		}
	}

	if err := a.Store.Put(r.Context(), key, file, header.Size, contentType); err != nil {
		redirectErr(w, r, back, "No se pudo subir el archivo: "+err.Error())
		return
	}

	u := userFrom(r.Context())
	if _, err := a.Q.CreateResource(r.Context(), repo.CreateResourceParams{
		ProjectID:   p.ID,
		UploadedBy:  u.ID,
		FileName:    header.Filename,
		R2Key:       key,
		ContentType: contentType,
		SizeBytes:   header.Size,
		CreatedAt:   domain.Now(),
	}); err != nil {
		_ = a.Store.Delete(r.Context(), key)
		redirectErr(w, r, back, "No se pudo registrar el archivo")
		return
	}

	a.logEvent(r.Context(), p.ID, "resource_uploaded", "Archivo subido: "+header.Filename, u.ID)
	if u.Role == domain.RoleClient {
		go func(project, client, filename string) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = a.WhatsApp.SendResourceAlert(ctx, project, client, filename)
		}(p.Name, u.Name, header.Filename)
		redirectMsg(w, r, back, "Archivo subido y administrador notificado")
		return
	}
	redirectMsg(w, r, back, "Archivo subido correctamente")
}

// ---------- Finances ----------

func (a *App) AdminFinances(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	sums, _ := a.Q.SumBillings(r.Context(), p.ID)
	claims, _ := a.Q.ListPaymentClaimsByProject(r.Context(), p.ID)
	paidBase, _ := a.Q.SumPaidBase(r.Context(), p.ID)

	a.page(w, r, http.StatusOK, "admin/workspace/finances.html", map[string]any{
		"Title":     "Finanzas · " + p.Name,
		"Project":   p,
		"Totals":    sumsMap(sums),
		"Claims":    claims,
		"PaidBase":  asFloat(paidBase),
		"Remaining": remainingBase(p.BaseCost, paidBase),
	})
}

func (a *App) ClientFinances(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	billings, _ := a.Q.ListBillings(r.Context(), p.ID)
	sums, _ := a.Q.SumBillings(r.Context(), p.ID)
	claims, _ := a.Q.ListPaymentClaimsByProject(r.Context(), p.ID)
	paidBase, _ := a.Q.SumPaidBase(r.Context(), p.ID)

	a.page(w, r, http.StatusOK, "client/finances.html", map[string]any{
		"Title":         "Finanzas · " + p.Name,
		"Project":       p,
		"Billings":      billings,
		"Totals":        sumsMap(sums),
		"Claims":        claims,
		"PaidBase":      asFloat(paidBase),
		"Remaining":     remainingBase(p.BaseCost, paidBase),
		"ActiveSection": "finanzas",
	})
}

func (a *App) AdminTickets(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	tickets, _ := a.Q.ListTicketsByProject(r.Context(), p.ID)

	a.page(w, r, http.StatusOK, "admin/workspace/tickets.html", map[string]any{
		"Title":   "Tickets · " + p.Name,
		"Project": p,
		"Tickets": tickets,
	})
}

func (a *App) AdminUpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	ticketID, ok := pathID(r, "ticketID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	t, err := a.Q.GetTicket(r.Context(), ticketID)
	if err != nil || t.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	status := r.FormValue("status")
	switch status {
	case "open", "in_progress", "closed":
	default:
		status = t.Status
	}
	_ = a.Q.UpdateTicketStatus(r.Context(), repo.UpdateTicketStatusParams{
		Status:     status,
		AdminReply: strings.TrimSpace(r.FormValue("admin_reply")),
		ID:         t.ID,
	})
	redirectMsg(w, r, a.workspacePath(p.ID, "tickets"), "Ticket actualizado")
}

func (a *App) AdminDeleteResource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, ok := a.projectFor(w, r, id)
	if !ok {
		return
	}
	resourceID, ok := pathID(r, "resourceID")
	if !ok {
		http.NotFound(w, r)
		return
	}
	res, err := a.Q.GetResource(r.Context(), resourceID)
	if err != nil || res.ProjectID != p.ID {
		http.NotFound(w, r)
		return
	}
	_ = a.Store.Delete(r.Context(), res.R2Key)
	if err := a.Q.DeleteResource(r.Context(), res.ID); err != nil {
		redirectErr(w, r, a.workspacePath(p.ID, "resources"), "No se pudo eliminar el archivo")
		return
	}
	u := userFrom(r.Context())
	a.logEvent(r.Context(), p.ID, "resource_deleted", "Archivo eliminado: "+res.FileName, u.ID)
	redirectMsg(w, r, a.workspacePath(p.ID, "resources"), "Archivo eliminado")
}

// ---------- helpers ----------

func (a *App) proposalReturnPath(r *http.Request, projectID int64) string {
	if r.FormValue("from") == "kanban" {
		return a.workspacePath(projectID, "kanban")
	}
	return a.workspacePath(projectID, "srs")
}

func (a *App) workspacePath(projectID int64, section string) string {
	return "/admin/projects/" + strconv.FormatInt(projectID, 10) + "/" + section
}

func (a *App) resourcePath(projectID int64) string {
	return "/admin/projects/" + strconv.FormatInt(projectID, 10) + "/resources"
}

func remainingBase(baseCost float64, paidBase any) float64 {
	r := baseCost - asFloat(paidBase)
	if r < 0 {
		return 0
	}
	return r
}

func sumsMap(s repo.SumBillingsRow) map[string]float64 {
	return map[string]float64{
		"paid":    asFloat(s.TotalPaid),
		"pending": asFloat(s.TotalPending),
		"extras":  asFloat(s.TotalExtras),
	}
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	case []byte:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	default:
		return 0
	}
}
