// api/handlers.go - HTTP handler functions for the REST API endpoints.
//
// This file contains the handler functions that are called when specific API endpoints are hit.
// It handles request parsing, calls the appropriate service functions, and writes responses.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// SetupPublicRoutes defines public API endpoints that do not require authentication.
func SetupPublicRoutes(router *mux.Router, releaseService *ReleaseService, userService *UserService, logger *log.Logger) {
	router.HandleFunc("/status", handleGetStatus(releaseService, logger)).Methods("GET")
	router.HandleFunc("/packages", handleListPackages(releaseService, logger)).Methods("GET")
	router.HandleFunc("/packages/{software_name}/releases", handleListReleasesForSoftware(releaseService, logger)).Methods("GET")
	router.HandleFunc("/packages/{software_name}/latest", handleGetLatestReleaseForSoftware(releaseService, logger)).Methods("GET")
}

// SetupAdminRoutes defines admin API endpoints requiring basic authentication and admin role.
func SetupAdminRoutes(router *mux.Router, releaseService *ReleaseService, userService *UserService, authService *AuthService, logger *log.Logger) {
	adminRouter := router.PathPrefix("/admin").Subrouter()
	adminRouter.Use(authService.BasicAuthMiddleware)
	adminRouter.Use(AdminRoleMiddleware) // Ensure only admins can access

	adminRouter.HandleFunc("/users", handleListUsers(userService, logger)).Methods("GET")
	adminRouter.HandleFunc("/users", handleCreateUser(userService, logger)).Methods("POST")
	adminRouter.HandleFunc("/users/{username}", handleUpdateUser(userService, logger)).Methods("PUT")
	adminRouter.HandleFunc("/users/{username}", handleDeleteUser(userService, logger)).Methods("DELETE")
	adminRouter.HandleFunc("/users/{username}/status", handleEnableDisableUser(userService, logger)).Methods("PATCH")

	adminRouter.HandleFunc("/packages", handleCreateSoftwarePackage(releaseService, logger)).Methods("POST")
	adminRouter.HandleFunc("/packages/{software_name}", handleUpdateSoftwarePackage(releaseService, logger)).Methods("PUT")
	adminRouter.HandleFunc("/packages/{software_name}", handleDeleteSoftwarePackage(releaseService, logger)).Methods("DELETE")
	adminRouter.HandleFunc("/packages/{software_name}/status", handleEnableDisableSoftwarePackage(releaseService, logger)).Methods("PATCH")
}

// SetupUserRoutes defines user API endpoints requiring basic authentication for all users.
func SetupUserRoutes(router *mux.Router, userService *UserService, authService *AuthService, logger *log.Logger) {
	userRouter := router.PathPrefix("/auth").Subrouter()
	userRouter.Use(authService.BasicAuthMiddleware) // All authenticated users

	userRouter.HandleFunc("/token", handleCreateAPIToken(userService, authService, logger)).Methods("POST")
}

// SetupTokenRoutes defines API endpoints requiring API key authentication in header.
func SetupTokenRoutes(router *mux.Router, releaseService *ReleaseService, authService *AuthService, logger *log.Logger) {
	tokenRouter := router.PathPrefix("/releases").Subrouter()
	tokenRouter.Use(authService.APIKeyAuthMiddleware) // API Key required in header

	tokenRouter.HandleFunc("", handleUploadRelease(releaseService, logger)).Methods("POST")
	tokenRouter.HandleFunc("/{software_name}/{version}", handleRetrieveRelease(releaseService, logger)).Methods("GET")
}

// --- Public Endpoints Handlers ---

func handleGetStatus(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		status := map[string]interface{}{
			"uptime":         time.Since(startTime).String(),            // Placeholder - needs actual uptime tracking
			"total_packages": releaseService.GetTotalSoftwarePackages(), // Placeholder - needs implementation
			"total_releases": releaseService.GetTotalReleases(),         // Placeholder - needs implementation
		}
		respondJSON(w, http.StatusOK, status)
	}
}

func handleListPackages(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		packages, err := releaseService.ListSoftwarePackages()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to list software packages")
			return
		}
		respondJSON(w, http.StatusOK, packages)
	}
}

