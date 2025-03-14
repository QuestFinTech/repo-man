// main.go - Entry point for the Release Repository Manager application.
//
// This file sets up the configuration, logging, database connections,
// and starts the REST API server. It also handles graceful shutdown.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file if it exists
	godotenv.Load()

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger, logFile, err := SetupLogger(cfg.LogFilePath)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer logFile.Close() // Close log file on exit

	logger.Printf("Starting Release Repository Manager version %s", ServerVersion)
	logger.Printf("Configuration loaded from: %s", cfg.ConfigFileUsed)

	userDB, err := NewJSONUserDatabase(cfg.DataPath + "/users.json")
	if err != nil {
		logger.Fatalf("Failed to initialize user database: %v", err)
	}
	defer userDB.Close()

	releaseDB, err := NewJSONReleaseDatabase(cfg.DataPath + "/releases.json")
	if err != nil {
		logger.Fatalf("Failed to initialize release database: %v", err)
	}
	defer releaseDB.Close()

	releaseService := NewReleaseService(cfg, releaseDB, logger)
	userService := NewUserService(userDB, logger)
	authService := NewAuthService(userService, logger)

	// Initialize Admin User if not exists
	if _, err := userService.GetUserByUsername("admin"); err != nil {
		defaultAdmin := &User{
			Username:     "admin",
			PasswordHash: HashPassword("admin"), // Default password as specified
			Roles:        []string{"administrator"},
			Enabled:      true,
		}
		if err := userService.CreateUser(defaultAdmin); err != nil {
			logger.Fatalf("Failed to create default admin user: %v", err)
		}
		logger.Println("Default administrator user 'admin' created.")
	}

	// Perform database reconciliation at startup
	if err := releaseService.ReconcileReleases(); err != nil {
		logger.Fatalf("Release database reconciliation failed: %v", err)
		os.Exit(1) // Exit with error as per REQ-302
	}
	logger.Println("Release database reconciliation completed successfully.")

	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v1").Subrouter() // Versioned API

	SetupPublicRoutes(apiRouter, releaseService, userService, logger)
	SetupAdminRoutes(apiRouter, releaseService, userService, authService, logger)
	SetupUserRoutes(apiRouter, userService, authService, logger)
	SetupTokenRoutes(apiRouter, releaseService, authService, logger)

	// Add middleware for logging, rate limiting, CORS, and JSON validation can be added here.
	// Example: router.Use(middleware.RequestLogger(logger))

	server := &http.Server{
		Addr:         cfg.APIServerAddress,
		Handler:      router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Printf("Starting API server at %s", cfg.APIServerAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownDelay)*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server shutdown failed: %v", err)
	}
	logger.Println("Server shutdown completed.")
}
