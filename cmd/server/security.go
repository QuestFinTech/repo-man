// internal/security/security.go - Security related functionalities.
//
// This file implements security features like authentication, authorization,
// password hashing, and API key generation.
package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AuthService struct for authentication and authorization services.
type AuthService struct {
	userService *UserService // Dependency on UserService
	logger      *log.Logger
	apiKeys     map[string]string // In-memory API key storage (for simplicity, consider persistence)
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(userService *UserService, logger *log.Logger) *AuthService {
	return &AuthService{
		userService: userService,
		logger:      logger,
		apiKeys:     make(map[string]string), // Initialize API key map
	}
}

// HashPassword hashes a password using MD5 (for simplicity as per spec, consider bcrypt in real-world).
func HashPassword(password string) string {
	hasher := md5.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}

// CompareHashAndPassword compares a password with its hash.
func CompareHashAndPassword(hashedPassword, password string) bool {
	return hashedPassword == HashPassword(password)
}

// BasicAuthMiddleware is middleware for HTTP Basic Authentication.
func (as *AuthService) BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			respondUnauthorized(w, "Basic Auth credentials required")
			return
		}

		usr, err := as.userService.GetUserByUsername(username)
		if err != nil {
			respondUnauthorized(w, "Invalid username or password")
			return
		}

		if !usr.Enabled {
			respondUnauthorized(w, "Account disabled")
			return
		}

		if !CompareHashAndPassword(usr.PasswordHash, password) {
			respondUnauthorized(w, "Invalid username or password")
			return
		}

		// Authentication successful, proceed
		ctx := context.WithValue(r.Context(), ContextKeyUsername, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminRoleMiddleware is middleware to check if the user has the "administrator" role.
func AdminRoleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//username := r.Context().Value(ContextKeyUsername).(string) // Get username from context
		userRoles := getUserRolesFromContext(r.Context())

		isAdmin := false
		for _, role := range userRoles {
			if role == "administrator" {
				isAdmin = true
				break
			}
		}

		if !isAdmin {
			respondForbidden(w, "Administrator role required")
			return
		}

		// Authorization successful, proceed
		next.ServeHTTP(w, r)
	})
}

// APIKeyAuthMiddleware is middleware for API Key authentication via header.
func (as *AuthService) APIKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractAPIKeyFromHeader(r)
		if apiKey == "" {
			respondUnauthorized(w, "API Key required in Authorization header")
			return
		}

		username, ok := as.validateAPIKey(apiKey)
		if !ok {
			respondUnauthorized(w, "Invalid API Key")
			return
		}

		// Authentication successful, proceed
		ctx := context.WithValue(r.Context(), ContextKeyUsername, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GenerateAPIToken generates a new API token for a user.
func (as *AuthService) GenerateAPIToken(username string) (string, error) {
	token := uuid.New().String()
	as.apiKeys[token] = username // Store token -> username (consider persistent storage)
	return token, nil
}

// validateAPIKey validates an API key and returns the associated username if valid.
func (as *AuthService) validateAPIKey(apiKey string) (string, bool) {
	username, ok := as.apiKeys[apiKey]
	return username, ok
}

// extractAPIKeyFromHeader extracts the API key from the Authorization header (Bearer token).
func extractAPIKeyFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, "Bearer ")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != "" {
		return "" // Invalid format
	}

	return strings.TrimSpace(parts[1])
}

// Context keys for storing user information in request context.
type contextKey string

// ContextKeyUsername is the key for username in context.
var ContextKeyUsername contextKey = "username"

// GetUsernameFromContext retrieves the username from the request context.
func GetUsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(ContextKeyUsername).(string)
	return username, ok
}

// getUserRolesFromContext retrieves user roles - placeholder, needs to fetch from DB based on username in context.
func getUserRolesFromContext(ctx context.Context) []string {
	username, ok := GetUsernameFromContext(ctx)
	if !ok {
		return []string{} // No username, no roles
	}

	// Placeholder: Fetch user roles from database based on username (using AuthService's userService)
	// In real implementation, fetch from database using username.
	if username == "admin" { // Example: hardcoded admin role for "admin" user
		return []string{"administrator", "user"}
	}
	return []string{"user"} // Default user role
}

// --- Response helper functions ---

func respondUnauthorized(w http.ResponseWriter, message string) {
	respondErrorWithStatus(w, http.StatusUnauthorized, message)
}

func respondForbidden(w http.ResponseWriter, message string) {
	respondErrorWithStatus(w, http.StatusForbidden, message)
}

func respondErrorWithStatus(w http.ResponseWriter, status int, message string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Release Repository Manager API"`) // For Basic Auth prompt
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response, _ := json.Marshal(map[string]string{"error": message}) // Ignoring error for simplicity
	w.Write(response)                                                // Ignoring error for simplicity
}
