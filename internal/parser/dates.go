package parser

import (
	"fmt"
	"strings"
	"time"
)

// DateOnlyLayout is the documented human-friendly form for assumption dates
// (`expires_at: 2027-01-01` / `verified_at: 2026-10-01`) per
// docs/abstractDesign/intent-structure.md §假设引用.
const DateOnlyLayout = "2006-01-02"

// parseAssumptionDate accepts the two supported assumption date forms:
//
//   - full RFC3339 ("2026-10-01T08:00:00Z")
//   - date-only ("2026-10-01")
//
// The returned bool reports whether the input was date-only, so callers can
// pick the right day boundary.
func parseAssumptionDate(raw string) (time.Time, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false, fmt.Errorf("date is empty")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), false, nil
	}
	if parsed, err := time.Parse(DateOnlyLayout, value); err == nil {
		return parsed.UTC(), true, nil
	}
	return time.Time{}, false, fmt.Errorf("unsupported date %q: want YYYY-MM-DD or RFC3339", raw)
}

// ParseExpiryDate parses an `expires_at` value.
//
// A date-only value means the whole day: the returned instant is the END of
// that day in UTC, so `expires_at: 2027-01-01` is still valid throughout
// 2027-01-01 and expires at 2027-01-02T00:00:00Z.
func ParseExpiryDate(raw string) (time.Time, error) {
	parsed, dateOnly, err := parseAssumptionDate(raw)
	if err != nil {
		return time.Time{}, err
	}
	if dateOnly {
		return parsed.Add(24*time.Hour - time.Nanosecond), nil
	}
	return parsed, nil
}

// ParseVerificationDate parses a `verified_at` value.
//
// A date-only value means "that day": the returned instant is the START of
// the day in UTC. It is used for comparisons such as "was this assumption
// re-verified after it expired?".
func ParseVerificationDate(raw string) (time.Time, error) {
	parsed, _, err := parseAssumptionDate(raw)
	return parsed, err
}

// ValidateAssumptionDateFields validates the optional date fields shared by
// intent-level assumption references and registry assumption files. Empty
// values are allowed (the field is optional).
func ValidateAssumptionDateFields(expiresAt, verifiedAt string) error {
	if strings.TrimSpace(expiresAt) != "" {
		if _, err := ParseExpiryDate(expiresAt); err != nil {
			return fmt.Errorf("expires_at: %w", err)
		}
	}
	if strings.TrimSpace(verifiedAt) != "" {
		if _, err := ParseVerificationDate(verifiedAt); err != nil {
			return fmt.Errorf("verified_at: %w", err)
		}
	}
	return nil
}
