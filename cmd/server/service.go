// internal/service/service.go - Business logic and service layer.
//
// This file implements the business logic of the application,
// interacting with the data layer (repository) and handling service-level operations.
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ReleaseService struct holds dependencies for release management operations.
type ReleaseService struct {
	config    *Config
	releaseDB ReleaseDatabase
	logger    *log.Logger
}

// NewReleaseService creates a new ReleaseService instance.
func NewReleaseService(cfg *Config, db ReleaseDatabase, logger *log.Logger) *ReleaseService {
	return &ReleaseService{
		config:    cfg,
		releaseDB: db,
		logger:    logger,
	}
}

// GetTotalSoftwarePackages returns the total number of software packages (placeholder).
func (s *ReleaseService) GetTotalSoftwarePackages() int {
	releases, _ := s.releaseDB.ListAllReleasesMetadata() // Ignoring error for simplicity in this example
	softwarePackages := make(map[string]bool)
	for _, r := range releases {
		softwarePackages[r.SoftwareName] = true
	}
	return len(softwarePackages)
}

// GetTotalReleases returns the total number of releases (placeholder).
func (s *ReleaseService) GetTotalReleases() int {
	releases, _ := s.releaseDB.ListAllReleasesMetadata() // Ignoring error for simplicity in this example
	return len(releases)
}

// ListSoftwarePackages retrieves a list of all software packages (names and latest versions).
func (s *ReleaseService) ListSoftwarePackages() ([]*SoftwarePackageInfo, error) {
	allReleases, err := s.releaseDB.ListAllReleasesMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to list all releases for software packages overview: %w", err)
	}

	packageMap := make(map[string]*SoftwarePackageInfo) // softwareName -> PackageInfo
	for _, release := range allReleases {
		if pkgInfo, ok := packageMap[release.SoftwareName]; ok {
			currentVersion, _ := parseVersion(pkgInfo.LatestVersion)
			newVersion, _ := parseVersion(release.Version)
			if newVersion.GreaterThan(currentVersion) {
				pkgInfo.LatestVersion = release.Version // Update to latest version
				pkgInfo.LatestReleaseDate = release.ReleaseDate
			}
		} else {
			packageMap[release.SoftwareName] = &SoftwarePackageInfo{
				Name:              release.SoftwareName,
				LatestVersion:     release.Version,
				LatestReleaseDate: release.ReleaseDate,
			}
		}
	}

	packageList := make([]*SoftwarePackageInfo, 0, len(packageMap))
	for _, pkgInfo := range packageMap {
		packageList = append(packageList, pkgInfo)
	}
	sort.Slice(packageList, func(i, j int) bool { // Sort by software name
		return packageList[i].Name < packageList[j].Name
	})
	return packageList, nil
}

// ListReleasesForSoftware retrieves releases for a specific software, with sorting options.
func (s *ReleaseService) ListReleasesForSoftware(softwareName string, sortField string, sortOrder string) ([]*ReleaseMetadata, error) {
	releases, err := s.releaseDB.ListReleasesMetadataForSoftware(softwareName)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases for software %s: %w", softwareName, err)
	}

	// Sorting logic
	sort.Slice(releases, func(i, j int) bool {
		switch sortField {
		case "version":
			version1, _ := parseVersion(releases[i].Version)
			version2, _ := parseVersion(releases[j].Version)
			if sortOrder == "desc" {
				return version1.GreaterThan(version2)
			}
			return version2.GreaterThan(version1) // Default "asc"
		case "date":
			if sortOrder == "desc" {
				return releases[i].ReleaseDate.After(releases[j].ReleaseDate)
			}
			return releases[j].ReleaseDate.After(releases[i].ReleaseDate) // Default "asc"
		default: // Default sort by version descending
			version1, _ := parseVersion(releases[i].Version)
			version2, _ := parseVersion(releases[j].Version)
			return version1.GreaterThan(version2)
		}
	})
	return releases, nil
}

// GetLatestReleaseForSoftware retrieves the latest release for a specific software.
func (s *ReleaseService) GetLatestReleaseForSoftware(softwareName string) (*ReleaseMetadata, error) {
	releases, err := s.releaseDB.ListReleasesMetadataForSoftware(softwareName)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases for software %s to find latest: %w", softwareName, err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found for software: %s", softwareName)
	}

	sort.Slice(releases, func(i, j int) bool { // Sort by version descending to get latest first
		version1, _ := parseVersion(releases[i].Version)
		version2, _ := parseVersion(releases[j].Version)
		return version1.GreaterThan(version2)
	})
	return releases[0], nil // The first element after sorting is the latest
}

// CreateSoftwarePackage creates a new software package definition.
func (s *ReleaseService) CreateSoftwarePackage(software *SoftwarePackage) error {
	// For now, software package details are stored in memory or could be in metadata DB in future.
	// For now, only name is really used in metadata storage structure.
	// Consider adding a separate SoftwarePackageDatabase if more details need persistence.
	return nil // Placeholder for now, software packages are implicitly created with releases
}

// UpdateSoftwarePackageDetails updates details of a software package (name is key, other details can be updated).
func (s *ReleaseService) UpdateSoftwarePackageDetails(softwareName string, description string, category string) error {
	// Placeholder - update software package details (description, category).
	// Needs to be implemented if SoftwarePackage struct is persisted.
	return nil
}

