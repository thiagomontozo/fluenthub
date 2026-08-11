package router

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/fluenthub/backend/internal/assessment"
	"github.com/thiagomontozo/fluenthub/backend/internal/auth"
	"github.com/thiagomontozo/fluenthub/backend/internal/billing"
	"github.com/thiagomontozo/fluenthub/backend/internal/certificates"
	httpmw "github.com/thiagomontozo/fluenthub/backend/internal/http/middleware"
	"github.com/thiagomontozo/fluenthub/backend/internal/notifications"
	"github.com/thiagomontozo/fluenthub/backend/internal/records"
	"github.com/thiagomontozo/fluenthub/backend/internal/setup"
	"github.com/thiagomontozo/fluenthub/backend/internal/storage"
	"github.com/thiagomontozo/fluenthub/backend/internal/support"
	"github.com/thiagomontozo/fluenthub/backend/internal/users"
)

type Dependencies struct {
	DB                     *pgxpool.Pool
	Storage                storage.ObjectStorage
	Users                  *users.Store
	Certificates           *certificates.Service
	Hub                    *notifications.Hub
	Billing                *billing.Service
	Notifications          *notifications.Service
	Assessment             *assessment.Service
	Support                *support.Service
	Records                *records.Service
	Setup                  *setup.Service
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
	mux.HandleFunc("GET /api/v1/setup/status", func(w http.ResponseWriter, r *http.Request) {
		configured, err := d.Setup.Status(r.Context())
		if err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "SETUP_STATUS_FAILED", "Setup status is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"configured": configured})
	})
	mux.HandleFunc("POST /api/v1/setup/complete", func(w http.ResponseWriter, r *http.Request) {
		var input setup.Input
		if decodeJSON(w, r, &input) != nil {
			return
		}
		schoolID, err := d.Setup.Complete(r.Context(), input)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "SETUP_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"schoolId": schoolID})
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
	protected.Handle("GET /api/v1/users", httpmw.Require("users.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		role := r.URL.Query().Get("role")
		if role != "" && !slices.Contains([]string{"administrator", "teacher", "student", "operator"}, role) {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid role filter")
			return
		}
		if !slices.Contains(user.Permissions, "school.manage") {
			role = "student"
		}
		items, err := d.Users.List(r.Context(), user.SchoolID, role, parseInt(r.URL.Query().Get("limit"), 25), parseInt(r.URL.Query().Get("offset"), 0))
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "USERS_LIST_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})))
	protected.Handle("POST /api/v1/users", httpmw.Require("users.manage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _ := httpmw.User(r.Context())
		var input users.CreateInput
		if decodeJSON(w, r, &input) != nil {
			return
		}
		if input.RoleCode == "administrator" && !slices.Contains(actor.Permissions, "admin.create") {
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Administrator creation requires admin.create")
			return
		}
		created, err := d.Users.Create(r.Context(), actor.SchoolID, actor.ID, input)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "USER_CREATE_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})))
	protected.Handle("GET /api/v1/teacher/students", httpmw.Require("classes.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		items, err := d.Users.ListTeacherStudents(r.Context(), user.SchoolID, user.ID, parseInt(r.URL.Query().Get("limit"), 25), parseInt(r.URL.Query().Get("offset"), 0))
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "STUDENTS_LIST_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})))
	protected.Handle("POST /api/v1/users/{id}/disable", httpmw.Require("users.manage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _ := httpmw.User(r.Context())
		if err := d.Users.Disable(r.Context(), actor.SchoolID, actor.ID, r.PathValue("id")); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "USER_DISABLE_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.Handle("POST /api/v1/users/{id}/reactivate", httpmw.Require("users.manage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _ := httpmw.User(r.Context())
		if err := d.Users.Reactivate(r.Context(), actor.SchoolID, actor.ID, r.PathValue("id")); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "USER_REACTIVATE_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
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
	resources := []struct {
		Path, Kind, ReadPermission, WritePermission string
	}{
		{"units", "units", "units.read", "units.manage"},
		{"courses", "courses", "courses.read", "courses.manage"},
		{"classes", "classes", "classes.read", "classes.manage"},
		{"enrollments", "enrollments", "classes.read", "classes.manage"},
		{"lessons", "lessons", "lessons.read", "lessons.manage"},
		{"exercises", "exercises", "exercises.read", "exercises.manage"},
		{"exams", "exams", "exams.read", "exams.manage"},
		{"billing", "invoices", "billing.read", "billing.manage"},
		{"support", "support", "support.open", "support.open"},
		{"notifications", "notifications", "notifications.read", "notifications.send"},
	}
	for _, resource := range resources {
		resource := resource
		protected.Handle("GET /api/v1/"+resource.Path, httpmw.Require(resource.ReadPermission, listHandler(d.Records, resource.Kind)))
		if resource.Kind != "invoices" && resource.Kind != "notifications" {
			protected.Handle("POST /api/v1/"+resource.Path, httpmw.Require(resource.WritePermission, createHandler(d.Records, resource.Kind)))
		}
	}
	transitions := []struct{ Path, Kind, Permission string }{
		{"classes", "classes", "classes.manage"},
		{"exercises", "exercises", "exercises.manage"},
		{"exams", "exams", "exams.manage"},
		{"support", "support", "support.handle"},
	}
	for _, resource := range transitions {
		resource := resource
		protected.Handle("PATCH /api/v1/"+resource.Path+"/{id}/status", httpmw.Require(resource.Permission, transitionHandler(d.Records, resource.Kind)))
	}
	protected.Handle("POST /api/v1/billing", httpmw.Require("billing.manage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input billing.CreateInvoiceInput
		if decodeJSON(w, r, &input) != nil {
			return
		}
		invoice, err := d.Billing.Create(r.Context(), user.SchoolID, user.ID, input)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "INVOICE_CREATE_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, invoice)
	})))
	protected.Handle("POST /api/v1/notifications", httpmw.Require("notifications.send", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input notifications.SendInput
		if decodeJSON(w, r, &input) != nil {
			return
		}
		id, err := d.Notifications.Send(r.Context(), user.SchoolID, user.ID, input)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "NOTIFICATION_SEND_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	})))
	protected.HandleFunc("PATCH /api/v1/notifications/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		if err := d.Notifications.MarkRead(r.Context(), user.SchoolID, user.ID, r.PathValue("id")); err != nil {
			writeError(w, r, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	protected.HandleFunc("POST /api/v1/notifications/read-all", func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		if err := d.Notifications.MarkAllRead(r.Context(), user.SchoolID, user.ID); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "NOTIFICATIONS_UPDATE_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	protected.Handle("POST /api/v1/exercises/{id}/attempts", httpmw.Require("exercises.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		id, err := d.Assessment.StartExercise(r.Context(), user.SchoolID, user.ID, r.PathValue("id"))
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "ATTEMPT_START_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	})))
	protected.Handle("PUT /api/v1/exercise-attempts/{id}/answers/{questionId}", httpmw.Require("exercises.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input assessment.AnswerInput
		if decodeJSON(w, r, &input) != nil {
			return
		}
		input.QuestionID = r.PathValue("questionId")
		if err := d.Assessment.SaveExerciseAnswer(r.Context(), user.SchoolID, user.ID, r.PathValue("id"), input); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "ANSWER_SAVE_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.Handle("POST /api/v1/exercise-attempts/{id}/submit", httpmw.Require("exercises.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		if err := d.Assessment.SubmitExercise(r.Context(), user.SchoolID, user.ID, r.PathValue("id")); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "ATTEMPT_SUBMIT_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.Handle("POST /api/v1/exams/{id}/attempts", httpmw.Require("exams.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		id, expires, err := d.Assessment.StartExam(r.Context(), user.SchoolID, user.ID, r.PathValue("id"))
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "ATTEMPT_START_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "expiresAt": expires})
	})))
	protected.Handle("PUT /api/v1/exam-attempts/{id}/answers/{questionId}", httpmw.Require("exams.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input assessment.AnswerInput
		if decodeJSON(w, r, &input) != nil {
			return
		}
		input.QuestionID = r.PathValue("questionId")
		if err := d.Assessment.SaveExamAnswer(r.Context(), user.SchoolID, user.ID, r.PathValue("id"), input); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "ANSWER_SAVE_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.Handle("POST /api/v1/exam-attempts/{id}/submit", httpmw.Require("exams.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		if err := d.Assessment.SubmitExam(r.Context(), user.SchoolID, user.ID, r.PathValue("id")); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "ATTEMPT_SUBMIT_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.Handle("PUT /api/v1/grading/exercises/{attemptId}", httpmw.Require("exercises.grade", gradeHandler(d.Assessment, "exercise")))
	protected.Handle("PUT /api/v1/grading/exams/{attemptId}", httpmw.Require("exams.grade", gradeHandler(d.Assessment, "exam")))
	protected.Handle("POST /api/v1/support/{id}/assign", httpmw.Require("support.assign", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input struct{ AssignedToUserID string }
		if decodeJSON(w, r, &input) != nil {
			return
		}
		if err := d.Support.Assign(r.Context(), user.SchoolID, user.ID, r.PathValue("id"), input.AssignedToUserID); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "TICKET_ASSIGN_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.Handle("POST /api/v1/support/{id}/messages", httpmw.Require("support.open", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input struct {
			Message  string
			Internal bool
		}
		if decodeJSON(w, r, &input) != nil {
			return
		}
		staff := slices.Contains(user.Permissions, "support.handle") || slices.Contains(user.Permissions, "support.assign")
		id, err := d.Support.AddMessage(r.Context(), user.SchoolID, user.ID, r.PathValue("id"), input.Message, input.Internal, staff)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "TICKET_MESSAGE_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	})))
	mux.Handle("/api/v1/", httpmw.Authenticate(d.Users, protected))
	return httpmw.RequestID(httpmw.CORS(d.WebOrigin, mux))
}

func listHandler(service *records.Service, resource string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		limit := parseInt(r.URL.Query().Get("limit"), 25)
		offset := parseInt(r.URL.Query().Get("offset"), 0)
		unrestricted := slices.Contains(user.Permissions, "school.manage")
		if resource == "invoices" && slices.Contains(user.Permissions, "billing.manage") {
			unrestricted = true
		}
		if resource == "support" && slices.Contains(user.Permissions, "support.assign") {
			unrestricted = true
		}
		if resource == "notifications" {
			unrestricted = false
		}
		items, err := service.List(r.Context(), user.SchoolID, user.ID, resource, unrestricted, limit, offset)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "LIST_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
	})
}

func createHandler(service *records.Service, resource string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input json.RawMessage
		if decodeJSON(w, r, &input) != nil {
			return
		}
		id, err := service.Create(r.Context(), user.SchoolID, user.ID, resource, slices.Contains(user.Permissions, "school.manage"), input)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "CREATE_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	})
}

func transitionHandler(service *records.Service, resource string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input struct{ Status string }
		if decodeJSON(w, r, &input) != nil {
			return
		}
		unrestricted := slices.Contains(user.Permissions, "school.manage") || (resource == "support" && slices.Contains(user.Permissions, "support.assign"))
		if err := service.Transition(r.Context(), user.SchoolID, user.ID, resource, r.PathValue("id"), input.Status, unrestricted); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "TRANSITION_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return parsed
}

func gradeHandler(service *assessment.Service, kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := httpmw.User(r.Context())
		var input assessment.GradeInput
		if decodeJSON(w, r, &input) != nil {
			return
		}
		var err error
		if kind == "exam" {
			err = service.GradeExam(r.Context(), user.SchoolID, user.ID, r.PathValue("attemptId"), input)
		} else {
			err = service.GradeExercise(r.Context(), user.SchoolID, user.ID, r.PathValue("attemptId"), input)
		}
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "GRADE_SAVE_FAILED", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
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
