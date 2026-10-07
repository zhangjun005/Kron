package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/xxx/kron/internal/identity"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// runAdd implements `kron add <slug>`.
//
// It validates the slug, checks the .kron skeleton exists, refuses
// to overwrite an existing intent, then writes a scaffolded intent
// file with a default frontmatter (created_by from `git config
// user.name` if available, updated_at set to "now").
func runAdd(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: kron add <slug> [-CWD <root>] [-created-by <handle>]")
		fs.PrintDefaults()
	}
	cwd := fs.String("CWD", "", "Repository root (default: current working directory)")
	createdBy := fs.String("created-by", "", "Override created_by handle (default: from git config user.name)")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(errOut, "kron add: expected exactly one <slug> argument")
		return errUsage
	}
	slug := rest[0]
	if err := parser.ValidateSlug(slug); err != nil {
		fmt.Fprintf(errOut, "kron add: invalid slug %q: %v\n", slug, err)
		return errUsage
	}

	root, err := resolveRoot(*cwd)
	if err != nil {
		fmt.Fprintln(errOut, "kron add:", err)
		return errUsage
	}

	// Require an initialised .kron/ tree.
	kronDir := filepath.Join(root, model.KronDir)
	if _, err := os.Stat(kronDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(errOut, "kron add: .kron/ not found; run `kron init` first")
			return errUsage
		}
		fmt.Fprintln(errOut, "kron add:", err)
		return err
	}

	w, err := store.NewWriter(root)
	if err != nil {
		fmt.Fprintln(errOut, "kron add:", err)
		return err
	}
	if w.Exists(context.Background(), slug) {
		fmt.Fprintf(errOut, "kron add: intent %q already exists\n", slug)
		return errUsage
	}

	handle := identity.Handle(*createdBy, identity.GitUser())

	intent := &model.Intent{
		Slug: slug,
		Frontmatter: model.Frontmatter{
			CreatedBy: handle,
			UpdatedAt: time.Now().UTC(),
		},
		Body: parser.IntentBodyTemplateCN(slug),
	}
	if err := w.Write(context.Background(), intent); err != nil {
		fmt.Fprintln(errOut, "kron add:", err)
		return err
	}

	fmt.Fprintf(out, "kron: created %s\n", model.IntentPath(slug))
	return nil
}

// defaultIntentBody delegates to parser.IntentBodyTemplateCN. The
// wrapper is kept so the CLI's call site (runAdd) reads naturally;
// the actual template lives in internal/parser where it can be
// shared with any future access layer (the English variant is
// parser.IntentBodyTemplate).
func defaultIntentBody(slug string) string {
	return parser.IntentBodyTemplateCN(slug)
}
