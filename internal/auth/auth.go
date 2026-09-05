package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"docker-panel/internal/database"
	"docker-panel/internal/models"
	"docker-panel/internal/rbac"
)

type contextKey string

const UserContextKey = contextKey("auth_user")

type Authenticator struct {
	db           *database.DB
	allowedEmail []string
	adminEmail   string
	devMode      bool
}

func NewAuthenticator(db *database.DB, allowedEmails []string, adminEmail string, devMode bool) *Authenticator {
	cleanEmails := make([]string, 0, len(allowedEmails))
	for _, e := range allowedEmails {
		trimmed := strings.ToLower(strings.TrimSpace(e))
		if trimmed != "" {
			cleanEmails = append(cleanEmails, trimmed)
		}
	}

	return &Authenticator{
		db:           db,
		allowedEmail: cleanEmails,
		adminEmail:   strings.ToLower(strings.TrimSpace(adminEmail)),
		devMode:      devMode,
	}
}

func (a *Authenticator) Authenticate(r *http.Request) (*models.User, error) {
	// 1. Cloudflare Access Identity Header
	cfEmail := strings.TrimSpace(r.Header.Get("Cf-Access-Authenticated-User-Email"))
	if cfEmail != "" {
		return a.resolveUser(strings.ToLower(cfEmail))
	}

	// 2. Bearer token / Session Cookie
	token := extractToken(r)
	if token != "" {
		user, err := a.db.GetUserBySessionToken(token)
		if err == nil && user != nil {
			return user, nil
		}
	}

	// 3. Dev mode fallback
	if a.devMode {
		devEmail := "admin@local.dev"
		if a.adminEmail != "" {
			devEmail = a.adminEmail
		}
		return a.resolveUser(devEmail)
	}

	return nil, errors.New("unauthorized: missing Cloudflare identity or session token")
}

func (a *Authenticator) resolveUser(email string) (*models.User, error) {
	// Check allowlist if configured
	if len(a.allowedEmail) > 0 {
		allowed := false
		for _, allow := range a.allowedEmail {
			if strings.EqualFold(allow, email) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errors.New("user email is not in Cloudflare Access allowlist")
		}
	}

	defaultRole := models.RoleViewer
	if a.adminEmail != "" && strings.EqualFold(a.adminEmail, email) {
		defaultRole = models.RoleOwner
	} else if a.devMode {
		defaultRole = models.RoleOwner
	}

	return a.db.GetOrCreateUserByEmail(email, defaultRole)
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if cookie, err := r.Cookie("panel_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

// Middleware verifies authentication and attaches user to context
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify origin/CSRF on state-changing requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			if !a.validateOrigin(r) {
				http.Error(w, `{"error":"forbidden: origin validation failed"}`, http.StatusForbidden)
				return
			}
		}

		user, err := a.Authenticate(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePerm returns a middleware enforcing RBAC
func RequirePerm(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserContextKey).(*models.User)
			if !ok || user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if !rbac.Can(user.Role, permission) {
				http.Error(w, `{"error":"forbidden: insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateOrigin performs CSRF check by verifying Origin/Referer matches Host
func (a *Authenticator) validateOrigin(r *http.Request) bool {
	host := r.Host
	origin := r.Header.Get("Origin")
	if origin != "" {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(u.Host), []byte(host)) != 1 {
			// In dev mode allow localhost
			if a.devMode && strings.HasPrefix(u.Host, "localhost") {
				return true
			}
			return false
		}
		return true
	}

	referer := r.Header.Get("Referer")
	if referer != "" {
		u, err := url.Parse(referer)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(u.Host), []byte(host)) != 1 {
			if a.devMode && strings.HasPrefix(u.Host, "localhost") {
				return true
			}
			return false
		}
		return true
	}

	// If neither header is present, allow only in dev mode or safe clients
	return a.devMode || r.Header.Get("Cf-Access-Jwt-Assertion") != ""
}

func GetUser(ctx context.Context) *models.User {
	if u, ok := ctx.Value(UserContextKey).(*models.User); ok {
		return u
	}
	return nil
}
