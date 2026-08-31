package db

/*
#include <stdlib.h>
#include <stdbool.h>
#cgo CFLAGS: -I../vmp_sdk/Include/C/
#cgo LDFLAGS: -L../vmp_sdk/Lib/Windows -L../vmp_sdk/Lib/Windows/MinGW -lVMProtectSDK64
#include "VMProtectSDK.h"
*/
import "C"

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"
	"unsafe"

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
	// ==============================================================================
	// VMPROTECT SDK: ISOLATE CORE LOGIC
	// Wrap this critical security function in the Ultra virtualization macro.
	// This prevents memory dumping of the hashing algorithms and credential checks.
	// ==============================================================================
	marker := C.CString("eGPF_Auth_Logic")
	defer C.free(unsafe.Pointer(marker))

	// Tell VMProtect to start protecting here
	C.VMProtectBeginUltra(marker)
	defer C.VMProtectEnd() // Tell VMProtect to stop protecting when the function finishes
	// ==============================================================================

	var userRole string
	var rawLastLogin time.Time
	var status string
	var dbSystemID sql.NullString
	var storedHash string // Holds the encrypted hash retrieved from the database

	query := "SELECT password, role, COALESCE(last_login, NOW()), COALESCE(status, 'approved'), system_id FROM users WHERE username = $1;"
	err := db.QueryRow(query, username).Scan(&storedHash, &userRole, &rawLastLogin, &status, &dbSystemID)
	if err != nil {
		return "", time.Time{}, err
	}

	// Verify if account is flagged as suspended prior to key comparisons
	if status == "suspended" {
		return "", time.Time{}, fmt.Errorf("ACCOUNT_SUSPENDED: Access denied by administration")
	}

	if status == "pending" {
		return "", time.Time{}, fmt.Errorf("AWAITING_ADMIN_APPROVAL")
	}

	// Cryptographically verify that the input password matches the stored bcrypt hash
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
	if _, err := db.Exec("UPDATE users SET last_login = NOW() WHERE username = $1;", username); err != nil {
		log.Printf("Warning: Failed to refresh user session login footprint: %v", err)
	}
}

func ExecuteInsertUser(operator, newUsername, newPassword, newRole string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to process security key encryption")
	}

	queryStr := "INSERT INTO users (username, password, role, last_login, status) VALUES ($1, $2, $3, NOW(), 'approved');"
	_, err = db.Exec(queryStr, newUsername, string(hashedPassword), newRole)
	if err == nil {
		details := fmt.Sprintf("Provisioned new system user profile: %s with role clearance level [%s]", newUsername, newRole)
		_ = ExecuteInsertAuditLog(operator, "CREATE_USER", details)
	}
	return err
}

func ExecuteRegisterSelf(username, password string) error {
	var exists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username=$1);", username).Scan(&exists)
	if exists {
		return fmt.Errorf("Username is already taken")
	}

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
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func ExecuteProcessApproval(operator, targetUser, assignedRole string, approve bool) error {
	if approve {
		_, err := db.Exec("UPDATE users SET status = 'approved', role = $1 WHERE username = $2;", assignedRole, targetUser)
		if err == nil {
			details := fmt.Sprintf("Approved user registration: %s assigned to clearance level [%s]", targetUser, assignedRole)
			_ = ExecuteInsertAuditLog(operator, "APPROVE_USER_REGISTRATION", details)
		}
		return err
	}
	_, err := db.Exec("DELETE FROM users WHERE username = $1 AND status = 'pending';", targetUser)
	if err == nil {
		details := fmt.Sprintf("Rejected and purged user registration request from queue: %s", targetUser)
		_ = ExecuteInsertAuditLog(operator, "REJECT_USER_REGISTRATION", details)
	}
	return err
}