func handleListReleasesForSoftware(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		softwareName := vars["software_name"]
		sort := r.URL.Query().Get("sort")
		order := r.URL.Query().Get("order")

		releases, err := releaseService.ListReleasesForSoftware(softwareName, sort, order)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to list releases for software")
			return
		}
		respondJSON(w, http.StatusOK, releases)
	}
}

func handleGetLatestReleaseForSoftware(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		softwareName := vars["software_name"]

		release, err := releaseService.GetLatestReleaseForSoftware(softwareName)
		if err != nil {
			respondError(w, http.StatusNotFound, fmt.Sprintf("No releases found for software: %s", softwareName))
			return
		}
		respondJSON(w, http.StatusOK, release)
	}
}

// --- Admin Endpoints Handlers ---

func handleListUsers(userService *UserService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := userService.ListUsers()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to list users")
			return
		}
		respondJSON(w, http.StatusOK, users)
	}
}

func handleCreateUser(userService *UserService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var newUserRequest CreateUserRequest
		if err := decodeJSONBody(w, r, &newUserRequest); err != nil {
			return // decodeJSONBody already handles error response
		}

		u := &User{
			Username:     newUserRequest.Username,
			PasswordHash: HashPassword(newUserRequest.Password),
			Roles:        newUserRequest.Roles,
			Enabled:      true, // Default to enabled on creation
		}
		if err := userService.CreateUser(u); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to create user: %v", err))
			return
		}
		respondJSON(w, http.StatusCreated, map[string]string{"message": "User created successfully"})
	}
}

func handleUpdateUser(userService *UserService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]
		var updateUserRequest UpdateUserRequest
		if err := decodeJSONBody(w, r, &updateUserRequest); err != nil {
			return
		}

		if err := userService.UpdateUserPassword(username, updateUserRequest.Password); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to update user: %v", err))
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "User updated successfully"})
	}
}

func handleDeleteUser(userService *UserService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		if err := userService.DeleteUser(username); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to delete user: %v", err))
			return
		}
		respondNoContent(w)
	}
}

func handleEnableDisableUser(userService *UserService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]
		var statusRequest EnableDisableRequest
		if err := decodeJSONBody(w, r, &statusRequest); err != nil {
			return
		}

		if err := userService.EnableDisableUser(username, !statusRequest.Enabled); // Note the negation to toggle
		err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to enable/disable user: %v", err))
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "User status updated successfully"})
	}
}

func handleCreateSoftwarePackage(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var newSoftwareRequest CreateSoftwareRequest
		if err := decodeJSONBody(w, r, &newSoftwareRequest); err != nil {
			return
		}

		software := &SoftwarePackage{
			Name:        newSoftwareRequest.Name,
			Description: newSoftwareRequest.Description,
			Category:    newSoftwareRequest.Category,
			Enabled:     true, // Default to enabled
		}

		if err := releaseService.CreateSoftwarePackage(software); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to create software package: %v", err))
			return
		}
		respondJSON(w, http.StatusCreated, map[string]string{"message": "Software package created successfully"})
	}
}

func handleUpdateSoftwarePackage(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		softwareName := vars["software_name"]
		var updateSoftwareRequest UpdateSoftwareRequest
		if err := decodeJSONBody(w, r, &updateSoftwareRequest); err != nil {
			return
		}

		if err := releaseService.UpdateSoftwarePackageDetails(softwareName, updateSoftwareRequest.Description, updateSoftwareRequest.Category); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to update software package: %v", err))
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "Software package updated successfully"})
	}
}

func handleDeleteSoftwarePackage(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		softwareName := vars["software_name"]

		if err := releaseService.DeleteSoftwarePackage(softwareName); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to delete software package: %v", err))
			return
		}
		respondNoContent(w)
	}
}

func handleEnableDisableSoftwarePackage(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		softwareName := vars["software_name"]
		var statusRequest EnableDisableRequest
		if err := decodeJSONBody(w, r, &statusRequest); err != nil {
			return
		}

		if err := releaseService.EnableDisableSoftwarePackage(softwareName, !statusRequest.Enabled); // Toggle status
		err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to enable/disable software package: %v", err))
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "Software package status updated successfully"})
	}
}

