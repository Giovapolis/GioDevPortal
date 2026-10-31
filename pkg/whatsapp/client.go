package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	phoneNumberID    string
	accessToken      string
	adminNumber      string
	ticketTemplate   string
	resourceTemplate string
	paymentTemplate  string
	srsTemplate      string
	http             *http.Client
}

func New(phoneNumberID, accessToken, adminNumber, ticketTemplate, resourceTemplate, paymentTemplate, srsTemplate string) *Client {
	return &Client{
		phoneNumberID:    phoneNumberID,
		accessToken:      accessToken,
		adminNumber:      adminNumber,
		ticketTemplate:   ticketTemplate,
		resourceTemplate: resourceTemplate,
		paymentTemplate:  paymentTemplate,
		srsTemplate:      srsTemplate,
		http:             &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c.phoneNumberID != "" && c.accessToken != "" && c.adminNumber != ""
}

type templateParameter struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type templateComponent struct {
	Type       string              `json:"type"`
	Parameters []templateParameter `json:"parameters"`
}

type templatePayload struct {
	Name       string              `json:"name"`
	Language   map[string]string   `json:"language"`
	Components []templateComponent `json:"components"`
}

type messagePayload struct {
	MessagingProduct string            `json:"messaging_product"`
	To               string            `json:"to"`
	Type             string            `json:"type"`
	Template         *templatePayload  `json:"template,omitempty"`
	Text             map[string]string `json:"text,omitempty"`
}

func (c *Client) send(ctx context.Context, payload messagePayload) error {
	payload.MessagingProduct = "whatsapp"
	payload.To = c.adminNumber

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", c.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("whatsapp api %d: %s", resp.StatusCode, buf.String())
	}
	return nil
}

// SendTicketAlert notifies the admin that the client created a ticket.
func (c *Client) SendTicketAlert(ctx context.Context, project, client, message string) error {
	if !c.Enabled() {
		log.Printf("[whatsapp:dev] ticket alert project=%q client=%q msg=%q", project, client, message)
		return nil
	}
	params := []string{project, client, truncate(message, 120)}
	return c.send(ctx, messagePayload{
		Type: "template",
		Template: &templatePayload{
			Name:     c.ticketTemplate,
			Language: map[string]string{"code": "es"},
			Components: []templateComponent{
				{Type: "body", Parameters: toParams(params)},
			},
		},
	})
}

// SendResourceAlert notifies the admin that the client uploaded a resource.
func (c *Client) SendResourceAlert(ctx context.Context, project, client, filename string) error {
	if !c.Enabled() {
		log.Printf("[whatsapp:dev] resource alert project=%q client=%q file=%q", project, client, filename)
		return nil
	}
	params := []string{project, client, truncate(filename, 120)}
	return c.send(ctx, messagePayload{
		Type: "template",
		Template: &templatePayload{
			Name:     c.resourceTemplate,
			Language: map[string]string{"code": "es"},
			Components: []templateComponent{
				{Type: "body", Parameters: toParams(params)},
			},
		},
	})
}

// SendPaymentAlert notifies the admin that a client reported a payment.
func (c *Client) SendPaymentAlert(ctx context.Context, project, client, amount string) error {
	if !c.Enabled() {
		log.Printf("[whatsapp:dev] payment alert project=%q client=%q amount=%q", project, client, amount)
		return nil
	}
	return c.send(ctx, messagePayload{
		Type: "template",
		Template: &templatePayload{
			Name:     c.paymentTemplate,
			Language: map[string]string{"code": "es"},
			Components: []templateComponent{
				{Type: "body", Parameters: toParams([]string{project, client, amount})},
			},
		},
	})
}

// SendSRSRequestAlert notifies the admin that a client requested an SRS update.
func (c *Client) SendSRSRequestAlert(ctx context.Context, project, client, summary string) error {
	if !c.Enabled() {
		log.Printf("[whatsapp:dev] srs request alert project=%q client=%q summary=%q", project, client, summary)
		return nil
	}
	return c.send(ctx, messagePayload{
		Type: "template",
		Template: &templatePayload{
			Name:     c.srsTemplate,
			Language: map[string]string{"code": "es"},
			Components: []templateComponent{
				{Type: "body", Parameters: toParams([]string{project, client, truncate(summary, 120)})},
			},
		},
	})
}

func toParams(values []string) []templateParameter {
	out := make([]templateParameter, 0, len(values))
	for _, v := range values {
		out = append(out, templateParameter{Type: "text", Text: v})
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
