package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ServiceRequest represents a user-raised change request
type ServiceRequest struct {
	ID                int64      `json:"id"`
	RequesterUsername string     `json:"requester_username"`
	RequestType       string     `json:"request_type"` // CREATE_PIN, UPDATE_SUBSCRIBER, CREATE_SUBSCRIBER, CREATE_DDO, UPDATE_DDO
	TargetEntity      string     `json:"target_entity"`
	RequestDetails    string     `json:"request_details"` // JSON string
	Status            string     `json:"status"`          // PENDING, APPROVED, REJECTED
	ReviewerUsername  *string    `json:"reviewer_username"`
	ReviewerRemarks   *string    `json:"reviewer_remarks"`
	CreatedAt         time.Time  `json:"created_at"`
	ReviewedAt        *time.Time `json:"reviewed_at"`
}

// RequestPayloadDetails parses individual JSON payload attributes for different request types
type RequestPayloadDetails struct {
	SeriesID       string `json:"series_id,omitempty"`
	AccountNo      string `json:"account_no,omitempty"`
	SubscriberName string `json:"subscriber_name,omitempty"`
	EmployeeCode   string `json:"employee_code,omitempty"`
	Designation    string `json:"designation,omitempty"`
	MobileNo       string `json:"mobile_no,omitempty"`
	DOB            string `json:"dob,omitempty"`
	DDOCode        string `json:"ddo_code,omitempty"`
	PIN            string `json:"pin,omitempty"`
	PhoneNo        string `json:"phone_no,omitempty"`
	Email          string `json:"email,omitempty"`
	TreasuryCode   string `json:"treasury_code,omitempty"`
	TreasuryName   string `json:"treasury_name,omitempty"`
	VLCCode        string `json:"vlc_code,omitempty"`
	ReasonRemarks  string `json:"reason_remarks,omitempty"`
}

// EnsureServiceRequestsTableExists auto-creates the service_requests table and indexes if missing
func EnsureServiceRequestsTableExists() error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	createTableQuery := `
		CREATE TABLE IF NOT EXISTS service_requests (
			id BIGSERIAL PRIMARY KEY,
			requester_username VARCHAR(100) NOT NULL,
			request_type VARCHAR(50) NOT NULL,
			target_entity VARCHAR(150) NOT NULL,
			request_details TEXT NOT NULL,
			status VARCHAR(20) DEFAULT 'PENDING',
			reviewer_username VARCHAR(100),
			reviewer_remarks TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			reviewed_at TIMESTAMP WITH TIME ZONE
		);
		CREATE INDEX IF NOT EXISTS idx_service_requests_status ON service_requests(status);
		CREATE INDEX IF NOT EXISTS idx_service_requests_requester ON service_requests(requester_username);
		CREATE INDEX IF NOT EXISTS idx_service_requests_created ON service_requests(created_at DESC);
	`
	_, err := db.Exec(createTableQuery)
	if err != nil {
		log.Printf("Warning: Failed to ensure service_requests table: %v", err)
		return err
	}
	return nil
}

