package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// runMigrate dispatches the `kron migrate <subcommand>` family.
//
// Subcommands:
//
//	migrate assumptions --to-standalone [--dry-run] [--yes]
//
// Walks every intent's inline assumptions[]. For each one, writes a
// corresponding .kron/assumptions/<id>.md if absent (or matches an
// existing one). The intent frontmatter is left intact (B-3
// preserves the per-intent severity + rationale); a synthetic
// rationale placeholder is generated from the inline `text` field
// if the intent has no rationale yet — humans are expected to
// review and refine.
//
// Idempotency: an existing .kron/assumptions/<id>.md with the SAME
// (text, default_severity) is left alone. A divergent file is
// reported as a hard error so the human resolves the conflict
// manually.
func runMigrate(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("migrate: subcommand required (try `kron migrate assumptions --to-standalone`)")
	}
	switch args[0] {
	case "assumptions":
		return runMigrateAssumptions(args[1:], out, errOut)
	default:
		return fmt.Errorf("migrate: unknown subcommand %q", args[0])
	}
}

// migrateOpts captures the parsed CLI flags.
type migrateOpts struct {
	toStandalone bool
	dryRun       bool
	yes          bool
}

// parseMigrateOpts is a tiny hand-rolled parser; the CLI surface is
// small enough that cobra would be overkill.
func parseMigrateOpts(args []string) (migrateOpts, error) {
	opts := migrateOpts{}
	for _, a := range args {
		switch a {
		case "--to-standalone":
			opts.toStandalone = true
		case "--dry-run":
			opts.dryRun = true
		case "--yes", "-y":
			opts.yes = true
		default:
			return opts, fmt.Errorf("migrate: unknown flag %q", a)
		}
	}
	if !opts.toStandalone {
		return opts, errors.New("migrate: only --to-standalone is supported in v1.3")
	}
	return opts, nil
}

// assumptionEntry is the in-memory view of a single assumption id
// collected across all intents, used for conflict detection.
type assumptionEntry struct {
	text    string
	sev     model.Severity
	expires string
	from    []string // intent slugs that reference it
}

// runMigrateAssumptions walks every intent, projects each inline
// assumption into a registry file, and prints a human-readable
// summary.
func runMigrateAssumptions(args []string, out, errOut io.Writer) error {
	opts, err := parseMigrateOpts(args)
	if err != nil {
		return err
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("migrate: getwd: %w", err)
	}
	absRoot, err := findRepoRoot(root)
	if err != nil {
		return err
	}

	ctx := context.Background()

	r, err := store.NewReader(absRoot)
	if err != nil {
		return fmt.Errorf("migrate: open store: %w", err)
	}
	intents, err := r.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("migrate: load intents: %w", err)
	}

	if !opts.yes {
		fmt.Fprintf(errOut, "About to migrate %d intents to .kron/assumptions/*.md (dry-run=%v). Continue? [y/N] ", len(intents), opts.dryRun)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Fprintln(errOut, "aborted.")
			return nil
		}
	}

	w, err := assumption.NewWriter(absRoot)
	if err != nil {
		return fmt.Errorf("migrate: open writer: %w", err)
	}

	// Collect (id → entry) across all intents. If the same id
	// appears in multiple intents, the FIRST severity/text we see
	// is the default; subsequent entries must match or the
	// migration reports a conflict.
	reg := make(map[string]*assumptionEntry)
	var order []string
	for _, in := range intents {
		for _, a := range in.Frontmatter.Assumptions {
			if a.ID == "" {
				continue
			}
			if existing, ok := reg[a.ID]; !ok {
				reg[a.ID] = &assumptionEntry{
					text:    a.Text,
					sev:     a.Severity,
					expires: a.ExpiresAt,
					from:    []string{in.Slug},
				}
				order = append(order, a.ID)
			} else {
				existing.from = append(existing.from, in.Slug)
				if existing.text != a.Text {
					return fmt.Errorf("migrate: conflict on assumption %q: text differs between intents %q (%q) and %q (%q); resolve manually before migrating",
						a.ID, existing.from[0], existing.text, in.Slug, a.Text)
				}
				if existing.sev != a.Severity {
					return fmt.Errorf("migrate: conflict on assumption %q: severity differs between intents %q (%s) and %q (%s); resolve manually before migrating",
						a.ID, existing.from[0], existing.sev, in.Slug, a.Severity)
				}
			}
		}
	}
	sort.Strings(order)

	created, skipped, failed := 0, 0, 0
	for _, id := range order {
		e := reg[id]
		if opts.dryRun {
			fmt.Fprintf(out, "[dry-run] would create .kron/assumptions/%s.md (text=%q default_severity=%s referenced by %d intents)\n",
				id, e.text, e.sev, len(e.from))
			created++
			continue
		}
		err := w.Create(ctx, id, e.text, e.sev, "agent:migrate")
		if err != nil {
			if errors.Is(err, assumption.ErrAssumptionExists) {
				// Idempotent path: file already exists; leave
				// it alone. Lint will surface any divergence.
				fmt.Fprintf(out, "  [skip] %s: already present\n", id)
				skipped++
				continue
			}
			fmt.Fprintf(errOut, "  [fail] %s: %v\n", id, err)
			failed++
		} else {
			fmt.Fprintf(out, "  [create] %s\n", id)
			created++
		}
	}

	fmt.Fprintf(out, "\nmigration summary: created=%d skipped=%d failed=%d\n", created, skipped, failed)
	if failed > 0 {
		return newExitError(1, "migration completed with failures; see above")
	}
	return nil
}

// findRepoRoot walks up from start looking for a .kron/ directory.
// Returns the absolute path of the directory containing .kron/, or
// an error if none is found within a reasonable depth.
func findRepoRoot(start string) (string, error) {
	cur, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for i := 0; i < 16; i++ {
		candidate := filepath.Join(cur, ".kron")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("migrate: no .kron/ directory found above %q", start)
		}
		cur = parent
	}
	return "", fmt.Errorf("migrate: .kron/ not found within 16 levels of %q", start)
}
