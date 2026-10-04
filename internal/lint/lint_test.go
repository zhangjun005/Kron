package lint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/store"
)

// writeFile creates a file at <root>/<rel> with content, creating
// intermediate directories. Mirrors store/reader_test.go's helper but
// is duplicated (tests in internal/lint cannot import test helpers
// from internal/store). Keep the two helpers in sync if the layout
// ever changes.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

const sampleIntent = `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
status: active
<!-- /kron:frontmatter -->

# Sample
`

func TestRun_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	// No .kron/intents/ at all — anchor scan returns empty, LoadAll
	// returns empty, no diags, no error.
	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRun_AnchorDangling(t *testing.T) {
	dir := t.TempDir()
	// Source file references a slug that does not exist.
	writeFile(t, dir, "internal/auth/refresh.go", `package auth

// @kron:intent auth/refresh-token
func NewRefreshToken() string { return "" }
`)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleAnchorDangling, diags[0].Rule)
	assert.Equal(t, SeverityError, diags[0].Severity)
	// Where is repo-relative and includes the line number. Compare
	// path components separately to avoid platform-specific separators
	// (Windows filepath.Rel returns backslashes).
	assert.Contains(t, diags[0].Where, "refresh.go")
	assert.Contains(t, diags[0].Where, ":3")
	assert.Contains(t, diags[0].Detail, "auth/refresh-token")
}

func TestRun_AnchorValid(t *testing.T) {
	dir := t.TempDir()
	// The referenced intent exists — no diags expected.
	writeFile(t, dir, "internal/auth/auth.go", "// @kron:intent auth/jwt\npackage auth\n")
	writeFile(t, dir, ".kron/intents/auth/jwt.md", sampleIntent)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRun_FrontmatterLoadFailureRepromDiag(t *testing.T) {
	// v1 LoadAll is fail-fast: the first broken intent surfaces as a Go
	// error. Run() converts that error into a single repo-wide Diag so
	// the v1 contract "rule failures are observations, not exceptions"
	// holds uniformly across both rule classes.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/broken.md", "not a frontmatter block\njust body\n")

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleFrontmatterInvalid, diags[0].Rule)
	assert.Equal(t, SeverityError, diags[0].Severity)
	assert.Equal(t, "<root>", diags[0].Where)
	assert.NotEmpty(t, diags[0].Detail)
}

func TestRun_FrontmatterLoadFailureWithValidAnchor(t *testing.T) {
	// A dangling anchor combined with a broken frontmatter file: both
	// diags must appear (anchor scan succeeds, LoadAll fails). Run does
	// NOT short-circuit on LoadAll failure.
	dir := t.TempDir()
	writeFile(t, dir, "x.go", "// @kron:intent missing/slug\npackage x\n")
	writeFile(t, dir, ".kron/intents/broken.md", "not a frontmatter block\n")

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 2)

	// Order is deterministic: A-class runs first (anchor), B-class runs
	// second (frontmatter load).
	assert.Equal(t, RuleAnchorDangling, diags[0].Rule)
	assert.Equal(t, RuleFrontmatterInvalid, diags[1].Rule)
}

func TestRun_EmptyRoot(t *testing.T) {
	_, err := Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root path is empty")
}

func TestRun_SkipsKronDir(t *testing.T) {
	// The .kron/intents/*.md files may legitimately include "@kron:intent"
	// inside body examples. Run MUST NOT pick them up via anchor scan.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/sample.md", sampleIntent+
		"\nSee // @kron:intent fake/in-sample for context.\n")
	writeFile(t, dir, "real.go", "// @kron:intent sample\npackage x\n")

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags, "anchor in .kron/intents/*.md body must not count")
}

func TestHasErrors(t *testing.T) {
	cases := []struct {
		name  string
		diags []Diag
		want  bool
	}{
		{"empty", []Diag{}, false},
		{"only-warning", []Diag{{Severity: SeverityWarning}}, false},
		{"one-error", []Diag{{Severity: SeverityError}}, true},
		{"mixed", []Diag{{Severity: SeverityWarning}, {Severity: SeverityError}}, true},
		{"empty-severity-is-error", []Diag{{Severity: ""}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HasErrors(tc.diags))
		})
	}
}

func TestDiagSeverityEmptyIsError(t *testing.T) {
	d := Diag{Severity: ""}
	assert.Equal(t, SeverityError, d.severity())
}

func TestDiagSeverityRoundTrip(t *testing.T) {
	assert.Equal(t, SeverityError, (Diag{Severity: SeverityError}).severity())
	assert.Equal(t, SeverityWarning, (Diag{Severity: SeverityWarning}).severity())
}

