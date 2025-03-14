// internal/repository/repository.go - Release repository management.
//
// This file defines the ReleaseMetadata struct, ReleaseDatabase interface,
// and JSONReleaseDatabase implementation for managing release metadata and storage.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ReleaseDatabase interface defines operations for release metadata management.
type ReleaseDatabase interface {
	GetReleaseMetadata(softwareName string, version string) (*ReleaseMetadata, error)
	ListReleasesMetadataForSoftware(softwareName string) ([]*ReleaseMetadata, error)
	ListAllReleasesMetadata() ([]*ReleaseMetadata, error)
	CreateReleaseMetadata(metadata *ReleaseMetadata) error
	UpdateReleaseMetadata(metadata *ReleaseMetadata) error // For status updates, etc.
	DeleteReleaseMetadata(softwareName string, version string) error
	ReconcileReleases(repoPath string) error
	StoreReleaseFile(repoPath string, tgzFilePath string, metadata *ReleaseMetadata) (string, error)
	GetReleaseTGZReader(repoPath string, metadata *ReleaseMetadata) (io.ReadCloser, error)
	GetReleaseFilePath(repoPath string, metadata *ReleaseMetadata) string
	Close() error
}

// JSONReleaseDatabase is a JSON file-based implementation of ReleaseDatabase.
type JSONReleaseDatabase struct {
	filepath string
	releases map[string]map[string]*ReleaseMetadata // softwareName -> version -> metadata
	mu       sync.RWMutex                           // Mutex for read/write operations
	config   *Config
}

// NewJSONReleaseDatabase creates a new JSONReleaseDatabase instance.
func NewJSONReleaseDatabase(filepath string) (*JSONReleaseDatabase, error) {
	db := &JSONReleaseDatabase{
		filepath: filepath,
		releases: make(map[string]map[string]*ReleaseMetadata),
	}
	if err := db.loadReleasesMetadata(); err != nil {
		return nil, err
	}
	return db, nil
}

// GetReleaseFilePath returns the file path for a release based on the repository path and release metadata.
func (db *JSONReleaseDatabase) GetReleaseFilePath(repoPath string, metadata *ReleaseMetadata) string {
	return db.getReleaseFilePath(repoPath, metadata)
}

// GetReleaseMetadata retrieves release metadata for a specific software and version.
func (db *JSONReleaseDatabase) GetReleaseMetadata(softwareName string, version string) (*ReleaseMetadata, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	softwareReleases, ok := db.releases[softwareName]
	if !ok {
		return nil, fmt.Errorf("software package not found: %s", softwareName)
	}
	metadata, ok := softwareReleases[version]
	if !ok {
		return nil, fmt.Errorf("release version not found for software %s: %s", softwareName, version)
	}
	return metadata, nil
}

// ListReleasesMetadataForSoftware retrieves all release metadata for a software package.
func (db *JSONReleaseDatabase) ListReleasesMetadataForSoftware(softwareName string) ([]*ReleaseMetadata, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	softwareReleases, ok := db.releases[softwareName]
	if !ok {
		return nil, fmt.Errorf("software package not found: %s", softwareName)
	}
	var releasesMetadata []*ReleaseMetadata
	for _, metadata := range softwareReleases {
		releasesMetadata = append(releasesMetadata, metadata)
	}
	return releasesMetadata, nil
}

// ListAllReleasesMetadata retrieves metadata for all releases across all software packages.
func (db *JSONReleaseDatabase) ListAllReleasesMetadata() ([]*ReleaseMetadata, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var allReleasesMetadata []*ReleaseMetadata
	for _, softwareReleases := range db.releases {
		for _, metadata := range softwareReleases {
			allReleasesMetadata = append(allReleasesMetadata, metadata)
		}
	}
	return allReleasesMetadata, nil
}

// CreateReleaseMetadata creates new release metadata.
func (db *JSONReleaseDatabase) CreateReleaseMetadata(metadata *ReleaseMetadata) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, softwareExists := db.releases[metadata.SoftwareName]; !softwareExists {
		db.releases[metadata.SoftwareName] = make(map[string]*ReleaseMetadata)
	}
	if _, versionExists := db.releases[metadata.SoftwareName][metadata.Version]; versionExists {
		return fmt.Errorf("release version already exists for software %s: %s", metadata.SoftwareName, metadata.Version)
	}
	db.releases[metadata.SoftwareName][metadata.Version] = metadata
	return db.saveReleasesMetadata()
}

