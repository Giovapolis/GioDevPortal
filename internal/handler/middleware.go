package handler

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"

	"giodevportal/internal/domain"
	"giodevportal/internal/service/auth"
)

func chiURLParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

// SecurityHeaders sets conservative security headers for all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// LimitBody caps request bodies (uploads) before form parsing.
func LimitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *App) secureCookies() bool { return a.Cfg.Env == "production" }

func (a *App) setAccessCookie(w http.ResponseWriter, token string) {
	setCookie(w, accessCookie, token, int(a.Auth.AccessTTL().Seconds()), a.secureCookies(), "/")
}

func (a *App) setRefreshCookie(w http.ResponseWriter, token string) {
	setCookie(w, refreshCookie, token, int(a.Auth.RefreshTTL().Seconds()), a.secureCookies(), "/")
}

func (a *App) clearAuthCookies(w http.ResponseWriter) {
	setCookie(w, accessCookie, "", -1, a.secureCookies(), "/")
	setCookie(w, refreshCookie, "", -1, a.secureCookies(), "/")
}

// Authenticate resolves the user from the access token, transparently refreshing it.
func (a *App) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(accessCookie); err == nil && c.Value != "" {
			if claims, err := a.Auth.ParseAccessToken(c.Value); err == nil {
				if u, err := a.Q.GetUserByID(r.Context(), claims.UserID); err == nil && u.IsActive == 1 {
					next.ServeHTTP(w, r.WithContext(withUser(r.Context(), &u)))
					return
				}
			}
		}

		if c, err := r.Cookie(refreshCookie); err == nil && c.Value != "" {
			row, err := a.Q.GetRefreshTokenByHash(r.Context(), auth.HashToken(c.Value))
			if err == nil && row.Revoked == 0 {
				exp, ok := domain.ParseTime(row.ExpiresAt)
				if ok && time.Now().Before(exp) {
					if u, err := a.Q.GetUserByID(r.Context(), row.UserID); err == nil && u.IsActive == 1 {
						if at, err := a.Auth.IssueAccessToken(u.ID, u.Role); err == nil {
							a.setAccessCookie(w, at)
							next.ServeHTTP(w, r.WithContext(withUser(r.Context(), &u)))
							return
						}
					}
				}
			}
		}

		a.clearAuthCookies(w)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (a *App) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r.Context())
			if u == nil || u.Role != role {
				if r.Header.Get("HX-Request") == "true" {
					http.Error(w, "No autorizado", http.StatusForbidden)
					return
				}
				http.Error(w, "No autorizado", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF implements a double-submit cookie check and exposes the token to templates.
func (a *App) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie(csrfCookie); err == nil {
			token = c.Value
		}
		if token == "" {
			t, err := auth.GenerateToken()
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			token = t
			setCookie(w, csrfCookie, token, 7*24*3600, a.secureCookies(), "/")
		}

		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			sent := r.Header.Get("X-CSRF-Token")
			if sent == "" {
				sent = r.FormValue("csrf_token")
			}
			if sent == "" || sent != token {
				log.Printf("[csrf] rejected %s %s (sent=%q)", r.Method, r.URL.Path, sent)
				http.Error(w, "Token CSRF inválido o ausente", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(withCSRF(r.Context(), token)))
	})
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    rate.Limit
	burst    int
	done     chan struct{}
}

func newIPLimiter(limit rate.Limit, burst int) *ipLimiter {
	l := &ipLimiter{
		visitors: map[string]*visitor{},
		limit:    limit,
		burst:    burst,
		done:     make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func (l *ipLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			for ip, v := range l.visitors {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(l.visitors, ip)
				}
			}
			l.mu.Unlock()
		case <-l.done:
			return
		}
	}
}

func (l *ipLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

// RateLimit returns middleware limiting requests per client IP.
func (a *App) RateLimit(perMinute int, burst int) func(http.Handler) http.Handler {
	limiter := newIPLimiter(rate.Limit(float64(perMinute)/60.0), burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				ip = host
			}
			if !limiter.allow(ip) {
				w.Header().Set("Retry-After", "30")
				if r.Header.Get("HX-Request") == "true" {
					w.Header().Set("HX-Retarget", "body")
				}
				http.Error(w, "Demasiadas solicitudes. Intenta de nuevo en un momento.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
