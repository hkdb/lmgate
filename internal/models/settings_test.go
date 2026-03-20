package models

import (
	"testing"
)

func TestGetGeneralSettings_ReturnsDefaults(t *testing.T) {
	db := testOpenDB(t)

	defaults := GeneralSettings{
		RateLimitEnabled:    true,
		RateLimitDefaultRPM: 60,
		MaxFailedLogins:     5,
	}

	got, err := GetGeneralSettings(db, defaults)
	if err != nil {
		t.Fatalf("GetGeneralSettings: %v", err)
	}

	if !got.RateLimitEnabled {
		t.Error("expected RateLimitEnabled to be true (default)")
	}
	if got.RateLimitDefaultRPM != 60 {
		t.Errorf("RateLimitDefaultRPM: got %d, want 60", got.RateLimitDefaultRPM)
	}
}

func TestSaveAndGetGeneralSettings_RoundTrip(t *testing.T) {
	db := testOpenDB(t)

	saved := GeneralSettings{
		RateLimitEnabled:    false,
		RateLimitDefaultRPM: 120,
		APILogEnabled:       true,
		MaxFailedLogins:     10,
		PasswordMinLength:   16,
	}

	if err := SaveGeneralSettings(db, saved); err != nil {
		t.Fatalf("SaveGeneralSettings: %v", err)
	}

	got, err := GetGeneralSettings(db, GeneralSettings{})
	if err != nil {
		t.Fatalf("GetGeneralSettings: %v", err)
	}

	if got.RateLimitEnabled {
		t.Error("expected RateLimitEnabled to be false")
	}
	if got.RateLimitDefaultRPM != 120 {
		t.Errorf("RateLimitDefaultRPM: got %d, want 120", got.RateLimitDefaultRPM)
	}
	if got.MaxFailedLogins != 10 {
		t.Errorf("MaxFailedLogins: got %d, want 10", got.MaxFailedLogins)
	}
}