func TestRuleConstantsAreStable(t *testing.T) {
	// Guard against accidental rename of the public Rule constants.
	// lint consumers (CI, editor plugins) depend on these string values.
	assert.Equal(t, Rule("anchor-dangling"), RuleAnchorDangling)
	assert.Equal(t, Rule("frontmatter-invalid"), RuleFrontmatterInvalid)
	assert.Equal(t, Rule("dangling-reference"), RuleDanglingReference)
	assert.Equal(t, Rule("dangling-depends-on"), RuleDanglingDependsOn)
	assert.Equal(t, Rule("depends-on-cycle"), RuleDependsOnCycle)
	assert.Equal(t, Rule("self-reference"), RuleSelfReference)
	assert.Equal(t, Rule("stale-superseded-candidate"), RuleStaleSupersededCandidate)
	assert.Equal(t, Rule("expired-hard-assumption"), RuleExpiredHardAssumption)
}

// --- C-class rules (intent → intent frontmatter relations) -----------
//
// Helper: build a 2-intent repo where target exists, then assert the
// expected diags. Kept local to this file so lint tests don't grow
// inter-file dependencies.

func TestRun_DanglingReference(t *testing.T) {
	// target references a slug that does not exist → warning, not error.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/a.md", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
references:
  - ghost/never-existed
  - b
<!-- /kron:frontmatter -->

# A
`)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	// Need intent `b` to exist so LoadAll succeeds.
	writeFile(t, dir, ".kron/intents/b.md", sampleIntent)

	diags, err = Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleDanglingReference, diags[0].Rule)
	assert.Equal(t, SeverityWarning, diags[0].Severity)
	assert.Equal(t, "a", diags[0].Where)
	assert.Contains(t, diags[0].Detail, "ghost/never-existed")
}

func TestRun_DanglingDependsOn(t *testing.T) {
	// target depends_on a slug that does not exist → error.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/a.md", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
depends_on:
  - ghost/missing
<!-- /kron:frontmatter -->

# A
`)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleDanglingDependsOn, diags[0].Rule)
	assert.Equal(t, SeverityError, diags[0].Severity)
	assert.Equal(t, "a", diags[0].Where)
	assert.Contains(t, diags[0].Detail, "ghost/missing")
}

func TestRun_DependsOnCycle(t *testing.T) {
	// A.depends_on B AND B.depends_on A → cycle, error.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/a.md", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
depends_on:
  - b
<!-- /kron:frontmatter -->

# A
`)
	writeFile(t, dir, ".kron/intents/b.md", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
depends_on:
  - a
<!-- /kron:frontmatter -->

# B
`)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.NotEmpty(t, diags)

	var cycleDiag *Diag
	for i := range diags {
		if diags[i].Rule == RuleDependsOnCycle {
			cycleDiag = &diags[i]
			break
		}
	}
	require.NotNil(t, cycleDiag, "expected at least one depends-on-cycle diag, got %+v", diags)
	assert.Equal(t, SeverityError, cycleDiag.Severity)
	assert.Contains(t, cycleDiag.Detail, "a")
	assert.Contains(t, cycleDiag.Detail, "b")
}

func TestRun_SelfReference(t *testing.T) {
	// A.references contains A → self-reference, error.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/a.md", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
references:
  - a
<!-- /kron:frontmatter -->

# A
`)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleSelfReference, diags[0].Rule)
	assert.Equal(t, SeverityError, diags[0].Severity)
}

func TestRun_ValidReferences_NoDiags(t *testing.T) {
	// A.references B and A.depends_on B; both exist → no diags.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/a.md", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
references:
  - b
depends_on:
  - b
<!-- /kron:frontmatter -->

# A
`)
	writeFile(t, dir, ".kron/intents/b.md", sampleIntent)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

// --- S-class rules (lifecycle staleness) ------------------------------
//
// Use opts.WithNow to pin the reference time so we don't depend on
// real wall-clock arithmetic (e.g., "100 days before time.Now()"
// would silently shift assertions across CI runs).

func TestRun_StaleSupersededCandidate_Fires(t *testing.T) {
	// An active intent whose UpdatedAt is older than the threshold
	// must be reported as a superseded candidate.
	dir := t.TempDir()
	// updated_at 200 days in the past; threshold 90 → stale.
	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := oldTime.AddDate(0, 0, 200)
	writeFile(t, dir, ".kron/intents/old-active.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: %s
status: active
<!-- /kron:frontmatter -->

# Old active
`, oldTime.Format(time.RFC3339)))

	diags, err := RunWith(context.Background(), dir, RunOptions{}.WithNow(now))
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleStaleSupersededCandidate, diags[0].Rule)
	assert.Equal(t, SeverityWarning, diags[0].Severity)
	assert.Equal(t, "old-active", diags[0].Where)
}

func TestRun_StaleSupersededCandidate_SkipsDraftAndSuperseded(t *testing.T) {
	// Draft and Superseded statuses are not flagged, even if old.
	dir := t.TempDir()
	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := oldTime.AddDate(0, 0, 200)
	for _, status := range []string{"draft", "superseded"} {
		writeFile(t, dir, fmt.Sprintf(".kron/intents/%s.md", status), fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: %s
status: %s
<!-- /kron:frontmatter -->

# %s
`, oldTime.Format(time.RFC3339), status, status))
	}
	diags, err := RunWith(context.Background(), dir, RunOptions{}.WithNow(now))
	require.NoError(t, err)
	assert.Empty(t, diags, "draft and superseded statuses must not trigger the staleness rule")
}

