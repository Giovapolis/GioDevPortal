package handler

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"giodevportal/internal/config"
	"giodevportal/internal/db"
	"giodevportal/internal/domain"
	repo "giodevportal/internal/repository/sqlc"
	"giodevportal/internal/service/auth"
	"giodevportal/internal/service/email"
	render "giodevportal/internal/template"
	"giodevportal/pkg/gemini"
	"giodevportal/pkg/storage"
	"giodevportal/pkg/whatsapp"
)

type testEnv struct {
	app *App
	srv *httptest.Server
	db  *sql.DB
}

func setup(t *testing.T) *testEnv {
	t.Helper()

	cfg := &config.Config{
		Env:               "test",
		ServerAddr:        ":0",
		DBPath:            ":memory:",
		DataDir:           t.TempDir(),
		JWTSecret:         "integration-test-secret-key-1234567890",
		AccessTTL:         15 * time.Minute,
		RefreshTTL:        time.Hour,
		GeminiModel:       "mock",
		EmailFrom:         "test@localhost",
		AppBaseURL:        "http://localhost",
		SeedAdminName:     "Admin Test",
		SeedAdminEmail:    "admin@test.local",
		SeedAdminPassword: "Admin123!",
		SeedDemo:          true,
	}

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	q := repo.New(conn)
	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	emailSvc := email.New("", cfg.EmailFrom)
	wa := whatsapp.New("", "", "", "", "", "", "")
	gem, err := gemini.New(context.Background(), "", cfg.GeminiModel)
	if err != nil {
		t.Fatalf("gemini: %v", err)
	}
	store, err := storage.NewLocal(filepath.Join(cfg.DataDir, "uploads"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	rnd, err := render.New("../../web/templates")
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	app := NewApp(cfg, q, conn, authSvc, emailSvc, gem, wa, store, rnd)
	if err := app.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	srv := httptest.NewServer(app.Router())
	t.Cleanup(srv.Close)

	return &testEnv{app: app, srv: srv, db: conn}
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 10 * time.Second,
	}
}

func (e *testEnv) csrf(t *testing.T, c *http.Client) string {
	t.Helper()
	resp, err := c.Get(e.srv.URL + "/login")
	if err != nil {
		t.Fatalf("get login: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	u, _ := url.Parse(e.srv.URL)
	for _, ck := range c.Jar.Cookies(u) {
		if ck.Name == csrfCookie {
			return ck.Value
		}
	}
	t.Fatal("csrf cookie not set")
	return ""
}

func (e *testEnv) login(t *testing.T, c *http.Client, email, password string) string {
	t.Helper()
	token := e.csrf(t, c)
	form := url.Values{}
	form.Set("csrf_token", token)
	form.Set("email", email)
	form.Set("password", password)

	resp, err := c.PostForm(e.srv.URL+"/login", form)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", resp.StatusCode)
	}
	return token
}

func (e *testEnv) post(t *testing.T, c *http.Client, path, token string, values url.Values) *http.Response {
	t.Helper()
	values.Set("csrf_token", token)
	resp, err := c.PostForm(e.srv.URL+path, values)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func (e *testEnv) get(t *testing.T, c *http.Client, path string) *http.Response {
	t.Helper()
	resp, err := c.Get(e.srv.URL + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	return resp
}

func TestAuthenticationAndRoleIsolation(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	e.login(t, admin, "admin@test.local", "Admin123!")

	resp := e.get(t, admin, "/admin")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin /admin = %d, want 200", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Panel Global") {
		t.Error("admin dashboard missing expected content")
	}

	resp = e.get(t, admin, "/client")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("admin /client = %d, want 403", resp.StatusCode)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	client := newHTTPClient()
	e.login(t, client, "cliente@demo.local", "Cliente123!")

	resp = e.get(t, client, "/client/projects/1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client project = %d, want 200", resp.StatusCode)
	}
	body = readBody(t, resp)
	if !strings.Contains(body, "Portal de Reservas") {
		t.Error("client dashboard missing project")
	}
	if strings.Contains(body, "Ficha técnica") {
		t.Error("client must not see technical sheet navigation")
	}

	resp = e.get(t, client, "/admin")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("client /admin = %d, want 403", resp.StatusCode)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	anon := newHTTPClient()
	resp = e.get(t, anon, "/admin")
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("anon /admin = %d, want 303", resp.StatusCode)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// seeded demo has an accepted and a current deliverable
	if _, err := e.app.Q.GetCurrentDeliverable(ctx, 1); err != nil {
		t.Fatalf("current deliverable: %v", err)
	}
	accepted, err := e.app.Q.ListAcceptedDeliverables(ctx, 1)
	if err != nil || len(accepted) == 0 {
		t.Fatalf("expected accepted deliverable, got %d (err=%v)", len(accepted), err)
	}
}

func TestKanbanMovePersists(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	token := e.login(t, admin, "admin@test.local", "Admin123!")

	tasks, err := e.app.Q.ListTasksByProject(ctx, 1)
	if err != nil || len(tasks) == 0 {
		t.Fatalf("list tasks: %v", err)
	}
	task := tasks[0]

	form := url.Values{}
	form.Set("status", domain.TaskDone)
	form.Set("order", "0")
	resp := e.post(t, admin, "/admin/projects/1/kanban/tasks/"+itoa(task.ID)+"/move", token, form)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("move = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "kanban-board") {
		t.Error("move response missing kanban board partial")
	}

	moved, err := e.app.Q.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if moved.Status != domain.TaskDone {
		t.Fatalf("task status = %q, want %q", moved.Status, domain.TaskDone)
	}
}

func TestCSRFRequiredForMutations(t *testing.T) {
	e := setup(t)

	admin := newHTTPClient()
	e.login(t, admin, "admin@test.local", "Admin123!")

	form := url.Values{}
	form.Set("title", "No CSRF")
	form.Set("status", "backlog")
	resp, err := admin.PostForm(e.srv.URL+"/admin/projects/1/kanban/tasks", form)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestClientTicketTriggersNotificationAndPersists(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	client := newHTTPClient()
	token := e.login(t, client, "cliente@demo.local", "Cliente123!")

	current, err := e.app.Q.GetCurrentDeliverable(ctx, 1)
	if err != nil {
		t.Fatalf("current deliverable: %v", err)
	}

	form := url.Values{}
	form.Set("deliverable_id", itoa(current.ID))
	form.Set("message", "El botón de reserva falla en móvil")
	resp := e.post(t, client, "/client/projects/1/deliverables/"+itoa(current.ID)+"/tickets", token, form)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ticket = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Nota enviada") {
		t.Errorf("unexpected ticket response: %s", strings.TrimSpace(body))
	}

	tickets, err := e.app.Q.ListTicketsByProject(ctx, 1)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	found := false
	for _, tk := range tickets {
		if tk.ClientMessage == "El botón de reserva falla en móvil" {
			found = true
		}
	}
	if !found {
		t.Fatal("ticket was not persisted")
	}
}

func TestSRSGenerationCreatesProposalWithoutPublishing(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	token := e.login(t, admin, "admin@test.local", "Admin123!")

	form := url.Values{}
	form.Set("notes", "Agregar reportes por sucursal y notificaciones al cliente")
	resp := e.post(t, admin, "/admin/projects/1/srs/generate", token, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("srs generate = %d, want 303", resp.StatusCode)
	}

	// Generating an update must NOT publish a new version immediately.
	latest, err := e.app.Q.GetLatestSRS(ctx, 1)
	if err != nil {
		t.Fatalf("latest srs: %v", err)
	}
	if latest.Version != 1 {
		t.Fatalf("version = %d, want 1 (no auto-publish)", latest.Version)
	}

	proposal, err := e.app.Q.GetPendingSRSProposal(ctx, 1)
	if err != nil {
		t.Fatalf("pending proposal: %v", err)
	}
	if proposal.AdminApproved != 0 || proposal.ClientApproved != 0 {
		t.Fatalf("proposal should start unauthorized: %+v", proposal)
	}
	if !strings.Contains(proposal.ClientContent, "## Actualización propuesta") {
		t.Fatal("proposal should merge current content with new notes")
	}

	resp = e.post(t, admin, "/admin/projects/1/srs/tasks/generate", token, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("tasks generate = %d, want 303", resp.StatusCode)
	}
	pending, err := e.app.Q.ListPendingProposedTasks(ctx, 1)
	if err != nil || len(pending) == 0 {
		t.Fatalf("pending tasks = %d err=%v", len(pending), err)
	}
}

func TestSRSProposalRequiresBothAuthorizations(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	adminToken := e.login(t, admin, "admin@test.local", "Admin123!")
	form := url.Values{}
	form.Set("notes", "Agregar reprogramación de reservas")
	resp := e.post(t, admin, "/admin/projects/1/srs/generate", adminToken, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("srs generate = %d, want 303", resp.StatusCode)
	}
	proposal, err := e.app.Q.GetPendingSRSProposal(ctx, 1)
	if err != nil {
		t.Fatalf("pending proposal: %v", err)
	}

	// Admin authorization alone must not publish.
	resp = e.post(t, admin, "/admin/projects/1/srs/proposals/"+itoa(proposal.ID)+"/approve", adminToken, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("admin approve = %d, want 303", resp.StatusCode)
	}
	latest, _ := e.app.Q.GetLatestSRS(ctx, 1)
	if latest.Version != 1 {
		t.Fatalf("version published before client approval: %d", latest.Version)
	}
	afterAdmin, _ := e.app.Q.GetSRSProposal(ctx, proposal.ID)
	if afterAdmin.AdminApproved != 1 || afterAdmin.ClientApproved != 0 {
		t.Fatalf("unexpected approvals after admin: %+v", afterAdmin)
	}

	// Client authorization publishes the version.
	client := newHTTPClient()
	clientToken := e.login(t, client, "cliente@demo.local", "Cliente123!")
	resp = e.post(t, client, "/client/projects/1/srs/proposals/"+itoa(proposal.ID)+"/approve", clientToken, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("client approve = %d, want 303", resp.StatusCode)
	}

	latest, err = e.app.Q.GetLatestSRS(ctx, 1)
	if err != nil {
		t.Fatalf("latest srs: %v", err)
	}
	if latest.Version != 2 {
		t.Fatalf("version = %d, want 2 after both approvals", latest.Version)
	}
	applied, _ := e.app.Q.GetSRSProposal(ctx, proposal.ID)
	if applied.Status != "applied" || !applied.AppliedVersionID.Valid {
		t.Fatalf("proposal not applied: %+v", applied)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := setup(t)

	c := newHTTPClient()
	token := e.csrf(t, c)

	limited := 0
	for i := 0; i < 8; i++ {
		form := url.Values{}
		form.Set("csrf_token", token)
		form.Set("email", "admin@test.local")
		form.Set("password", "wrong-password")
		resp, err := c.PostForm(e.srv.URL+"/login", form)
		if err != nil {
			t.Fatalf("login attempt: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("expected at least one 429 after burst")
	}
}

func TestKanbanReorderIsContiguous(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	token := e.login(t, admin, "admin@test.local", "Admin123!")

	now := domain.Now()
	extra, err := e.app.Q.CreateTask(ctx, repo.CreateTaskParams{
		ProjectID:   1,
		Title:       "Tarea extra",
		Description: "",
		Status:      domain.TaskDoing,
		ColumnOrder: 9,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	form := url.Values{}
	form.Set("status", domain.TaskDoing)
	form.Set("order", "0")
	resp := e.post(t, admin, "/admin/projects/1/kanban/tasks/"+itoa(extra.ID)+"/move", token, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("move = %d, want 200", resp.StatusCode)
	}

	col, err := e.app.Q.ListTasksByStatus(ctx, repo.ListTasksByStatusParams{ProjectID: 1, Status: domain.TaskDoing})
	if err != nil {
		t.Fatalf("list column: %v", err)
	}
	if len(col) < 2 {
		t.Fatalf("expected at least 2 tasks, got %d", len(col))
	}
	if col[0].ID != extra.ID {
		t.Fatalf("moved task should be first, got id %d", col[0].ID)
	}
	for i, task := range col {
		if task.ColumnOrder != int64(i) {
			t.Fatalf("column_order not contiguous at %d: got %d", i, task.ColumnOrder)
		}
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func (e *testEnv) postMultipart(t *testing.T, c *http.Client, path, token string, fields map[string]string, fileField, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("csrf_token", token); err != nil {
		t.Fatalf("write csrf: %v", err)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	fw, err := mw.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, e.srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("post multipart: %v", err)
	}
	return resp
}

func TestClientPaymentClaimPaidAndPendingToggle(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	client := newHTTPClient()
	clientToken := e.login(t, client, "cliente@demo.local", "Cliente123!")

	resp := e.postMultipart(t, client, "/client/projects/1/payments", clientToken,
		map[string]string{"amount": "1200", "note": "Transferencia demo"},
		"proof", "comprobante.txt", []byte("comprobante de pago"))
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("payment claim = %d, want 303", resp.StatusCode)
	}

	claims, err := e.app.Q.ListPaymentClaimsByProject(ctx, 1)
	if err != nil {
		t.Fatalf("list claims: %v", err)
	}
	var claim *repo.ListPaymentClaimsByProjectRow
	for i := range claims {
		if claims[i].Amount == 1200 {
			claim = &claims[i]
			break
		}
	}
	if claim == nil {
		t.Fatal("new payment claim not found")
	}
	if claim.Status != "pending" {
		t.Fatalf("claim status = %q, want pending", claim.Status)
	}

	admin := newHTTPClient()
	adminToken := e.login(t, admin, "admin@test.local", "Admin123!")
	form := url.Values{}
	form.Set("admin_note", "Verificado")
	resp = e.post(t, admin, "/admin/projects/1/payments/"+itoa(claim.ID)+"/paid", adminToken, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("mark paid = %d, want 303", resp.StatusCode)
	}

	paidClaim, err := e.app.Q.GetPaymentClaim(ctx, claim.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if paidClaim.Status != "paid" || !paidClaim.BillingID.Valid {
		t.Fatalf("claim not paid with billing: %+v", paidClaim)
	}
	paidBase, _ := e.app.Q.SumPaidBase(ctx, 1)
	if asFloat(paidBase) < 3600 {
		t.Fatalf("paid base = %v, want >= 3600", asFloat(paidBase))
	}

	// Toggle back to pending: removes the billing and lowers paid base.
	resp = e.post(t, admin, "/admin/projects/1/payments/"+itoa(claim.ID)+"/pending", adminToken, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	pendingClaim, _ := e.app.Q.GetPaymentClaim(ctx, claim.ID)
	if pendingClaim.Status != "pending" || pendingClaim.BillingID.Valid {
		t.Fatalf("claim not returned to pending: %+v", pendingClaim)
	}
	paidBase, _ = e.app.Q.SumPaidBase(ctx, 1)
	if asFloat(paidBase) != 2400 {
		t.Fatalf("paid base after pending = %v, want 2400", asFloat(paidBase))
	}
}

func TestAdminCannotUploadResources(t *testing.T) {
	e := setup(t)

	admin := newHTTPClient()
	token := e.login(t, admin, "admin@test.local", "Admin123!")

	resp := e.postMultipart(t, admin, "/admin/projects/1/resources", token, map[string]string{}, "file", "x.txt", []byte("x"))
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusSeeOther {
		t.Fatalf("admin upload unexpectedly allowed: %d", resp.StatusCode)
	}
}

func TestAdminDeletesClientResource(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	client := newHTTPClient()
	clientToken := e.login(t, client, "cliente@demo.local", "Cliente123!")
	resp := e.postMultipart(t, client, "/client/projects/1/resources", clientToken, map[string]string{}, "file", "manual.txt", []byte("manual"))
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("upload = %d, want 303", resp.StatusCode)
	}

	resources, err := e.app.Q.ListResources(ctx, 1)
	if err != nil || len(resources) != 1 {
		t.Fatalf("resources = %d, err=%v", len(resources), err)
	}

	admin := newHTTPClient()
	adminToken := e.login(t, admin, "admin@test.local", "Admin123!")
	resp = e.post(t, admin, "/admin/projects/1/resources/"+itoa(resources[0].ID)+"/delete", adminToken, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete = %d, want 303", resp.StatusCode)
	}

	after, err := e.app.Q.ListResources(ctx, 1)
	if err != nil || len(after) != 0 {
		t.Fatalf("resources after delete = %d, err=%v", len(after), err)
	}
}

func TestClientSRSRequestApprovedCreatesVersion(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	client := newHTTPClient()
	clientToken := e.login(t, client, "cliente@demo.local", "Cliente123!")
	form := url.Values{}
	form.Set("content", "Agregar caso de uso: reprogramar reserva hasta 24 horas antes.")
	resp := e.post(t, client, "/client/projects/1/requests", clientToken, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("request = %d, want 303", resp.StatusCode)
	}

	requests, err := e.app.Q.ListPendingSRSRequests(ctx, 1)
	if err != nil || len(requests) != 1 {
		t.Fatalf("pending requests = %d, err=%v", len(requests), err)
	}

	admin := newHTTPClient()
	adminToken := e.login(t, admin, "admin@test.local", "Admin123!")
	resp = e.post(t, admin, "/admin/projects/1/srs/requests/"+itoa(requests[0].ID)+"/approve", adminToken, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("approve request = %d, want 303", resp.StatusCode)
	}

	latest, err := e.app.Q.GetLatestSRS(ctx, 1)
	if err != nil {
		t.Fatalf("latest srs: %v", err)
	}
	if latest.Version != 2 {
		t.Fatalf("version = %d, want 2", latest.Version)
	}
	if !strings.Contains(latest.ClientContent, "reprogramar reserva") {
		t.Fatal("approved client request not applied to SRS")
	}

	approved, err := e.app.Q.GetSRSRequest(ctx, requests[0].ID)
	if err != nil || approved.Status != "approved" || !approved.SrsVersionID.Valid {
		t.Fatalf("request not approved: %+v err=%v", approved, err)
	}
}

func TestKanbanAssignSRSVersionAndValidate(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	token := e.login(t, admin, "admin@test.local", "Admin123!")

	form := url.Values{}
	form.Set("srs_version_id", "1")
	resp := e.post(t, admin, "/admin/projects/1/kanban/srs", token, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("assign version = %d, want 303", resp.StatusCode)
	}
	project, err := e.app.Q.GetProject(ctx, 1)
	if err != nil || !project.KanbanSrsVersionID.Valid || project.KanbanSrsVersionID.Int64 != 1 {
		t.Fatalf("kanban srs version not set: %+v err=%v", project, err)
	}

	before, _ := e.app.Q.ListPendingProposedTasks(ctx, 1)
	resp = e.post(t, admin, "/admin/projects/1/kanban/validate", token, url.Values{})
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("validate backlog = %d, want 303", resp.StatusCode)
	}
	after, _ := e.app.Q.ListPendingProposedTasks(ctx, 1)
	if len(after) <= len(before) {
		t.Fatalf("expected new proposals, before=%d after=%d", len(before), len(after))
	}
	for _, pt := range after {
		if pt.SrsVersionID.Valid && pt.SrsVersionID.Int64 == 1 {
			return
		}
	}
	t.Fatal("expected proposals linked to SRS v1")
}

func TestCurrencyIsAlwaysMXN(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	admin := newHTTPClient()
	token := e.login(t, admin, "admin@test.local", "Admin123!")

	form := url.Values{}
	form.Set("name", "Proyecto MXN")
	form.Set("client_id", "2")
	form.Set("base_cost", "1000")
	form.Set("currency", "USD")
	resp := e.post(t, admin, "/admin/projects", token, form)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	projects, err := e.app.Q.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	found := false
	for _, p := range projects {
		if p.Name == "Proyecto MXN" {
			found = true
			if p.Currency != "MXN" {
				t.Fatalf("currency = %q, want MXN", p.Currency)
			}
		}
	}
	if !found {
		t.Fatal("project not created")
	}
}
