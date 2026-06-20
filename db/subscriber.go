package db

import (
	"database/sql"
	"fmt"
)

// ExecuteInsertAuditLog writes internal operational logs to the system database
func ExecuteInsertAuditLog(operator, action, details string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}
	query := "INSERT INTO system_audit_logs (operator_username, action_type, details, action_timestamp) VALUES ($1, $2, $3, NOW());"
	_, err := db.Exec(query, operator, action, details)
	return err
}

func FetchSeriesDropdownOptions() ([]string, map[string]string, error) {
	query := "SELECT series_id::text, series_name::text FROM mm_series ORDER BY series_id ASC;"
	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var options []string
	mapping := make(map[string]string)

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err == nil {
			displayStr := fmt.Sprintf("%s - %s", id, name)
			options = append(options, displayStr)
			mapping[displayStr] = id
		}
	}
	return options, mapping, nil
}

func FetchSubscriberRegistry(seriesFilter, accountFilter, nameFilter string, forceAll bool) ([][]string, error) {
	gridData := [][]string{{
		"Series ID & Name", "Account Number", "Subscriber Name",
		"Employee Code", "Beneficiary Code", "Designation",
		"Mobile No", "Date of Birth", "DDO Code", "PIN",
	}}

	var rows *sql.Rows
	var err error

	if seriesFilter == "" && accountFilter == "" && nameFilter == "" && !forceAll {
		return gridData, nil
	}

	baseQuery := `
		SELECT 
			COALESCE(s.series_id::text, ''), COALESCE(m.series_name::text, 'UNKNOWN'), COALESCE(s.account_no::text, ''), 
			COALESCE(d.subscriber_name::text, 'N/A'), COALESCE(d.employee_code::text, 'N/A'), COALESCE(d.beneficiary_code::text, 'N/A'),
			COALESCE(d.designation::text, 'N/A'), COALESCE(NULLIF(d.mobile_no::text, ''), NULLIF(s.mobile::text, ''), 'N/A') AS final_mobile,
			COALESCE(d.date_of_birth::text, 'N/A'), COALESCE(d.ddo_code::text, 'N/A'), COALESCE(s.pin::text, '') 
		FROM subscriber_login_details s
		LEFT JOIN subscriber_details d ON s.series_id = d.series_id AND s.account_no = d.account_no
		LEFT JOIN mm_series m ON s.series_id = m.series_id`

	if forceAll {
		queryStr := baseQuery + " ORDER BY s.account_no LIMIT 150;"
		rows, err = db.Query(queryStr)
	} else {
		queryStr := baseQuery + " WHERE s.series_id LIKE $1 AND s.account_no LIKE $2 AND d.subscriber_name ILIKE $3 ORDER BY s.account_no LIMIT 150;"
		rows, err = db.Query(queryStr, "%"+seriesFilter+"%", "%"+accountFilter+"%", "%"+nameFilter+"%")
	}

	if err != nil {
		return gridData, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, accNo, subName, empCode, benCode, desig, mobile, dob, ddo, pin string
		err := rows.Scan(&id, &name, &accNo, &subName, &empCode, &benCode, &desig, &mobile, &dob, &ddo, &pin)
		if err == nil {
			displaySeries := id
			if name != "" && name != "UNKNOWN" {
				displaySeries = fmt.Sprintf("%s - %s", id, name)
			}
			gridData = append(gridData, []string{
				displaySeries, accNo, subName, empCode, benCode, desig, mobile, dob, ddo, pin,
			})
		}
	}
	return gridData, nil
}

func ExecuteInsertRecord(series, account, pin string) error {
	_, err := db.Exec("INSERT INTO subscriber_login_details (series_id, account_no, pin) VALUES ($1, $2, $3);", series, account, pin)
	return err
}

// MODIFIED: Accepts operator string parameter and writes system audit trails automatically
func ExecuteUpdateRecord(operator, newSeries, newAccount, newPin, newMobile, newDesignation, oldSeries, oldAccount string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE subscriber_login_details SET series_id = $1, account_no = $2, pin = $3 
		WHERE series_id = $4 AND account_no = $5;`,
		newSeries, newAccount, newPin, oldSeries, oldAccount,
	)
	if err != nil {
		return err
	}

	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM subscriber_details WHERE series_id = $1 AND account_no = $2);", oldSeries, oldAccount).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		_, err = tx.Exec(`
			UPDATE subscriber_details SET series_id = $1, account_no = $2, mobile_no = $3, designation = $4 
			WHERE series_id = $5 AND account_no = $6;`,
			newSeries, newAccount, newMobile, newDesignation, oldSeries, oldAccount,
		)
	} else {
		_, err = tx.Exec(`
			INSERT INTO subscriber_details (series_id, account_no, mobile_no, designation) VALUES ($1, $2, $3, $4);`,
			newSeries, newAccount, newMobile, newDesignation,
		)
	}
	if err != nil {
		return err
	}

	// Queue audit entry trace inside transactional scope boundaries
	logDetails := fmt.Sprintf("Updated series %s, account %s. Set Designation=%s, Mobile=%s", newSeries, newAccount, newDesignation, newMobile)
	if auditErr := ExecuteInsertAuditLog(operator, "UPDATE_RECORD", logDetails); auditErr != nil {
		return fmt.Errorf("audit checkpoint failure: %w", auditErr)
	}

	return tx.Commit()
}

// MODIFIED: Tracks dropping actions to prevent untraceable information deletions
func ExecuteDeleteRecord(operator, series, account string) error {
	_, err := db.Exec("DELETE FROM subscriber_login_details WHERE series_id = $1 AND account_no = $2;", series, account)
	if err == nil {
		logDetails := fmt.Sprintf("Deleted profile mapping targeting Series ID: %s, Account No: %s", series, account)
		_ = ExecuteInsertAuditLog(operator, "DELETE_RECORD", logDetails)
	}
	return err
}

func ExecuteInsertSeries(seriesID string, seriesName string) error {
	if db == nil {
		return fmt.Errorf("database session handle is uninitialized")
	}
	query := "INSERT INTO mm_series (series_id, series_name) VALUES ($1, $2);"
	_, err := db.Exec(query, seriesID, seriesName)
	if err != nil {
		return fmt.Errorf("failed to register new system master series: %w", err)
	}
	return nil
}
