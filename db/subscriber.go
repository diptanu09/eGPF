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

	logDetails := fmt.Sprintf("Updated series %s, account %s. Set Designation=%s, Mobile=%s", newSeries, newAccount, newDesignation, newMobile)
	if auditErr := ExecuteInsertAuditLog(operator, "UPDATE_RECORD", logDetails); auditErr != nil {
		return fmt.Errorf("audit checkpoint failure: %w", auditErr)
	}

	return tx.Commit()
}

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

func FetchDDODetails(ddoCode string) (map[string]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle uninitialized")
	}

	queryStr := `
		SELECT 
			COALESCE(d.ddo_code, ''), 
			COALESCE(d.ddo_desg, 'N/A'), 
			COALESCE(d.ddo_phone, 'N/A'), 
			COALESCE(d.ddo_email, 'N/A'), 
			COALESCE(d.ddo_tres_code, 'N/A'), 
			COALESCE(d.vlc_ddo_code, 'N/A'),
			COALESCE(l.pin, COALESCE(d.pin, 'N/A')) AS security_pin
		FROM agartala.mm_ddo d
		LEFT JOIN agartala.ddo_login l ON d.ddo_code = l.ddo_code
		WHERE d.ddo_code = $1;`

	var code, desg, phone, email, tresCode, vlcCode, pin string
	err := db.QueryRow(queryStr, ddoCode).Scan(&code, &desg, &phone, &email, &tresCode, &vlcCode, &pin)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no DDO master configuration registry found matching code: %s", ddoCode)
		}
		return nil, err
	}

	result := map[string]string{
		"ddo_code":      code,
		"ddo_desg":      desg,
		"ddo_phone":     phone,
		"ddo_email":     email,
		"ddo_tres_code": tresCode,
		"vlc_ddo_code":  vlcCode,
		"pin":           pin,
	}
	return result, nil
}

func FetchAllDDOMasterProfiles(filterCode string) ([][]string, error) {
	directoryRows := [][]string{}
	if db == nil {
		return directoryRows, fmt.Errorf("database session handle context uninitialized")
	}

	baseQuery := `
		SELECT 
			COALESCE(d.ddo_code, ''), 
			COALESCE(d.ddo_desg, 'N/A'), 
			COALESCE(d.ddo_phone, 'N/A'), 
			COALESCE(d.ddo_email, 'N/A'), 
			COALESCE(d.ddo_tres_code, 'N/A'), 
			COALESCE(d.vlc_ddo_code, 'N/A'),
			COALESCE(l.pin, COALESCE(d.pin, 'N/A')) AS security_pin
		FROM agartala.mm_ddo d
		LEFT JOIN agartala.ddo_login l ON d.ddo_code = l.ddo_code`

	var rows *sql.Rows
	var err error

	if filterCode != "" {
		queryStr := baseQuery + " WHERE d.ddo_code LIKE $1 ORDER BY d.ddo_code ASC LIMIT 100;"
		rows, err = db.Query(queryStr, "%"+filterCode+"%")
	} else {
		queryStr := baseQuery + " ORDER BY d.ddo_code ASC LIMIT 100;"
		rows, err = db.Query(queryStr)
	}

	if err != nil {
		return directoryRows, err
	}
	defer rows.Close()

	for rows.Next() {
		var code, desg, phone, email, tres, vlc, pin string
		if err := rows.Scan(&code, &desg, &phone, &email, &tres, &vlc, &pin); err == nil {
			directoryRows = append(directoryRows, []string{code, desg, phone, email, tres, vlc, pin})
		}
	}
	return directoryRows, nil
}

func ExecuteResetSubscriberPIN(operator, seriesID, accountNo, newPIN string) error {
	if db == nil {
		return fmt.Errorf("database instance handle uninitialized")
	}

	queryStr := "UPDATE agartala.subscriber_login_details SET pin = $1 WHERE series_id = $2 AND account_no = $3;"
	_, err := db.Exec(queryStr, newPIN, seriesID, accountNo)
	if err == nil {
		details := fmt.Sprintf("Overwrote subscriber gateway access PIN targeting Series: %s, Account Number: %s", seriesID, accountNo)
		_ = ExecuteInsertAuditLog(operator, "RESET_SUBSCRIBER_PIN", details)
	}
	return err
}

