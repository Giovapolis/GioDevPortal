package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type SRS struct {
	ClientContent string `json:"client_content"`
	DevContent    string `json:"dev_content"`
}

type ProposedTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Client struct {
	api   *genai.Client
	model string
}

func New(ctx context.Context, apiKey, model string) (*Client, error) {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	if strings.TrimSpace(apiKey) == "" {
		return &Client{model: model}, nil
	}
	api, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini client: %w", err)
	}
	return &Client{api: api, model: model}, nil
}

func (c *Client) Enabled() bool { return c.api != nil }

// GenerateSRS turns natural language notes into a dual SRS (executive + technical).
func (c *Client) GenerateSRS(ctx context.Context, notes string) (*SRS, error) {
	if !c.Enabled() {
		return mockSRS(notes), nil
	}

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"client_content": {Type: genai.TypeString},
			"dev_content":    {Type: genai.TypeString},
		},
		Required: []string{"client_content", "dev_content"},
	}

	prompt := `Actúa como analista de software. A partir de las notas del cliente, genera dos documentos SRS en español:
1) client_content: "Vista Ejecutiva" sin tecnicismos, con reglas de negocio, casos de uso y alcance.
2) dev_content: "Vista Técnica" con arquitectura propuesta, modelo de datos, endpoints REST principales, integraciones y criterios de aceptación.
Devuelve SOLO el JSON solicitado.

NOTAS DEL CLIENTE:
` + notes

	resp, err := c.api.Models.GenerateContent(ctx, c.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      float32Ptr(0.3),
	})
	if err != nil {
		return nil, fmt.Errorf("gemini generate srs: %w", err)
	}

	var out SRS
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return nil, fmt.Errorf("parse srs json: %w", err)
	}
	if strings.TrimSpace(out.ClientContent) == "" || strings.TrimSpace(out.DevContent) == "" {
		return nil, fmt.Errorf("gemini returned empty srs")
	}
	return &out, nil
}

// ProposeTasks reads an SRS and proposes backlog tasks.
func (c *Client) ProposeTasks(ctx context.Context, srsContent string) ([]ProposedTask, error) {
	if !c.Enabled() {
		return mockTasks(srsContent), nil
	}

	schema := &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"title":       {Type: genai.TypeString},
				"description": {Type: genai.TypeString},
			},
			Required: []string{"title", "description"},
		},
	}

	prompt := `Extrae de este SRS una lista de tareas de desarrollo concretas para un tablero Kanban.
Cada tarea debe tener un título corto (máx 80 caracteres) y una descripción accionable.
Devuelve SOLO el arreglo JSON solicitado.

SRS:
` + srsContent

	resp, err := c.api.Models.GenerateContent(ctx, c.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      float32Ptr(0.2),
	})
	if err != nil {
		return nil, fmt.Errorf("gemini propose tasks: %w", err)
	}

	var out []ProposedTask
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return nil, fmt.Errorf("parse tasks json: %w", err)
	}
	return out, nil
}

// UpdateSRS merges the current SRS with new notes, returning the full updated content.
func (c *Client) UpdateSRS(ctx context.Context, currentClient, currentDev, notes string) (*SRS, error) {
	notes = strings.TrimSpace(notes)
	if strings.TrimSpace(currentClient) == "" && strings.TrimSpace(currentDev) == "" {
		return c.GenerateSRS(ctx, notes)
	}
	if !c.Enabled() {
		return mockUpdateSRS(currentClient, currentDev, notes), nil
	}

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"client_content": {Type: genai.TypeString},
			"dev_content":    {Type: genai.TypeString},
		},
		Required: []string{"client_content", "dev_content"},
	}

	prompt := `Actúa como analista de software. Actualiza el SRS existente incorporando las nuevas notas.
Reglas:
- Conserva toda la información vigente que siga siendo válida.
- Integra, ajusta o amplía lo necesario según las nuevas notas.
- Devuelve el documento COMPLETO actualizado, no solo el fragmento nuevo.
- client_content: vista ejecutiva (reglas de negocio, casos de uso, alcance) sin tecnicismos.
- dev_content: vista técnica (arquitectura, modelo de datos, endpoints, criterios de aceptación).
Devuelve SOLO el JSON solicitado.

SRS EJECUTIVO ACTUAL:
` + currentClient + `

SRS TÉCNICO ACTUAL:
` + currentDev + `

NUEVAS NOTAS / PROPUESTAS:
` + notes

	resp, err := c.api.Models.GenerateContent(ctx, c.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      float32Ptr(0.3),
	})
	if err != nil {
		return nil, fmt.Errorf("gemini update srs: %w", err)
	}

	var out SRS
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return nil, fmt.Errorf("parse srs json: %w", err)
	}
	if strings.TrimSpace(out.ClientContent) == "" || strings.TrimSpace(out.DevContent) == "" {
		return nil, fmt.Errorf("gemini returned empty srs")
	}
	return &out, nil
}

