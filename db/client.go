package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
)

// Shared across all files under 'package db' automatically (Declared ONCE)
var db *sql.DB

// InitDatabase initializes the PostgreSQL database connection using maximum SSL security (verify-full)
// Called by main.go upon application startup
func InitDatabase() (*sql.DB, error) {
	// Read connection parameters from environment variables with safe defaults
	dbHost := getEnv("DB_HOST", "10.47.240.169")
	dbPort := getEnvAsInt("DB_PORT", 5432)
	dbUser := getEnv("DB_USER", "egpf_app_user")
	dbPassword := getEnv("DB_PASSWORD", "P@ssw()rd123")
	dbName := getEnv("DB_NAME", "AsstPro")
	dbSchema := getEnv("DB_SCHEMA", "agartala")

	// Path to the root CA certificate used to verify the DB server's identity and hostname
	// Ensure 'root.crt' sits inside a 'certs' subfolder next to your executable
	certPath := filepath.ToSlash(filepath.Join("certs", "root.crt"))

	// Pre-flight check: Verify that the root CA certificate exists on disk
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		log.Printf("SSL Certificate Error: Expected CA certificate at %s but file was not found", certPath)
		return nil, fmt.Errorf("missing required SSL Root CA certificate at %s", certPath)
	}

	// Construct DSN using sslmode=verify-full and passing sslrootcert
	psqlInfo := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s search_path=%s sslmode=verify-full sslrootcert=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSchema, certPath,
	)

	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database handle: %w", err)
	}

	// Verify database reachability and complete the TLS handshake
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("SSL TLS handshake / connection failed (verify-full): %w", err)
	}

	log.Println("[SSL SUCCESS] Database connection established successfully over SSL (verify-full).")

	// Ensure chat messaging schema and indexes are present
	if chatErr := EnsureChatTableExists(); chatErr != nil {
		log.Printf("Notice: Chat table initialization check: %v", chatErr)
	}

	return db, nil
}

// InitDB alias provided for backward compatibility
func InitDB() (*sql.DB, error) {
	return InitDatabase()
}

// GetDB returns the active global database pool handle
func GetDB() *sql.DB {
	return db
}

// CloseDB gracefully shuts down the database connection pool
func CloseDB() {
	if db != nil {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connection pool: %v", err)
		} else {
			log.Println("Database connection closed gracefully.")
		}
	}
}

// FetchLatestAppVersion queries the system_config table for current version details
func FetchLatestAppVersion() (string, string, error) {
	var latestVersion, downloadPath string
	err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'latest_version';").Scan(&latestVersion)
	if err != nil {
		return "2.3.5", "", nil
	}
	err = db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'download_path';").Scan(&downloadPath)
	if err != nil {
		// return "2.3.4", "", nil
		return strings.TrimSpace(latestVersion), "", nil
	}
	// return latestVersion, downloadPath, nil
	return strings.TrimSpace(latestVersion), strings.TrimSpace(downloadPath), nil
}

// ExecuteUpdateAppConfig updates systemic application version and update path settings
func ExecuteUpdateAppConfig(latestVersion, downloadPath string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE system_config SET config_value = $1 WHERE config_key = 'latest_version';", latestVersion)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE system_config SET config_value = $1 WHERE config_key = 'download_path';", downloadPath)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Helper function to read environment string variables with a default fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

// Helper function to read environment integer variables with a default fallback
func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return fallback
	}
	return value
}

// FetchReleaseNotes retrieves the release summary description for the latest version update
func FetchReleaseNotes() string {
	if db == nil {
		return "• Live Enterprise Support Chat Portal\n• Real-Time Unread Message Badge Indicator\n• Dynamic Architecture & Performance Optimizations"
	}
	var notes string
	err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'release_notes';").Scan(&notes)
	if err != nil || strings.TrimSpace(notes) == "" {
		return "• Live Enterprise Support Chat Portal\n• Real-Time Unread Message Badge Indicator\n• Dynamic Architecture & Performance Optimizations"
	}
	return strings.TrimSpace(notes)
}