func FetchSystemUsers(searchFilter string) ([][]string, error) {
	directoryData := [][]string{}

	baseQuery := `
        SELECT username, role, COALESCE(last_login::text, 'Never Logged In'), status, COALESCE(system_id, 'UNLOCKED'), COALESCE(avatar_selection, 'default') 
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
		var uName, uRole, lastLog, uStatus, sysID, avatar string
		if err := rows.Scan(&uName, &uRole, &lastLog, &uStatus, &sysID, &avatar); err == nil {
			directoryData = append(directoryData, []string{uName, uRole, lastLog, uStatus, sysID, avatar})
		}
	}
	return directoryData, nil
}

func ExecuteUpdateUserRole(operator, username, newRole string) error {
	_, err := db.Exec("UPDATE users SET role = $1 WHERE username = $2;", newRole, username)
	if err == nil {
		details := fmt.Sprintf("Altered user profile authorization level: target account=%s changed to role=%s", username, newRole)
		_ = ExecuteInsertAuditLog(operator, "UPDATE_USER_ROLE", details)
	}
	return err
}

func ExecuteResetSystemLock(operator, username string) error {
	_, err := db.Exec("UPDATE users SET system_id = NULL WHERE username = $1;", username)
	if err == nil {
		details := fmt.Sprintf("Cleared terminal physical workstation locking reference context for: %s", username)
		_ = ExecuteInsertAuditLog(operator, "RESET_SYSTEM_LOCK", details)
	}
	return err
}

func ExecuteToggleUserSuspend(operator, username string, shouldSuspend bool) error {
	targetStatus := "approved"
	actionWord := "Reactivated"
	logType := "REACTIVATE_USER"
	if shouldSuspend {
		targetStatus = "suspended"
		actionWord = "Suspended"
		logType = "SUSPEND_USER"
	}
	_, err := db.Exec("UPDATE users SET status = $1 WHERE username = $2;", targetStatus, username)
	if err == nil {
		details := fmt.Sprintf("%s user profile account space context: target account=%s", actionWord, username)
		_ = ExecuteInsertAuditLog(operator, logType, details)
	}
	return err
}

func ExecuteResetUserPassword(operator, username, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to process password re-encryption sequence")
	}
	_, err = db.Exec("UPDATE users SET password = $1 WHERE username = $2;", string(hashedPassword), username)
	if err == nil {
		details := fmt.Sprintf("Executed administrative security password reset hash overwrite targeting profile: %s", username)
		_ = ExecuteInsertAuditLog(operator, "RESET_USER_PASSWORD", details)
	}
	return err
}

func ExecuteTerminateUserSession(operator, username string) error {
	_, err := db.Exec("UPDATE users SET system_id = NULL, last_login = NULL WHERE username = $1;", username)
	if err == nil {
		details := fmt.Sprintf("Forced live session token teardown and dropped system biometric fingerprints for operator user profile: %s", username)
		_ = ExecuteInsertAuditLog(operator, "TERMINATE_USER_SESSION", details)
	}
	return err
}

func ExecuteUpdateUserAvatar(operator, username, chosenAvatar string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}
	_, err := db.Exec("UPDATE users SET avatar_selection = $1 WHERE username = $2;", chosenAvatar, username)
	if err == nil {
		details := fmt.Sprintf("Admin altered user graphic context: user %s set to [%s]", username, chosenAvatar)
		_ = ExecuteInsertAuditLog(operator, "UPDATE_USER_AVATAR", details)
	}
	return err
}

// ==============================================================================
// VMPROTECT RUNTIME SECURITY API INSPECTION HELPERS
// ==============================================================================

// IsVMProtected checks whether the binary has been processed by VMProtect
func IsVMProtected() bool {
	return bool(C.VMProtectIsProtected())
}

// IsDebuggerPresent queries the VMProtect engine to detect active debuggers
func IsDebuggerPresent() bool {
	return bool(C.VMProtectIsDebuggerPresent(C.bool(true)))
}

// IsValidImageCRC verifies the integrity of the binary against memory patching/tampering
func IsValidImageCRC() bool {
	return bool(C.VMProtectIsValidImageCRC())
}

// IsVirtualMachinePresent checks if application is running in a virtualized guest VM
func IsVirtualMachinePresent() bool {
	return bool(C.VMProtectIsVirtualMachinePresent())
}

