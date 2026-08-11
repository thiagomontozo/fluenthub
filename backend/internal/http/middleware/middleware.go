package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/thiagomontozo/fluenthub/backend/internal/auth"
	"github.com/thiagomontozo/fluenthub/backend/internal/users"
	"net/http"
	"slices"
)

type key string

const (
	userKey      key = "user"
	requestIDKey key = "request-id"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 12)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}
func CORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Request-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func Authenticate(store *users.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.CookieName)
		if err != nil {
			writeUnauthorized(w, RequestID(r.Context()))
			return
		}
		u, err := store.ByToken(r.Context(), cookie.Value)
		if err != nil {
			writeUnauthorized(w, RequestID(r.Context()))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func Require(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := User(r.Context())
		if !ok || !slices.Contains(u.Permissions, permission) {
			http.Error(w, `{"error":{"code":"FORBIDDEN","message":"Permission denied","requestId":"`+RequestID(r.Context())+`"}}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func User(ctx context.Context) (users.SessionUser, bool) {
	u, ok := ctx.Value(userKey).(users.SessionUser)
	return u, ok
}
func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey).(string); return id }
func writeUnauthorized(w http.ResponseWriter, id string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHENTICATED","message":"Authentication required","requestId":"` + id + `"}}`))
}
