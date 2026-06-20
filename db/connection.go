package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitializeDatabase() {
	// 🟢 UPDATE THESE CREDENTIALS TO MATCH YOUR LOCAL POSTGRES SETUP
	connStr := "host=10.47.240.169 port=5432 user=egpf_app_user password=P@ssw()rd123 dbname=AsstPro sslmode=disable"

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("❌ Database Driver Initialization Failure: %v\n", err)
		return
	}

	err = DB.Ping()
	if err != nil {
		// This prints the message you saw if the credentials or server status fail
		fmt.Printf("❌ System Database Server Network Unreachable: %v\n", err)
	} else {
		fmt.Println("🚀 Secure Core Database Tunnel Established Successfully.")
	}
}

func CloseDatabase() {
	if DB != nil {
		DB.Close()
		fmt.Println("🔒 Secure Core Database Connection Closed Safely.")
	}
}