// --- User Endpoints Handlers ---

func handleCreateAPIToken(userService *UserService, authService *AuthService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _, _ := r.BasicAuth() // Already authenticated by BasicAuthMiddleware

		token, err := authService.GenerateAPIToken(username)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate API token")
			return
		}
		respondJSON(w, http.StatusCreated, map[string]string{"api_key": token})
	}
}

// --- Token-Based Endpoints Handlers ---

func handleUploadRelease(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var uploadRequest UploadReleaseRequest
		if err := decodeJSONBody(w, r, &uploadRequest); err != nil {
			return
		}

		// Simulate downloading the file from file_url and creating a tgz (replace with actual logic)
		tempDir, err := os.MkdirTemp("", "release-temp-")
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to create temporary directory")
			return
		}
		defer os.RemoveAll(tempDir) // Clean up temp dir

		downloadedFilePath := filepath.Join(tempDir, "downloaded-file") // Simulate downloaded file
		if err := os.WriteFile(downloadedFilePath, []byte("This is a dummy release file content."), 0644); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to create dummy release file")
			return
		}

		tgzFilePath := filepath.Join(tempDir, "release.tgz")                      // Simulate tgz creation
		if err := createTGZArchive(downloadedFilePath, tgzFilePath); err != nil { // Dummy implementation below
			respondError(w, http.StatusInternalServerError, "Failed to create TGZ archive")
			return
		}
		defer os.Remove(tgzFilePath) // Clean up tgz file

		releaseMetadata := ReleaseMetadata{
			SoftwareName:     uploadRequest.SoftwareName,
			Version:          uploadRequest.Version,
			ReleaseDate:      uploadRequest.ReleaseDate,
			Changelog:        uploadRequest.Changelog,
			FileSize:         1024, // Dummy size
			ReleaseState:     "available",
			ReleaseTimestamp: time.Now(), // Current Timestamp
		}

		if err := releaseService.UploadRelease(tgzFilePath, releaseMetadata); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to upload release: %v", err))
			return
		}

		respondJSON(w, http.StatusCreated, map[string]string{"message": "Release uploaded successfully"})
	}
}

func handleRetrieveRelease(releaseService *ReleaseService, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		softwareName := vars["software_name"]
		version := vars["version"]

		releaseFilePath, err := releaseService.GetReleaseFilePath(softwareName, version)
		if err != nil {
			respondError(w, http.StatusNotFound, fmt.Sprintf("Release not found: %v", err))
			return
		}

		http.ServeFile(w, r, releaseFilePath) // Serve the TGZ file
	}
}

// --- Helper functions ---

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		response, _ := json.Marshal(payload) // Ignoring error for simplicity in example
		w.Write(response)                    // Ignoring error for simplicity in example
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func respondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "" {
		if r.Header.Get("Content-Type") != "application/json" {
			msg := "Content-Type header is not application/json"
			respondError(w, http.StatusUnsupportedMediaType, msg)
			return fmt.Errorf(msg)
		}
	} else {
		msg := "Content-Type header is not present"
		respondError(w, http.StatusUnsupportedMediaType, msg)
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Catch extra fields in request body

	if err := decoder.Decode(&dst); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			respondError(w, http.StatusBadRequest, msg)

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			respondError(w, http.StatusBadRequest, msg)

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			respondError(w, http.StatusBadRequest, msg)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			respondError(w, http.StatusBadRequest, msg)

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			respondError(w, http.StatusBadRequest, msg)

		case err.Error() == "http: request body too large":
			msg := "Request body too large"
			respondError(w, http.StatusRequestEntityTooLarge, msg)

		default:
			log.Printf("Error decoding JSON: %v", err.Error()) // Log unexpected errors for debugging
			respondError(w, http.StatusBadRequest, "Bad request")
		}
		return err
	}

	if decoder.More() {
		msg := "Request body must only contain a single JSON object"
		respondError(w, http.StatusBadRequest, msg)
		return fmt.Errorf(msg)
	}

	return nil
}

// Dummy TGZ creation function - replace with actual implementation
func createTGZArchive(sourceFile string, destFile string) error {
	file, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add source file to the archive
	info, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}
	_, err = tw.Write(data)
	return err
}
