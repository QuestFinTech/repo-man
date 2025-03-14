// internal/model/model.go - Data models and request/response structs.
//
// This file defines the data structures (structs) used throughout the application,
// including models for software packages, releases, users, and API request/response formats.
package main

import "time"

// SoftwarePackage represents a software package definition.
type SoftwarePackage struct {
	Name        string `json:"name"`        // Unique name for the software package
	Description string `json:"description"` // Description of the software
	Category    string `json:"category"`    // Category of software (e.g., "Library", "Application")
	Enabled     bool   `json:"enabled"`     // Is the software package enabled for releases/access
}

// SoftwarePackageInfo is a simplified info for listing software packages.
type SoftwarePackageInfo struct {
	Name              string    `json:"name"`
	LatestVersion     string    `json:"version"`
	LatestReleaseDate time.Time `json:"release_date"`
}

// ReleaseMetadata holds metadata about a specific software release.
type ReleaseMetadata struct {
	ID               string    `json:"id"`                // Unique ID for the release (e.g., UUID)
	SoftwareName     string    `json:"software_name"`     // Name of the software package
	Version          string    `json:"version"`           // Release version (X.Y.Z)
	ReleaseTimestamp time.Time `json:"release_timestamp"` // Timestamp of when the release was created/uploaded
	FileSize         int64     `json:"file_size"`         // Size of the release TGZ file in bytes
	ReleaseState     string    `json:"release_state"`     // State of the release ("available", "unavailable", etc.)
	Changelog        string    `json:"changelog"`         // Release changelog/notes
	ReleaseDate      time.Time `json:"release_date"`      // Release date provided by user
}

// --- Request and Response structs for API endpoints ---

// CreateUserRequest is the request body for creating a new user.
type CreateUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Roles    []string `json:"role"` // e.g., ["user", "administrator"]
}

// UpdateUserRequest is the request body for updating a user (e.g., password change).
type UpdateUserRequest struct {
	Password string `json:"password"` // New password
}

// EnableDisableRequest is the request body for enabling/disabling entities (users, software).
type EnableDisableRequest struct {
	Enabled bool `json:"enabled"`
}

// CreateSoftwareRequest is the request body for creating a new software package.
type CreateSoftwareRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// UpdateSoftwareRequest is the request body for updating a software package's details.
type UpdateSoftwareRequest struct {
	Description string `json:"description"`
	Category    string `json:"category"`
}

// UploadReleaseRequest is the request body for uploading a new software release.
type UploadReleaseRequest struct {
	SoftwareName string    `json:"software_name"`
	Version      string    `json:"version"`
	ReleaseDate  time.Time `json:"release_date"`
	Changelog    string    `json:"changelog"`
	FileUrl      string    `json:"file_url"` // URL to download the release file from (or file upload in future)
}
