package lint

// Diag is one diagnostic produced by Run.
//
// Rule and Severity are typed (not strings) so consumers get compile-time
// guarantees on which values are well-formed. Where carries enough
// context to point a human at the offending location; for repo-wide
// issues (e.g., LoadAll failure) it is the sentinel value "<root>".
type Diag struct {
	// Rule is the stable id of the check that produced this diagnostic.
	Rule Rule

	// Where is the repo-relative location of the issue. For anchor
	// diags it is "<relative path>:<line>"; for repo-wide diags it is
	// the literal "<root>".
	Where string

	// Detail is a human-readable explanation. It is intended for humans
	// reading lint output, not for machine parsing; use Rule for that.
	Detail string

	// Severity is "error" or "warning". Empty is treated as error.
	Severity Severity
}

// severity returns d.Severity, defaulting to SeverityError when empty.
// Used by cli/json output formatters; kept package-private so callers
// go through HasErrors / future Render helpers rather than comparing
// strings directly.
func (d Diag) severity() Severity {
	if d.Severity == "" {
		return SeverityError
	}
	return d.Severity
}

// ExpiredAssumption is one machine-readable entry in StaleReport.
// Mirrors the wire shape of MCP `kron_stale.expired_assumptions[]`
// so a caller can render the report directly without re-typing the
// fields. Field tags use jsonschema so the MCP SDK can derive the
// tools/list response schema automatically.
type ExpiredAssumption struct {
	Slug         string `json:"slug"           jsonschema:"slug of the owning intent"`
	AssumptionID string `json:"assumption_id"  jsonschema:"stable assumption identifier"`
	ExpiresAt    string `json:"expires_at"    jsonschema:"RFC3339 expiry timestamp"`
	DaysOverdue  int    `json:"days_overdue"   jsonschema:"days past expiry (positive)"`
}

// StaleReport is the structured view of every S-class (staleness) rule
// match. It is the data twin of the RuleStaleSupersededCandidate /
// RuleExpiredHardAssumption entries in []Diag.
//
// Callers that need a list of stale slugs or expired assumptions in
// machine-readable form (MCP kron_stale, future IDE status bar) read
// this directly. Callers that need human-readable output (CLI kron
// lint) read the Diag list returned by Run/RunWith — both come from
// the same walk over the same data, so they never disagree.
type StaleReport struct {
	// ThresholdDays is the effective staleness threshold the report
	// was computed with (resolved from RunOptions.StaleDaysThreshold
	// or StaleDaysDefault). 0 indicates "staleness rules disabled".
	ThresholdDays int

	// SupersededCandidates is the sorted list of slugs whose status
	// is active and whose UpdatedAt is older than ThresholdDays days.
	SupersededCandidates []string

	// ExpiredAssumptions lists every hard-severity assumption past
	// its ExpiresAt with no post-expiry verification. Order is the
	// LoadAll order (lexicographic by slug); the consumer is expected
	// to sort if a different order matters.
	ExpiredAssumptions []ExpiredAssumption
}
