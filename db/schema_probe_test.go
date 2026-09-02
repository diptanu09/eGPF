package db

import (
	"os"
	"testing"
)

func TestTreasuryProfiles(t *testing.T) {
	_ = os.Chdir("..")
	dbConn, err := InitDatabase()
	if err != nil {
		t.Logf("Database connection failed (could be offline/VPN required): %v", err)
		return
	}
	defer CloseDB()

	// 1. Test FetchAllTreasuryProfiles
	profiles, err := FetchAllTreasuryProfiles("")
	if err != nil {
		t.Fatalf("FetchAllTreasuryProfiles error: %v", err)
	}
	if len(profiles) == 0 {
		t.Fatalf("Expected treasury profiles, got 0")
	}
	t.Logf("Successfully fetched %d treasury profiles", len(profiles))

	// 2. Test FetchAllTreasuryProfiles with filter
	filtered, err := FetchAllTreasuryProfiles("TPA01")
	if err != nil {
		t.Fatalf("FetchAllTreasuryProfiles('TPA01') error: %v", err)
	}
	if len(filtered) == 0 {
		t.Fatalf("Expected filtered treasury profiles matching 'TPA01', got 0")
	}
	t.Logf("Filtered profiles matching 'TPA01': %+v", filtered[0])

	// 3. Test FetchTreasuryDetails
	details, err := FetchTreasuryDetails("TPA01")
	if err != nil {
		t.Fatalf("FetchTreasuryDetails('TPA01') error: %v", err)
	}
	if details["tres_code"] != "TPA01" {
		t.Fatalf("Expected tres_code 'TPA01', got '%s'", details["tres_code"])
	}
	t.Logf("FetchTreasuryDetails('TPA01') => %+v", details)

	_ = dbConn
}
