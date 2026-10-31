package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"giodevportal/internal/config"
	"giodevportal/internal/db"
	"giodevportal/internal/handler"
	repo "giodevportal/internal/repository/sqlc"
	"giodevportal/internal/service/auth"
	"giodevportal/internal/service/email"
	render "giodevportal/internal/template"
	"giodevportal/pkg/gemini"
	"giodevportal/pkg/storage"
	"giodevportal/pkg/whatsapp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	if err := db.Migrate(sqlDB); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	q := repo.New(sqlDB)
	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	emailSvc := email.New(cfg.ResendAPIKey, cfg.EmailFrom)
	wa := whatsapp.New(cfg.WhatsAppPhoneID, cfg.WhatsAppToken, cfg.AdminWhatsAppNumber, cfg.WhatsAppTicketTemplate, cfg.WhatsAppResourceTemplate, cfg.WhatsAppPaymentTemplate, cfg.WhatsAppSRSTemplate)

	gem, err := gemini.New(context.Background(), cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("gemini: %v", err)
	}

	var store storage.Store
	if cfg.R2Enabled() {
		r2, err := storage.NewR2(context.Background(), storage.R2Config{
			AccountID: cfg.R2AccountID,
			AccessKey: cfg.R2AccessKey,
			SecretKey: cfg.R2SecretKey,
			Bucket:    cfg.R2Bucket,
		})
		if err != nil {
			log.Fatalf("r2: %v", err)
		}
		store = r2
		log.Printf("storage: Cloudflare R2 (%s)", cfg.R2Bucket)
	} else {
		local, err := storage.NewLocal(filepath.Join(cfg.DataDir, "uploads"))
		if err != nil {
			log.Fatalf("local storage: %v", err)
		}
		store = local
		log.Printf("storage: local disk (%s)", filepath.Join(cfg.DataDir, "uploads"))
	}

	rnd, err := render.New("web/templates")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	app := handler.NewApp(cfg, q, sqlDB, authSvc, emailSvc, gem, wa, store, rnd)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := app.Seed(ctx); err != nil {
		log.Fatalf("seed: %v", err)
	}

	if !cfg.GeminiEnabled() {
		log.Printf("gemini: disabled (using deterministic mock). Set GEMINI_API_KEY to enable.")
	}
	if !cfg.WhatsAppEnabled() {
		log.Printf("whatsapp: disabled (logging notifications). Set WHATSAPP_* vars to enable.")
	}
	if !emailSvc.Enabled() {
		log.Printf("email: disabled (logging). Set RESEND_API_KEY to enable.")
	}

	srv := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           app.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("GioDevPortal listening on %s (env=%s)", cfg.ServerAddr, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Printf("bye")
}