// UpdateReleaseMetadata updates existing release metadata.
func (db *JSONReleaseDatabase) UpdateReleaseMetadata(metadata *ReleaseMetadata) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, softwareExists := db.releases[metadata.SoftwareName]; !softwareExists {
		return fmt.Errorf("software package not found: %s", metadata.SoftwareName)
	}
	if _, versionExists := db.releases[metadata.SoftwareName][metadata.Version]; !versionExists {
		return fmt.Errorf("release version not found for software %s: %s", metadata.SoftwareName, metadata.Version)
	}
	db.releases[metadata.SoftwareName][metadata.Version] = metadata // Overwrite with new metadata
	return db.saveReleasesMetadata()
}

// DeleteReleaseMetadata deletes release metadata.
func (db *JSONReleaseDatabase) DeleteReleaseMetadata(softwareName string, version string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, softwareReleases := db.releases[softwareName]; !softwareReleases {
		return fmt.Errorf("software package not found: %s", softwareName)
	}
	if _, versionExists := db.releases[softwareName][version]; !versionExists {
		return fmt.Errorf("release version not found for software %s: %s", softwareName, version)
	}
	delete(db.releases[softwareName], version)
	if len(db.releases[softwareName]) == 0 { // Clean up software entry if no releases left
		delete(db.releases, softwareName)
	}
	return db.saveReleasesMetadata()
}

// ReconcileReleases reconciles the metadata database with the actual files in the repository.
func (db *JSONReleaseDatabase) ReconcileReleases(repoPath string) error {
	allReleasesMetadata, err := db.ListAllReleasesMetadata()
	if err != nil {
		return fmt.Errorf("failed to list all release metadata for reconciliation: %w", err)
	}

	for _, metadata := range allReleasesMetadata {
		releaseFilePath := db.getReleaseFilePath(repoPath, metadata)
		_, err := os.Stat(releaseFilePath)
		if os.IsNotExist(err) {
			metadata.ReleaseState = "unavailable" // Mark as unavailable if file is missing
			if err := db.UpdateReleaseMetadata(metadata); err != nil {
				return fmt.Errorf("failed to update metadata during reconciliation for %s %s: %w", metadata.SoftwareName, metadata.Version, err)
			}
		} else if err == nil {
			metadata.ReleaseState = "available" // Ensure state is "available" if file exists
			fileInfo, err := os.Stat(releaseFilePath)
			if err != nil {
				return fmt.Errorf("failed to stat file during reconciliation for %s %s: %w", metadata.SoftwareName, metadata.Version, err)
			}
			if metadata.FileSize != fileInfo.Size() {
				metadata.FileSize = fileInfo.Size() // Update file size if it has changed
				if err := db.UpdateReleaseMetadata(metadata); err != nil {
					return fmt.Errorf("failed to update file size during reconciliation for %s %s: %w", metadata.SoftwareName, metadata.Version, err)
				}
			}

			// Optionally verify timestamp as well if needed, but file size is more robust.
		} else if err != nil {
			return fmt.Errorf("error checking release file during reconciliation for %s %s: %w", metadata.SoftwareName, metadata.Version, err)
		}
	}
	return db.saveReleasesMetadata() // Save any state changes after reconciliation
}

// Close closes the database connection (no action needed for JSON file).
func (db *JSONReleaseDatabase) Close() error {
	return nil // No resources to close for JSON file DB
}

// loadReleasesMetadata loads release metadata from the JSON file.
func (db *JSONReleaseDatabase) loadReleasesMetadata() error {
	if _, err := os.Stat(db.filepath); os.IsNotExist(err) {
		return nil // File doesn't exist, assume empty DB
	}

	file, err := os.Open(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open release metadata database file: %w", err)
	}
	defer file.Close()

	var releasesMetadata []*ReleaseMetadata // Slice to hold releases from JSON
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&releasesMetadata); err != nil {
		return fmt.Errorf("failed to decode release metadata database: %w", err)
	}

	db.releases = make(map[string]map[string]*ReleaseMetadata) // Initialize map
	for _, metadata := range releasesMetadata {
		if _, ok := db.releases[metadata.SoftwareName]; !ok {
			db.releases[metadata.SoftwareName] = make(map[string]*ReleaseMetadata)
		}
		db.releases[metadata.SoftwareName][metadata.Version] = metadata // Populate nested map
	}
	return nil
}