func ExecuteCreateNewSubscriber(operator, seriesID, accountNo, initialPIN string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	var exists bool
	checkQuery := "SELECT EXISTS(SELECT 1 FROM agartala.subscriber_login_details WHERE series_id = $1 AND account_no = $2);"
	err := db.QueryRow(checkQuery, seriesID, accountNo).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("account mapping registration violation: subscriber account %s already exists under series %s", accountNo, seriesID)
	}

	insertQuery := "INSERT INTO agartala.subscriber_login_details (series_id, account_no, pin) VALUES ($1, $2, $3);"
	_, err = db.Exec(insertQuery, seriesID, accountNo, initialPIN)
	if err == nil {
		details := fmt.Sprintf("Created new master subscriber account mapping. Series: %s, Account No: %s", seriesID, accountNo)
		_ = ExecuteInsertAuditLog(operator, "CREATE_SUBSCRIBER_ACCOUNT", details)
	}
	return err
}

func FetchApplicationPermissions() (map[string]bool, error) {
	perms := map[string]bool{
		"operator_can_write":      true,
		"operator_can_assign_pin": false,
		"user_can_view_ddo":       true,
	}
	if db == nil {
		return perms, nil
	}

	rows, err := db.Query("SELECT config_key, config_value FROM agartala.system_config WHERE config_key LIKE 'perm_%';")
	if err != nil {
		return perms, nil
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			trimmedKey := k[5:] // Trim away 'perm_' prefix
			perms[trimmedKey] = (v == "true" || v == "1")
		}
	}
	return perms, nil
}

func ExecuteSavePermissionToggle(operator, targetScope, ruleKey string, allowed bool) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	dbKey := fmt.Sprintf("perm_%s_%s", targetScope, ruleKey)
	if targetScope == "role" {
		dbKey = "perm_" + ruleKey
	}

	dbValue := "false"
	if allowed {
		dbValue = "true"
	}

	var exists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.system_config WHERE config_key = $1);", dbKey).Scan(&exists)

	var err error
	if exists {
		_, err = db.Exec("UPDATE agartala.system_config SET config_value = $1 WHERE config_key = $2;", dbValue, dbKey)
	} else {
		_, err = db.Exec("INSERT INTO agartala.system_config (config_key, config_value) VALUES ($1, $2);", dbKey, dbValue)
	}

	if err == nil {
		details := fmt.Sprintf("Updated granular authorization rule: target %s context %s set to [%s]", targetScope, ruleKey, dbValue)
		_ = ExecuteInsertAuditLog(operator, "ALTER_ACCESS_PERMISSIONS", details)
	}
	return err
}

func EvaluateUserPermission(username, role, ruleKey string, defaultFallback bool) bool {
	if role == "admin" {
		return true // Administrator overrides everything
	}
	if db == nil {
		return defaultFallback
	}

	var userVal string
	userKey := fmt.Sprintf("perm_%s_%s", username, ruleKey)
	err := db.QueryRow("SELECT config_value FROM agartala.system_config WHERE config_key = $1;", userKey).Scan(&userVal)
	if err == nil {
		return userVal == "true"
	}

	var roleVal string
	roleKeyName := fmt.Sprintf("perm_%s_%s", role, ruleKey)
	err = db.QueryRow("SELECT config_value FROM agartala.system_config WHERE config_key = $1;", roleKeyName).Scan(&roleVal)
	if err == nil {
		return roleVal == "true"
	}

	return defaultFallback
}

// FIXED & ADDED: Atomically update master designation, phone, email, and gate PIN context
func ExecuteUpdateDDOProfile(operator, ddoCode, designation, phone, email, pin string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Synchronize details back into central master configuration table
	_, err = tx.Exec(`
		UPDATE agartala.mm_ddo 
		SET ddo_desg = $1, ddo_phone = $2, ddo_email = $3 
		WHERE ddo_code = $4;`,
		designation, phone, email, ddoCode,
	)
	if err != nil {
		return fmt.Errorf("master configuration profile layer update failure: %w", err)
	}

	// 2. Evaluate and manage security system login map settings
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM agartala.ddo_login WHERE ddo_code = $1);", ddoCode).Scan(&exists)
	if err != nil {
		return err
	}

	if pin != "" {
		if exists {
			_, err = tx.Exec("UPDATE agartala.ddo_login SET pin = $1 WHERE ddo_code = $2;", pin, ddoCode)
		} else {
			_, err = tx.Exec("INSERT INTO agartala.ddo_login (pin, ddo_code) VALUES ($1, $2);", pin, ddoCode)
		}
		if err != nil {
			return fmt.Errorf("gate access PIN persistence framework deployment breakdown: %w", err)
		}
	}

	// 3. Append track log trails
	details := fmt.Sprintf("Updated DDO profile records for %s: Desg=[%s], Phone=[%s], Email=[%s], PIN Context Updated", ddoCode, designation, phone, email)
	if auditErr := ExecuteInsertAuditLog(operator, "UPDATE_DDO_PROFILE_DATA", details); auditErr != nil {
		return fmt.Errorf("audit logging session failure: %w", auditErr)
	}

	return tx.Commit()
}