func TestRun_StaleSupersededCandidate_DisabledByNegativeThreshold(t *testing.T) {
	// Negative StaleDaysThreshold opts the whole S-class out.
	dir := t.TempDir()
	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now := oldTime.AddDate(0, 0, 200)
	writeFile(t, dir, ".kron/intents/old.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: %s
status: active
<!-- /kron:frontmatter -->

# Old
`, oldTime.Format(time.RFC3339)))
	diags, err := RunWith(context.Background(), dir, RunOptions{StaleDaysThreshold: -1}.WithNow(now))
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRun_ExpiredHardAssumption_Fires(t *testing.T) {
	// A hard assumption with ExpiresAt in the past and no
	// post-expiry verification must be reported.
	dir := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := now.AddDate(0, 0, -30) // 30 days ago
	writeFile(t, dir, ".kron/intents/has-expiry.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-01-01T00:00:00Z
assumptions:
  - id: region-stays-single
    text: single region
    severity: hard
    expires_at: %s
<!-- /kron:frontmatter -->

# Has expiry
`, expiry.Format(time.RFC3339)))

	diags, err := RunWith(context.Background(), dir, RunOptions{}.WithNow(now))
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleExpiredHardAssumption, diags[0].Rule)
	assert.Equal(t, SeverityWarning, diags[0].Severity)
	assert.Equal(t, "has-expiry", diags[0].Where)
	assert.Contains(t, diags[0].Detail, "region-stays-single")
}

func TestRun_ExpiredHardAssumption_VerifiedAfterExpiry_NoDiag(t *testing.T) {
	// If VerifiedAt is after the expiry, the assumption is current.
	dir := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := now.AddDate(0, 0, -30)
	verified := now.AddDate(0, 0, -10) // after expiry
	writeFile(t, dir, ".kron/intents/recently-verified.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-01-01T00:00:00Z
assumptions:
  - id: region-stays-single
    text: single region
    severity: hard
    expires_at: %s
    verified_at: %s
    verified_by: "@alice"
<!-- /kron:frontmatter -->

# Verified after expiry
`, expiry.Format(time.RFC3339), verified.Format(time.RFC3339)))

	diags, err := RunWith(context.Background(), dir, RunOptions{}.WithNow(now))
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRun_ExpiredHardAssumption_SoftAssumptionSkipped(t *testing.T) {
	// Soft assumptions past expiry do NOT trigger the rule.
	dir := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := now.AddDate(0, 0, -30)
	writeFile(t, dir, ".kron/intents/soft-only.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-01-01T00:00:00Z
assumptions:
  - id: dau-under-10k
    text: DAU under 10k
    severity: soft
    expires_at: %s
<!-- /kron:frontmatter -->

# Soft
`, expiry.Format(time.RFC3339)))

	diags, err := RunWith(context.Background(), dir, RunOptions{}.WithNow(now))
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestComputeStaleReport_MatchesDiagsView(t *testing.T) {
	// ComputeStaleReport's structured view must agree with the
	// Diag-shaped view produced by runStalenessRules for the same
	// input. This is the contract MCP kron_stale and CLI kron lint
	// rely on.
	dir := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	oldTime := now.AddDate(0, 0, -200)
	expiry := now.AddDate(0, 0, -30)
	writeFile(t, dir, ".kron/intents/stale-active.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: %s
status: active
<!-- /kron:frontmatter -->

# Stale active
`, oldTime.Format(time.RFC3339)))
	writeFile(t, dir, ".kron/intents/has-expiry.md", fmt.Sprintf(`<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-01-01T00:00:00Z
assumptions:
  - id: x
    text: x
    severity: hard
    expires_at: %s
<!-- /kron:frontmatter -->

# Has expiry
`, expiry.Format(time.RFC3339)))

	opts := RunOptions{}.WithNow(now)
	diags, err := RunWith(context.Background(), dir, opts)
	require.NoError(t, err)

	// Recompute via store.LoadAll + ComputeStaleReport.
	r, err := store.NewReader(dir)
	require.NoError(t, err)
	all, err := r.LoadAll(context.Background())
	require.NoError(t, err)
	report := ComputeStaleReport(all, opts)

	// Slug from S1 in diags matches SupersededCandidates.
	var diagsStale []string
	for _, d := range diags {
		if d.Rule == RuleStaleSupersededCandidate {
			diagsStale = append(diagsStale, d.Where)
		}
	}
	assert.Equal(t, report.SupersededCandidates, diagsStale)

	// ExpiredAssumptions count matches diags count for S2.
	var diagsS2 int
	for _, d := range diags {
		if d.Rule == RuleExpiredHardAssumption {
			diagsS2++
		}
	}
	assert.Len(t, report.ExpiredAssumptions, diagsS2)
}

// DO NOT REMOVE the closing blank line below; it keeps the file
// terminating with a newline as gofmt requires.