// DeleteSoftwarePackage deletes a software package and all associated releases.
func (s *ReleaseService) DeleteSoftwarePackage(softwareName string) error {
	// Placeholder - delete software package and releases.
	// Needs to be implemented if SoftwarePackage struct is persisted and releases need cascading delete.
	return nil
}

// EnableDisableSoftwarePackage enables or disables a software package (and potentially its releases).
func (s *ReleaseService) EnableDisableSoftwarePackage(softwareName string, enabled bool) error {
	// Placeholder - enable/disable software package.
	// Needs to be implemented if SoftwarePackage struct has an enabled status and impacts release visibility.
	return nil
}

// UploadRelease handles the upload of a new software release.
func (s *ReleaseService) UploadRelease(tgzFilePath string, metadata ReleaseMetadata) error {
	metadata.ReleaseTimestamp = time.Now() // Set upload timestamp
	destFilePath, err := s.releaseDB.StoreReleaseFile(s.config.RepositoryPath, tgzFilePath, &metadata)
	if err != nil {
		return fmt.Errorf("failed to store release file: %w", err)
	}

	fileInfo, err := os.Stat(destFilePath)
	if err != nil {
		return fmt.Errorf("failed to get file size after storing release: %w", err)
	}
	metadata.FileSize = fileInfo.Size()
	metadata.ReleaseState = "available" // Mark as available after successful upload

	if err := s.releaseDB.CreateReleaseMetadata(&metadata); err != nil {
		// Rollback: delete the file if metadata creation fails (consider more robust transaction).
		os.Remove(destFilePath)
		return fmt.Errorf("failed to create release metadata and rollback file storage: %w", err)
	}
	return nil
}

// GetReleaseFilePath returns the file path for a specific release.
func (s *ReleaseService) GetReleaseFilePath(softwareName string, version string) (string, error) {
	metadata, err := s.releaseDB.GetReleaseMetadata(softwareName, version)
	if err != nil {
		return "", err
	}
	if metadata.ReleaseState != "available" {
		return "", fmt.Errorf("release is not available: %s %s", softwareName, version)
	}
	reader, err := s.releaseDB.GetReleaseTGZReader(s.config.RepositoryPath, metadata)
	if err != nil {
		return "", err
	}
	defer reader.Close() // Close reader after getting path (reader itself not directly used here)

	return s.releaseDB.GetReleaseFilePath(s.config.RepositoryPath, metadata), nil // Return path from DB logic
}

// ReconcileReleases performs reconciliation of the release database with the file system.
func (s *ReleaseService) ReconcileReleases() error {
	return s.releaseDB.ReconcileReleases(s.config.RepositoryPath)
}

// --- Helper functions ---

// version type and parsing/comparison logic (can be moved to a separate util package if needed).
type Version struct {
	Major    int
	Minor    int
	Patch    int
	Original string // Store original string for representation
}

func parseVersion(versionStr string) (Version, error) {
	parts := strings.SplitN(versionStr, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format: %s, expected X.Y.Z", versionStr)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %w", err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %w", err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %w", err)
	}

	return Version{Major: major, Minor: minor, Patch: patch, Original: versionStr}, nil
}

// GreaterThan compares two versions.
func (v Version) GreaterThan(other Version) bool {
	if v.Major > other.Major {
		return true
	}
	if v.Major < other.Major {
		return false
	}
	// Majors are equal, compare minors
	if v.Minor > other.Minor {
		return true
	}
	if v.Minor < other.Minor {
		return false
	}
	// Majors and minors equal, compare patches
	return v.Patch > other.Patch
}

// UserService struct for user related operations.
type UserService struct {
	userDB UserDatabase // Assuming UserDatabase is defined in repository package
	logger *log.Logger
}

// NewUserService creates a new UserService instance.
func NewUserService(db UserDatabase, logger *log.Logger) *UserService {
	return &UserService{
		userDB: db,
		logger: logger,
	}
}

// GetUserByUsername retrieves a user by username.
func (s *UserService) GetUserByUsername(username string) (*User, error) {
	usr, err := s.userDB.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", username, err)
	}
	return usr, nil
}

// ListUsers retrieves all users.
func (s *UserService) ListUsers() ([]*User, error) {
	users, err := s.userDB.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}

// CreateUser creates a new
func (s *UserService) CreateUser(user *User) error {
	if err := s.userDB.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create user %s: %w", user.Username, err)
	}
	return nil
}

// UpdateUserPassword updates a user's password.
func (s *UserService) UpdateUserPassword(username string, newPassword string) error {
	hashedPassword := HashPassword(newPassword) // Hash the new password
	if err := s.userDB.UpdateUserPassword(username, hashedPassword); err != nil {
		return fmt.Errorf("failed to update password for user %s: %w", username, err)
	}
	return nil
}

// DeleteUser deletes a
func (s *UserService) DeleteUser(username string) error {
	if err := s.userDB.DeleteUser(username); err != nil {
		return fmt.Errorf("failed to delete user %s: %w", username, err)
	}
	return nil
}

// EnableDisableUser enables or disables a
func (s *UserService) EnableDisableUser(username string, enabled bool) error {
	if err := s.userDB.EnableDisableUser(username, enabled); err != nil {
		return fmt.Errorf("failed to enable/disable user %s: %w", username, err)
	}
	return nil
}
