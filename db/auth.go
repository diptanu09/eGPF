package db

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Remains private to package db since it's only called internally by AuthenticateUser
func getSystemIdentifier() string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 {
				return iface.HardwareAddr.String()
			}
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		return "SECURE_UNKNOWN_TERMINAL"
	}
	return hostname
}

func AuthenticateUser(username, password string) (string, time.Time, error) {
	var userRole string
	var rawLastLogin time.Time
	var status string
	var dbSystemID sql.NullString
	var storedHash string // Holds the encrypted hash retrieved from the database

	// MODIFIED: Fetch the hashed password from the DB instead of doing a plaintext comparison in SQL
	query := "SELECT password, role, last_login, COALESCE(status, 'approved'), system_id FROM users WHERE username = $1;"
	err := db.QueryRow(query, username).Scan(&storedHash, &userRole, &rawLastLogin, &status, &dbSystemID)
	if err != nil {
		return "", time.Time{}, err
	}

	// NEW: Verify if account is flagged as suspended prior to key comparisons
	if status == "suspended" {
		return "", time.Time{}, fmt.Errorf("ACCOUNT_SUSPENDED: Access denied by administration")
	}

	if status == "pending" {
		return "", time.Time{}, fmt.Errorf("AWAITING_ADMIN_APPROVAL")
	}

	// NEW: Cryptographically verify that the input password matches the stored bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("Authentication Rejected") // Password mismatch
	}

	currentTerminalID := getSystemIdentifier()

	if !dbSystemID.Valid || dbSystemID.String == "" {
		_, err = db.Exec("UPDATE users SET system_id = $1 WHERE username = $2;", currentTerminalID, username)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("failed to bind device fingerprint configuration settings")
		}
	} else if dbSystemID.String != currentTerminalID {
		return "", time.Time{}, fmt.Errorf("TERMINAL_LOCK_VIOLATION")
	}

	return userRole, rawLastLogin, nil
}

func RefreshUserTimestamp(username string) {
	_, _ = db.Exec("UPDATE users SET last_login = NOW() WHERE username = $1;", username)
}

func ExecuteInsertUser(newUsername, newPassword, newRole string) error {
	// NEW: Hash the password before saving it to the database
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to process security key encryption")
	}

	queryStr := "INSERT INTO users (username, password, role, last_login, status) VALUES ($1, $2, $3, NOW(), 'approved');"
	_, err = db.Exec(queryStr, newUsername, string(hashedPassword), newRole)
	return err
}

func ExecuteRegisterSelf(username, password string) error {
	var exists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username=$1);", username).Scan(&exists)
	if exists {
		return fmt.Errorf("Username is already taken")
	}

	// NEW: Hash the password before queuing the registration request
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to process security key encryption")
	}

	queryStr := "INSERT INTO users (username, password, role, last_login, status) VALUES ($1, $2, 'user', NOW(), 'pending');"
	_, err = db.Exec(queryStr, username, string(hashedPassword))
	return err
}

func FetchPendingUserCount() int {
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE status = 'pending';").Scan(&count)
	return count
}

func FetchPendingUsers() ([]string, error) {
	rows, err := db.Query("SELECT username FROM users WHERE status = 'pending' ORDER BY username ASC;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err == nil {
			users = append(users, u)
		}
	}
	return users, nil
}

func ExecuteProcessApproval(targetUser, assignedRole string, approve bool) error {
	if approve {
		_, err := db.Exec("UPDATE users SET status = 'approved', role = $1 WHERE username = $2;", assignedRole, targetUser)
		return err
	}
	_, err := db.Exec("DELETE FROM users WHERE username = $1 AND status = 'pending';", targetUser)
	return err
}

func FetchSystemUsers(searchFilter string) ([][]string, error) {
	directoryData := [][]string{}

	baseQuery := `
        SELECT username, role, COALESCE(last_login::text, 'Never Logged In'), status, COALESCE(system_id, 'UNLOCKED') 
        FROM users`

	var rows *sql.Rows
	var err error

	if searchFilter != "" {
		queryStr := baseQuery + " WHERE username LIKE $1 ORDER BY username ASC;"
		rows, err = db.Query(queryStr, "%"+searchFilter+"%")
	} else {
		queryStr := baseQuery + " ORDER BY username ASC;"
		rows, err = db.Query(queryStr)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var uName, uRole, lastLog, uStatus, sysID string
		if err := rows.Scan(&uName, &uRole, &lastLog, &uStatus, &sysID); err == nil {
			directoryData = append(directoryData, []string{uName, uRole, lastLog, uStatus, sysID})
		}
	}
	return directoryData, nil
}

func ExecuteUpdateUserRole(username, newRole string) error {
	_, err := db.Exec("UPDATE users SET role = $1 WHERE username = $2;", newRole, username)
	return err
}

func ExecuteResetSystemLock(username string) error {
	_, err := db.Exec("UPDATE users SET system_id = NULL WHERE username = $1;", username)
	return err
}

// FEATURE: Toggle the user's suspension profile status flag criteria
func ExecuteToggleUserSuspend(username string, shouldSuspend bool) error {
	targetStatus := "approved"
	if shouldSuspend {
		targetStatus = "suspended"
	}
	_, err := db.Exec("UPDATE users SET status = $1 WHERE username = $2;", targetStatus, username)
	return err
}

// FEATURE: Re-hash and overwrite an active user password credential bundle safely
func ExecuteResetUserPassword(username, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to process password re-encryption sequence")
	}
	_, err = db.Exec("UPDATE users SET password = $1 WHERE username = $2;", string(hashedPassword), username)
	return err
}

// FEATURE: Drop active session identification data stamps and clear hardware signature links instantly
func ExecuteTerminateUserSession(username string) error {
	_, err := db.Exec("UPDATE users SET system_id = NULL, last_login = NULL WHERE username = $1;", username)
	return err
}