// ValidateBacklog compares an SRS version with existing task titles and proposes missing work.
func (c *Client) ValidateBacklog(ctx context.Context, srsContent string, existing []string) ([]ProposedTask, error) {
	if !c.Enabled() {
		return mockValidation(existing), nil
	}

	schema := &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"title":       {Type: genai.TypeString},
				"description": {Type: genai.TypeString},
			},
			Required: []string{"title", "description"},
		},
	}

	prompt := `Compara el siguiente SRS con las tareas actuales del tablero Kanban.
Propón únicamente tareas faltantes o que requieran ajuste para cumplir el SRS.
No repitas tareas existentes ni propongas tareas triviales.
Cada tarea: título corto (máx 80 caracteres) y descripción accionable.
Devuelve SOLO el arreglo JSON solicitado.

SRS:
` + srsContent + `

TAREAS ACTUALES:
- ` + strings.Join(existing, "\n- ")

	resp, err := c.api.Models.GenerateContent(ctx, c.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      float32Ptr(0.2),
	})
	if err != nil {
		return nil, fmt.Errorf("gemini validate backlog: %w", err)
	}

	var out []ProposedTask
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return nil, fmt.Errorf("parse validation json: %w", err)
	}
	return out, nil
}

func mockUpdateSRS(currentClient, currentDev, notes string) *SRS {
	if notes == "" {
		notes = "Sin nuevas notas."
	}
	client := strings.TrimRight(currentClient, "\n") + "\n\n## Actualización propuesta\n" + notes
	dev := strings.TrimRight(currentDev, "\n") + "\n\n## Cambios técnicos propuestos\n- Incorporar al alcance: " + notes
	return &SRS{ClientContent: client, DevContent: dev}
}

func mockValidation(existing []string) []ProposedTask {
	candidates := []ProposedTask{
		{Title: "Revisar criterios de aceptación del SRS", Description: "Validar que el backlog cubra los criterios de aceptación de la versión asignada."},
		{Title: "Alinear backlog con objetivos del SRS", Description: "Detectar brechas entre las tareas actuales y el alcance documentado."},
		{Title: "Actualizar tareas afectadas por cambios del SRS", Description: "Refinar las tareas existentes que cambian con la nueva versión."},
	}
	seen := map[string]bool{}
	for _, t := range existing {
		seen[strings.ToLower(strings.TrimSpace(t))] = true
	}
	out := make([]ProposedTask, 0, len(candidates))
	for _, t := range candidates {
		if !seen[strings.ToLower(t.Title)] {
			out = append(out, t)
		}
	}
	return out
}

func mockSRS(notes string) *SRS {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		notes = "Proyecto sin notas iniciales."
	}
	client := "## Objetivo\n" + notes + "\n\n## Reglas de negocio\n- El sistema debe cubrir el alcance descrito en las notas.\n- Toda entrega requiere aprobación del cliente.\n\n## Casos de uso\n1. El cliente revisa avances.\n2. El cliente reporta incidencias.\n3. El administrador gestiona el proyecto."
	dev := "## Arquitectura\nAplicación web monolítica Go (Chi) con SQLite, plantillas HTMX y despliegue Docker.\n\n## Modelo de datos\nUsers, Projects, SRS, Tasks, Deliverables, Tickets, Resources, Billings.\n\n## Endpoints principales\n- POST /auth/login\n- GET /admin/projects/{id}/kanban\n- POST /client/projects/{id}/deliverables/{did}/tickets\n\n## Criterios de aceptación\n- Flujos protegidos por rol.\n- Persistencia de estado.\n- Notificaciones al administrador."
	return &SRS{ClientContent: client, DevContent: dev}
}

func mockTasks(srsContent string) []ProposedTask {
	base := []ProposedTask{
		{Title: "Configurar proyecto base y autenticación", Description: "Estructura Go, rutas protegidas y login por roles."},
		{Title: "Modelar base de datos y migraciones", Description: "Definir tablas principales y migraciones versionadas."},
		{Title: "Implementar tablero Kanban", Description: "Columnas backlog, en curso, testeo y terminado con drag & drop."},
		{Title: "Portal de entregables del cliente", Description: "Listado de entregables, enlace staging y tickets."},
		{Title: "Notificaciones WhatsApp al administrador", Description: "Alertas al recibir tickets y recursos."},
	}
	if len(strings.Fields(srsContent)) < 10 {
		return base[:3]
	}
	return base
}

func float32Ptr(f float32) *float32 { return &f }