// ExecuteSubmitServiceRequest submits a new service request from a normal user
func ExecuteSubmitServiceRequest(requesterUsername, requestType, targetEntity string, payload RequestPayloadDetails) error {
	if db == nil {
		return fmt.Errorf("database session handle context uninitialized")
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize request payload: %w", err)
	}

	query := `
		INSERT INTO service_requests (
			requester_username, request_type, target_entity, request_details, status, created_at
		) VALUES ($1, $2, $3, $4, 'PENDING', NOW());
	`
	_, err = db.Exec(query, requesterUsername, requestType, targetEntity, string(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to insert service request: %w", err)
	}

	auditDetails := fmt.Sprintf("Raised Service Request [%s] for target: %s", requestType, targetEntity)
	_ = ExecuteInsertAuditLog(requesterUsername, "RAISE_SERVICE_REQUEST", auditDetails)

	return nil
}

// FetchPendingServiceRequestCount returns the total count of pending service requests for the Admin badge
func FetchPendingServiceRequestCount() int {
	if db == nil {
		return 0
	}
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM service_requests WHERE status = 'PENDING';").Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// FetchUserPendingServiceRequestCount returns the count of pending requests raised by a specific user
func FetchUserPendingServiceRequestCount(username string) int {
	if db == nil {
		return 0
	}
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM service_requests WHERE requester_username = $1 AND status = 'PENDING';", username).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// FetchServiceRequests fetches requests based on status and requester filters
func FetchServiceRequests(filterStatus, requesterUsername string, limit int) ([]ServiceRequest, error) {
	var requests []ServiceRequest
	if db == nil {
		return requests, fmt.Errorf("database handle uninitialized")
	}

	if limit <= 0 {
		limit = 100
	}

	var query string
	var rows *sql.Rows
	var err error

	baseQuery := `
		SELECT 
			id, requester_username, request_type, target_entity, request_details, status, 
			reviewer_username, reviewer_remarks, created_at, reviewed_at
		FROM service_requests`

	if requesterUsername != "" {
		if filterStatus != "" && filterStatus != "ALL" {
			query = baseQuery + " WHERE requester_username = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3;"
			rows, err = db.Query(query, requesterUsername, filterStatus, limit)
		} else {
			query = baseQuery + " WHERE requester_username = $1 ORDER BY created_at DESC LIMIT $2;"
			rows, err = db.Query(query, requesterUsername, limit)
		}
	} else {
		if filterStatus != "" && filterStatus != "ALL" {
			query = baseQuery + " WHERE status = $1 ORDER BY created_at DESC LIMIT $2;"
			rows, err = db.Query(query, filterStatus, limit)
		} else {
			query = baseQuery + " ORDER BY created_at DESC LIMIT $1;"
			rows, err = db.Query(query, limit)
		}
	}

	if err != nil {
		return requests, err
	}
	defer rows.Close()

	for rows.Next() {
		var r ServiceRequest
		err := rows.Scan(
			&r.ID, &r.RequesterUsername, &r.RequestType, &r.TargetEntity, &r.RequestDetails, &r.Status,
			&r.ReviewerUsername, &r.ReviewerRemarks, &r.CreatedAt, &r.ReviewedAt,
		)
		if err == nil {
			requests = append(requests, r)
		}
	}

	return requests, nil
}

// ExecuteApproveServiceRequest approves a request and applies its changes into the database atomically
func ExecuteApproveServiceRequest(reviewerUsername string, requestID int64, remarks string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	// 1. Fetch the request record
	var r ServiceRequest
	query := `
		SELECT id, requester_username, request_type, target_entity, request_details, status
		FROM service_requests WHERE id = $1;
	`
	err := db.QueryRow(query, requestID).Scan(
		&r.ID, &r.RequesterUsername, &r.RequestType, &r.TargetEntity, &r.RequestDetails, &r.Status,
	)
	if err != nil {
		return fmt.Errorf("request not found: %w", err)
	}

	if r.Status != "PENDING" {
		return fmt.Errorf("request is already processed with status: %s", r.Status)
	}

	// 2. Parse payload
	var payload RequestPayloadDetails
	if err := json.Unmarshal([]byte(r.RequestDetails), &payload); err != nil {
		return fmt.Errorf("failed to parse request payload: %w", err)
	}

	// 3. Begin Transaction to apply changes and update request status
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 4. Execute database mutations according to RequestType
	switch r.RequestType {
	case "CREATE_PIN", "RESET_PIN":
		if payload.SeriesID != "" && payload.AccountNo != "" && payload.PIN != "" {
			_, err = tx.Exec(
				"UPDATE subscriber_login_details SET pin = $1 WHERE series_id = $2 AND account_no = $3;",
				payload.PIN, payload.SeriesID, payload.AccountNo,
			)
			if err != nil {
				return fmt.Errorf("failed to update subscriber PIN: %w", err)
			}
		} else if payload.DDOCode != "" && payload.PIN != "" {
			var exists bool
			_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM ddo_login WHERE ddo_code = $1);", payload.DDOCode).Scan(&exists)
			if exists {
				_, err = tx.Exec("UPDATE ddo_login SET pin = $1 WHERE ddo_code = $2;", payload.PIN, payload.DDOCode)
			} else {
				_, err = tx.Exec("INSERT INTO ddo_login (pin, ddo_code) VALUES ($1, $2);", payload.PIN, payload.DDOCode)
			}
			if err != nil {
				return fmt.Errorf("failed to update DDO PIN: %w", err)
			}
		} else if payload.TreasuryCode != "" && payload.PIN != "" {
			var exists bool
			_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.treasury_login WHERE tres_code = $1);", payload.TreasuryCode).Scan(&exists)
			if exists {
				_, err = tx.Exec("UPDATE agartala.treasury_login SET pin = $1 WHERE tres_code = $2;", payload.PIN, payload.TreasuryCode)
			} else {
				_, err = tx.Exec("INSERT INTO agartala.treasury_login (tres_code, pin) VALUES ($1, $2);", payload.TreasuryCode, payload.PIN)
			}
			if err != nil {
				return fmt.Errorf("failed to update Treasury PIN: %w", err)
			}
		} else {
			return fmt.Errorf("insufficient parameters for PIN update")
		}

	case "UPDATE_SUBSCRIBER":
		if payload.SeriesID == "" || payload.AccountNo == "" {
			return fmt.Errorf("missing required Series ID or Account Number")
		}

		if payload.PIN != "" {
			_, err = tx.Exec(
				"UPDATE subscriber_login_details SET pin = $1 WHERE series_id = $2 AND account_no = $3;",
				payload.PIN, payload.SeriesID, payload.AccountNo,
			)
			if err != nil {
				return fmt.Errorf("failed to update subscriber login details: %w", err)
			}
		}

		var exists bool
		_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM subscriber_details WHERE series_id = $1 AND account_no = $2);", payload.SeriesID, payload.AccountNo).Scan(&exists)
		if exists {
			_, err = tx.Exec(`
				UPDATE subscriber_details 
				SET mobile_no = COALESCE(NULLIF($1, ''), mobile_no),
				    designation = COALESCE(NULLIF($2, ''), designation),
				    subscriber_name = COALESCE(NULLIF($3, ''), subscriber_name),
				    ddo_code = COALESCE(NULLIF($4, ''), ddo_code)
				WHERE series_id = $5 AND account_no = $6;`,
				payload.MobileNo, payload.Designation, payload.SubscriberName, payload.DDOCode, payload.SeriesID, payload.AccountNo,
			)
		} else {
			_, err = tx.Exec(`
				INSERT INTO subscriber_details (series_id, account_no, mobile_no, designation, subscriber_name, ddo_code) 
				VALUES ($1, $2, $3, $4, $5, $6);`,
				payload.SeriesID, payload.AccountNo, payload.MobileNo, payload.Designation, payload.SubscriberName, payload.DDOCode,
			)
		}
		if err != nil {
			return fmt.Errorf("failed to update subscriber details: %w", err)
		}

	case "CREATE_SUBSCRIBER":
		if payload.SeriesID == "" || payload.AccountNo == "" {
			return fmt.Errorf("series and account number are mandatory")
		}

		var exists bool
		_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM subscriber_login_details WHERE series_id = $1 AND account_no = $2);", payload.SeriesID, payload.AccountNo).Scan(&exists)
		if exists {
			return fmt.Errorf("subscriber account %s under series %s already exists", payload.AccountNo, payload.SeriesID)
		}

		_, err = tx.Exec(
			"INSERT INTO subscriber_login_details (series_id, account_no, pin) VALUES ($1, $2, $3);",
			payload.SeriesID, payload.AccountNo, payload.PIN,
		)
		if err != nil {
			return fmt.Errorf("failed to insert into subscriber_login_details: %w", err)
		}

		if payload.SubscriberName != "" || payload.Designation != "" || payload.MobileNo != "" || payload.DDOCode != "" {
			_, err = tx.Exec(`
				INSERT INTO subscriber_details (series_id, account_no, subscriber_name, designation, mobile_no, employee_code, ddo_code)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (series_id, account_no) DO UPDATE 
				SET subscriber_name = EXCLUDED.subscriber_name,
				    designation = EXCLUDED.designation,
				    mobile_no = EXCLUDED.mobile_no,
				    ddo_code = EXCLUDED.ddo_code;`,
				payload.SeriesID, payload.AccountNo, payload.SubscriberName, payload.Designation, payload.MobileNo, payload.EmployeeCode, payload.DDOCode,
			)
			if err != nil {
				return fmt.Errorf("failed to insert into subscriber_details: %w", err)
			}
		}

	case "CREATE_DDO":
		if payload.DDOCode == "" {
			return fmt.Errorf("DDO Code is required")
		}

		var exists bool
		_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.mm_ddo WHERE ddo_code = $1);", payload.DDOCode).Scan(&exists)
		if exists {
			return fmt.Errorf("DDO master profile %s already exists", payload.DDOCode)
		}

		_, err = tx.Exec(`
			INSERT INTO agartala.mm_ddo (ddo_code, ddo_desg, ddo_phone, ddo_email, ddo_tres_code, vlc_ddo_code)
			VALUES ($1, $2, $3, $4, $5, $6);`,
			payload.DDOCode, payload.Designation, payload.PhoneNo, payload.Email, payload.TreasuryCode, payload.VLCCode,
		)
		if err != nil {
			return fmt.Errorf("failed to create DDO master profile: %w", err)
		}

		if payload.PIN != "" {
			_, err = tx.Exec("INSERT INTO agartala.ddo_login (ddo_code, pin) VALUES ($1, $2);", payload.DDOCode, payload.PIN)
			if err != nil {
				return fmt.Errorf("failed to create DDO login PIN: %w", err)
			}
		}

	case "UPDATE_DDO":
		if payload.DDOCode == "" {
			return fmt.Errorf("DDO Code is required")
		}

		_, err = tx.Exec(`
			UPDATE agartala.mm_ddo 
			SET ddo_desg = COALESCE(NULLIF($1, ''), ddo_desg),
			    ddo_phone = COALESCE(NULLIF($2, ''), ddo_phone),
			    ddo_email = COALESCE(NULLIF($3, ''), ddo_email),
			    ddo_tres_code = COALESCE(NULLIF($4, ''), ddo_tres_code),
			    vlc_ddo_code = COALESCE(NULLIF($5, ''), vlc_ddo_code)
			WHERE ddo_code = $6;`,
			payload.Designation, payload.PhoneNo, payload.Email, payload.TreasuryCode, payload.VLCCode, payload.DDOCode,
		)
		if err != nil {
			return fmt.Errorf("failed to update DDO master profile: %w", err)
		}

		if payload.PIN != "" {
			var exists bool
			_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.ddo_login WHERE ddo_code = $1);", payload.DDOCode).Scan(&exists)
			if exists {
				_, err = tx.Exec("UPDATE agartala.ddo_login SET pin = $1 WHERE ddo_code = $2;", payload.PIN, payload.DDOCode)
			} else {
				_, err = tx.Exec("INSERT INTO agartala.ddo_login (pin, ddo_code) VALUES ($1, $2);", payload.PIN, payload.DDOCode)
			}
			if err != nil {
				return fmt.Errorf("failed to set DDO login PIN: %w", err)
			}
		}

	case "CREATE_TREASURY":
		if payload.TreasuryCode == "" {
			return fmt.Errorf("Treasury Code is required")
		}

		var exists bool
		_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.mm_treasury WHERE tres_code = $1);", payload.TreasuryCode).Scan(&exists)
		if exists {
			return fmt.Errorf("Treasury %s already exists", payload.TreasuryCode)
		}

		_, err = tx.Exec(`
			INSERT INTO agartala.mm_treasury (tres_code, tres_name, vlc_tres_code)
			VALUES ($1, $2, $3);`,
			payload.TreasuryCode, payload.TreasuryName, payload.VLCCode,
		)
		if err != nil {
			return fmt.Errorf("failed to create Treasury master profile: %w", err)
		}

		if payload.PIN != "" {
			_, err = tx.Exec("INSERT INTO agartala.treasury_login (tres_code, pin) VALUES ($1, $2);", payload.TreasuryCode, payload.PIN)
			if err != nil {
				return fmt.Errorf("failed to create Treasury login PIN: %w", err)
			}
		}

	case "UPDATE_TREASURY":
		if payload.TreasuryCode == "" {
			return fmt.Errorf("Treasury Code is required")
		}

		_, err = tx.Exec(`
			UPDATE agartala.mm_treasury 
			SET tres_name = COALESCE(NULLIF($1, ''), tres_name),
			    vlc_tres_code = COALESCE(NULLIF($2, ''), vlc_tres_code)
			WHERE tres_code = $3;`,
			payload.TreasuryName, payload.VLCCode, payload.TreasuryCode,
		)
		if err != nil {
			return fmt.Errorf("failed to update Treasury master profile: %w", err)
		}

		if payload.PIN != "" {
			var exists bool
			_ = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.treasury_login WHERE tres_code = $1);", payload.TreasuryCode).Scan(&exists)
			if exists {
				_, err = tx.Exec("UPDATE agartala.treasury_login SET pin = $1 WHERE tres_code = $2;", payload.PIN, payload.TreasuryCode)
			} else {
				_, err = tx.Exec("INSERT INTO agartala.treasury_login (tres_code, pin) VALUES ($1, $2);", payload.PIN, payload.TreasuryCode)
			}
			if err != nil {
				return fmt.Errorf("failed to set Treasury login PIN: %w", err)
			}
		}

	default:
		return fmt.Errorf("unknown service request type: %s", r.RequestType)
	}

	// 5. Mark the request as APPROVED
	approvalQuery := `
		UPDATE service_requests 
		SET status = 'APPROVED', reviewer_username = $1, reviewer_remarks = $2, reviewed_at = NOW() 
		WHERE id = $3;
	`
	_, err = tx.Exec(approvalQuery, reviewerUsername, strings.TrimSpace(remarks), requestID)
	if err != nil {
		return fmt.Errorf("failed to mark request as approved: %w", err)
	}

	// 6. Record in Audit Log
	auditDetails := fmt.Sprintf("Approved Service Request #%d [%s] for target: %s (Raised by %s)", requestID, r.RequestType, r.TargetEntity, r.RequesterUsername)
	if auditErr := ExecuteInsertAuditLog(reviewerUsername, "APPROVE_SERVICE_REQUEST", auditDetails); auditErr != nil {
		return fmt.Errorf("audit log failure: %w", auditErr)
	}

	return tx.Commit()
}

// ExecuteRejectServiceRequest marks a service request as REJECTED with reviewer remarks
func ExecuteRejectServiceRequest(reviewerUsername string, requestID int64, remarks string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	query := `
		UPDATE service_requests 
		SET status = 'REJECTED', reviewer_username = $1, reviewer_remarks = $2, reviewed_at = NOW() 
		WHERE id = $3 AND status = 'PENDING';
	`
	res, err := db.Exec(query, reviewerUsername, strings.TrimSpace(remarks), requestID)
	if err != nil {
		return fmt.Errorf("failed to reject service request: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("request #%d is either not found or already processed", requestID)
	}

	auditDetails := fmt.Sprintf("Rejected Service Request #%d (Remarks: %s)", requestID, remarks)
	_ = ExecuteInsertAuditLog(reviewerUsername, "REJECT_SERVICE_REQUEST", auditDetails)

	return nil
}
