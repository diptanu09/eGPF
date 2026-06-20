package db

import (
	"database/sql"

	_ "github.com/lib/pq"
)

// Shared across all files under 'package db' automatically
var db *sql.DB

func InitDatabase() error {
	connStr := "host=10.47.240.169 port=5432 user=egpf_app_user password='P@ssw()rd123' dbname='AsstPro' sslmode=disable search_path=agartala"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	return db.Ping()
}

func FetchLatestAppVersion() (string, string, error) {
	var latestVersion, downloadPath string
	err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'latest_version';").Scan(&latestVersion)
	if err != nil {
		return "2.2.0.1", "", nil
	}
	err = db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'download_path';").Scan(&downloadPath)
	if err != nil {
		return "2.2.0.1", "", nil
	}
	return latestVersion, downloadPath, nil
}

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
