package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Service struct {
	apiKey string
	from   string
	client *http.Client
}

func New(apiKey, from string) *Service {
	return &Service{apiKey: apiKey, from: from, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *Service) Enabled() bool { return s.apiKey != "" }

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendPasswordReset sends a reset link. Without an API key it logs the link (dev mode).
func (s *Service) SendPasswordReset(ctx context.Context, to, name, link string) error {
	subject := "Restablece tu contraseña - GioDevPortal"
	html := fmt.Sprintf(`
<div style="font-family:Arial,sans-serif;max-width:600px;margin:auto;padding:24px">
  <h2 style="color:#0f172a">Hola %s</h2>
  <p>Recibimos una solicitud para restablecer tu contraseña en <strong>GioDevPortal</strong>.</p>
  <p><a href="%s" style="display:inline-block;padding:12px 20px;background:#2563eb;color:#fff;border-radius:8px;text-decoration:none">Restablecer contraseña</a></p>
  <p style="color:#64748b;font-size:13px">El enlace expira en 1 hora. Si no solicitaste esto, ignora este correo.</p>
  <p style="color:#94a3b8;font-size:12px;word-break:break-all">%s</p>
</div>`, name, link, link)

	if !s.Enabled() {
		log.Printf("[email:dev] password reset for %s -> %s", to, link)
		return nil
	}

	body, _ := json.Marshal(resendRequest{From: s.from, To: []string{to}, Subject: subject, HTML: html})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *Service) SendWelcome(ctx context.Context, to, name, tempPassword string, loginURL string) error {
	subject := "Tu acceso a GioDevPortal"
	html := fmt.Sprintf(`
<div style="font-family:Arial,sans-serif;max-width:600px;margin:auto;padding:24px">
  <h2 style="color:#0f172a">Bienvenido, %s</h2>
  <p>Se creó tu cuenta de cliente en <strong>GioDevPortal</strong>.</p>
  <p>Correo: <strong>%s</strong><br/>Contraseña temporal: <code style="background:#f1f5f9;padding:4px 8px;border-radius:6px">%s</code></p>
  <p><a href="%s/login" style="display:inline-block;padding:12px 20px;background:#2563eb;color:#fff;border-radius:8px;text-decoration:none">Ingresar</a></p>
  <p style="color:#64748b;font-size:13px">Por seguridad se te pedirá cambiar la contraseña al ingresar.</p>
</div>`, name, to, tempPassword, loginURL)

	if !s.Enabled() {
		log.Printf("[email:dev] welcome for %s temp password=%s", to, tempPassword)
		return nil
	}

	body, _ := json.Marshal(resendRequest{From: s.from, To: []string{to}, Subject: subject, HTML: html})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
