// internal/user/user.go - User management and data structures.
//
// This file defines the User struct and interfaces/implementations for user data access and management.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// User represents a user in the system.
type User struct {
	Username     string   `json:"username"`
	PasswordHash string   `json:"password_hash"`
	Roles        []string `json:"roles"`
	Enabled      bool     `json:"enabled"`
}

// UserDatabase interface defines operations for user management.
type UserDatabase interface {
	GetUserByUsername(username string) (*User, error)
	ListUsers() ([]*User, error)
	CreateUser(user *User) error
	UpdateUserPassword(username string, newPasswordHash string) error
	DeleteUser(username string) error
	EnableDisableUser(username string, enabled bool) error
	Close() error
}

// JSONUserDatabase is a JSON file-based implementation of UserDatabase.
type JSONUserDatabase struct {
	filepath string
	users    map[string]*User
	mu       sync.RWMutex // Mutex for read/write operations
}

// NewJSONUserDatabase creates a new JSONUserDatabase instance.
func NewJSONUserDatabase(filepath string) (*JSONUserDatabase, error) {
	db := &JSONUserDatabase{
		filepath: filepath,
		users:    make(map[string]*User),
	}
	if err := db.loadUsers(); err != nil {
		return nil, err
	}
	return db, nil
}

// GetUserByUsername retrieves a user by username.
func (db *JSONUserDatabase) GetUserByUsername(username string) (*User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	user, ok := db.users[username]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	return user, nil
}

// ListUsers retrieves all users.
func (db *JSONUserDatabase) ListUsers() ([]*User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var userList []*User
	for _, u := range db.users {
		userList = append(userList, u)
	}
	return userList, nil
}

// CreateUser creates a new user.
func (db *JSONUserDatabase) CreateUser(user *User) error {
	db.mu.Lock()
	if _, exists := db.users[user.Username]; exists {
		return fmt.Errorf("user already exists: %s", user.Username)
	}
	db.users[user.Username] = user
	db.mu.Unlock()
	return db.saveUsers()
}

// UpdateUserPassword updates a user's password.
func (db *JSONUserDatabase) UpdateUserPassword(username string, newPasswordHash string) error {
	db.mu.Lock()
	user, ok := db.users[username]
	if !ok {
		return fmt.Errorf("user not found: %s", username)
	}
	user.PasswordHash = newPasswordHash
	db.mu.Unlock()
	return db.saveUsers()
}

// DeleteUser deletes a user.
func (db *JSONUserDatabase) DeleteUser(username string) error {
	db.mu.Lock()
	if _, exists := db.users[username]; !exists {
		return fmt.Errorf("user not found: %s", username)
	}
	delete(db.users, username)
	db.mu.Unlock()
	return db.saveUsers()
}

// EnableDisableUser enables or disables a user account.
func (db *JSONUserDatabase) EnableDisableUser(username string, enabled bool) error {
	db.mu.Lock()
	usr, ok := db.users[username]
	if !ok {
		return fmt.Errorf("user not found: %s", username)
	}
	usr.Enabled = enabled
	db.mu.Unlock()
	return db.saveUsers()
}

// Close closes the database connection (no action needed for JSON file).
func (db *JSONUserDatabase) Close() error {
	return nil // No resources to close for JSON file DB
}

// loadUsers loads users from the JSON file.
func (db *JSONUserDatabase) loadUsers() error {
	if _, err := os.Stat(db.filepath); os.IsNotExist(err) {
		return nil // File doesn't exist, assume empty DB
	}

	file, err := os.Open(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open user database file: %w", err)
	}
	defer file.Close()

	var users []*User // Slice to hold users from JSON
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&users); err != nil {
		return fmt.Errorf("failed to decode user database: %w", err)
	}

	db.users = make(map[string]*User) // Initialize map
	for _, u := range users {
		db.users[u.Username] = u // Populate map for efficient lookup
	}
	return nil
}

// saveUsers saves users to the JSON file.
func (db *JSONUserDatabase) saveUsers() error {
	db.mu.RLock() // Read lock to prevent data race during encoding
	usersSlice := make([]*User, 0, len(db.users))
	for _, user := range db.users {
		usersSlice = append(usersSlice, user)
	}
	db.mu.RUnlock()

	file, err := os.Create(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open user database file for writing: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print JSON
	if err := encoder.Encode(usersSlice); err != nil {
		return fmt.Errorf("failed to encode user database to JSON: %w", err)
	}
	return nil
}
