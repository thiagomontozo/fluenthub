package router

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/fluenthub/backend/internal/auth"
	"github.com/thiagomontozo/fluenthub/backend/internal/certificates"
	httpmw "github.com/thiagomontozo/fluenthub/backend/internal/http/middleware"
	"github.com/thiagomontozo/fluenthub/backend/internal/notifications"
	"github.com/thiagomontozo/fluenthub/backend/internal/storage"
	"github.com/thiagomontozo/fluenthub/backend/internal/users"
	"net/http"
	"strings"
	"time"
)

type Dependencies struct {
	DB                     *pgxpool.Pool
	Storage                storage.ObjectStorage
	Users                  *users.Store
	Certificates           *certificates.Service
	Hub                    *notifications.Hub
	WebOrigin, Environment string
}

func New(d Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "NOT_READY", "Database unavailable")
			return
		}
		exists, err := d.Storage.Exists(ctx, "readiness/probe")
		_ = exists
		if err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "NOT_READY", "Storage unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/public/branding", func(w http.ResponseWriter, r *http.Request) {
		var title, name, primary, secondary, accent, welcome string
		err := d.DB.QueryRow(r.Context(), `SELECT b.system_title,b.school_display_name,b.primary_color,b.secondary_color,b.accent_color,b.welcome_text FROM school_branding b JOIN schools s ON s.id=b.school_id WHERE s.active=true ORDER BY s.created_at LIMIT 1`).Scan(&title, &name, &primary, &secondary, &accent, &welcome)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"systemTitle": "FluentHub", "schoolDisplayName": "Language School", "primaryColor": "#4f46e5", "secondaryColor": "#0f172a", "accentColor": "#f59e0b", "welcomeText": "Welcome to your learning journey."})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"systemTitle": title, "schoolDisplayName": name, "primaryColor": primary, "secondaryColor": secondary, "accentColor": accent, "welcomeText": welcome})
	})
	mux.HandleFunc("GET /api/v1/public/certificates/verify/{code}", func(w http.ResponseWriter, r *http.Request) {
		certificate, err := d.Certificates.Verify(r.Context(), r.PathValue("code"))
		if err != nil {
			writeError(w, r, http.StatusNotFound, "CERTIFICATE_NOT_FOUND", "Certificate not found")
			return
		}
		writeJSON(w, http.StatusOK, certificate)
	})
	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Email, Password string }
		if decodeJSON(w, r, &input) != nil {
			return
		}
		u, token, err := d.Users.Authenticate(r.Context(), strings.TrimSpace(input.Email), input.Password)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is invalid")
			return
		}
		auth.SetCookie(w, token, d.Environment == "production", time.Now().Add(7*24*time.Hour))
		writeJSON(w, http.StatusOK, u)
	})
	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		u, _ := httpmw.User(r.Context())
		writeJSON(w, http.StatusOK, u)
	})
	protected.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.CookieName); err == nil {
			_ = d.Users.Revoke(r.Context(), c.Value)
		}
		auth.ClearCookie(w, d.Environment == "production")
		w.WriteHeader(http.StatusNoContent)
	})
	protected.HandleFunc("POST /api/v1/auth/change-password", func(w http.ResponseWriter, r *http.Request) {
		var input struct{ CurrentPassword, NewPassword string }
		if decodeJSON(w, r, &input) != nil {
			return
		}
		u, _ := httpmw.User(r.Context())
		if err := d.Users.ChangePassword(r.Context(), u.ID, input.CurrentPassword, input.NewPassword); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "PASSWORD_CHANGE_FAILED", err.Error())
			return
		}
		auth.ClearCookie(w, d.Environment == "production")
		w.WriteHeader(http.StatusNoContent)
	})
	protected.HandleFunc("GET /api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		u, _ := httpmw.User(r.Context())
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, r, http.StatusNotImplemented, "SSE_UNSUPPORTED", "Streaming unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		events, unsubscribe := d.Hub.Subscribe(u.ID)
		defer unsubscribe()
		_, _ = w.Write([]byte("event: connected\ndata: {}\n\n"))
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				_, _ = w.Write(event)
				flusher.Flush()
			}
		}
	})
	mux.Handle("/api/v1/", httpmw.Authenticate(d.Users, protected))
	return httpmw.RequestID(httpmw.CORS(d.WebOrigin, mux))
}
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return err
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "requestId": httpmw.RequestID(r.Context())}})
}
