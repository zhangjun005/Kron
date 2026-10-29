package parser

import (
	"testing"
	"time"
)

func TestParseExpiryDate_DateOnlyCoversWholeDay(t *testing.T) {
	expiry, err := ParseExpiryDate("2027-01-01")
	if err != nil {
		t.Fatalf("ParseExpiryDate: %v", err)
	}
	want := time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)
	if expiry.After(want) || expiry.Before(want.Add(-2*time.Hour)) {
		t.Fatalf("date-only expiry = %s, want end of 2027-01-01 (≈%s)", expiry, want)
	}
	// 2027-01-01T23:00Z must still count as "not expired".
	if !expiry.After(time.Date(2027, 1, 1, 23, 0, 0, 0, time.UTC)) {
		t.Fatalf("date-only expiry %s must be later than 2027-01-01T23:00Z", expiry)
	}
}

func TestParseExpiryDate_RFC3339(t *testing.T) {
	expiry, err := ParseExpiryDate("2026-12-31T00:00:00Z")
	if err != nil {
		t.Fatalf("ParseExpiryDate: %v", err)
	}
	if !expiry.Equal(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("RFC3339 expiry = %s", expiry)
	}
}

func TestParseVerificationDate_DateOnly(t *testing.T) {
	verified, err := ParseVerificationDate("2026-10-01")
	if err != nil {
		t.Fatalf("ParseVerificationDate: %v", err)
	}
	if !verified.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date-only verified_at = %s, want 2026-10-01T00:00:00Z", verified)
	}
}

func TestParseAssumptionDate_RejectsUnknownFormats(t *testing.T) {
	bad := []string{"", " ", "2027/01/01", "2027-1-1", "01-01-2027", "tomorrow", "2027-01-01 10:00:00"}
	for _, raw := range bad {
		if _, err := ParseExpiryDate(raw); err == nil {
			t.Errorf("ParseExpiryDate(%q) must fail", raw)
		}
	}
}

func TestValidateAssumptionDateFields(t *testing.T) {
	if err := ValidateAssumptionDateFields("", ""); err != nil {
		t.Fatalf("empty optional dates must pass: %v", err)
	}
	if err := ValidateAssumptionDateFields("2027-01-01", "2026-10-01"); err != nil {
		t.Fatalf("documented date-only forms must pass: %v", err)
	}
	if err := ValidateAssumptionDateFields("nope", ""); err == nil {
		t.Fatal("invalid expires_at must fail")
	}
	if err := ValidateAssumptionDateFields("", "nope"); err == nil {
		t.Fatal("invalid verified_at must fail")
	}
}