// saveReleasesMetadata saves release metadata to the JSON file.
func (db *JSONReleaseDatabase) saveReleasesMetadata() error {
	db.mu.RLock() // Read lock to prevent data race during encoding
	releasesSlice := make([]*ReleaseMetadata, 0)
	for _, softwareReleases := range db.releases {
		for _, metadata := range softwareReleases {
			releasesSlice = append(releasesSlice, metadata)
		}
	}
	db.mu.RUnlock()

	file, err := os.Create(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open release metadata database file for writing: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print JSON
	if err := encoder.Encode(releasesSlice); err != nil {
		return fmt.Errorf("failed to encode release metadata database to JSON: %w", err)
	}
	return nil
}

// getSoftwareDirPath constructs the directory path for a software package.
func (db *JSONReleaseDatabase) getSoftwareDirPath(repoPath string, softwareName string) string {
	softwareID := generateSoftwareIDFromName(softwareName)                        // Implement ID generation logic
	dirName := fmt.Sprintf("%06d_%s", softwareID, sanitizeFilename(softwareName)) // REQ-301: Directory naming
	return filepath.Join(repoPath, dirName)
}

// getReleaseFilePath constructs the full file path for a release TGZ file.
func (db *JSONReleaseDatabase) getReleaseFilePath(repoPath string, metadata *ReleaseMetadata) string {
	softwareDirPath := db.getSoftwareDirPath(repoPath, metadata.SoftwareName)
	fileName := fmt.Sprintf("%06d_%s_%02s.%02s.%02s.tgz", generateSoftwareIDFromName(metadata.SoftwareName), sanitizeFilename(metadata.SoftwareName), strings.Split(metadata.Version, ".")[0], strings.Split(metadata.Version, ".")[1], strings.Split(metadata.Version, ".")[2]) // REQ-301: File naming
	return filepath.Join(softwareDirPath, fileName)
}

// EnsureReleaseDirExists creates the software-specific directory if it doesn't exist.
func (db *JSONReleaseDatabase) EnsureReleaseDirExists(repoPath string, softwareName string) error {
	dirPath := db.getSoftwareDirPath(repoPath, softwareName)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create software release directory: %w", err)
		}
	}
	return nil
}

// StoreReleaseFile stores the uploaded release TGZ file in the repository.
func (db *JSONReleaseDatabase) StoreReleaseFile(repoPath string, tgzFilePath string, metadata *ReleaseMetadata) (string, error) {
	if err := db.EnsureReleaseDirExists(repoPath, metadata.SoftwareName); err != nil {
		return "", err
	}
	destFilePath := db.getReleaseFilePath(repoPath, metadata)
	if err := copyFile(tgzFilePath, destFilePath); err != nil {
		return "", fmt.Errorf("failed to store release file: %w", err)
	}
	return destFilePath, nil
}

// GetReleaseTGZReader returns an io.Reader for the release TGZ file.
func (db *JSONReleaseDatabase) GetReleaseTGZReader(repoPath string, metadata *ReleaseMetadata) (io.ReadCloser, error) {
	releaseFilePath := db.getReleaseFilePath(repoPath, metadata)
	file, err := os.Open(releaseFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open release file for reading: %w", err)
	}
	return file, nil
}

// --- Helper functions ---

// generateSoftwareIDFromName generates a unique ID (placeholder - implement actual logic).
func generateSoftwareIDFromName(softwareName string) int {
	// TODO: Implement a proper ID generation strategy (e.g., using UUIDs, or a counter).
	// For now, using a simple hash or fixed number for demonstration.
	hash := 0
	for _, char := range softwareName {
		hash = hash*31 + int(char)
	}
	return hash & 0xFFFFF // Keep it within 6 digits range for example
}

// sanitizeFilename sanitizes a filename to be filesystem-safe (replace invalid chars).
func sanitizeFilename(filename string) string {
	// Replace spaces and other unsafe characters with underscores.
	return strings.ReplaceAll(strings.ToLower(filename), " ", "_")
}

// copyFile copies a file from source to destination path.
func copyFile(sourcePath string, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	return nil
}
