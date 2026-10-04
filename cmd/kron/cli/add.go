package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		Body: defaultIntentBody(slug),
	}
	if err := w.Write(context.Background(), intent); err != nil {
		fmt.Fprintln(errOut, "kron add:", err)
		return err
	}

	fmt.Fprintf(out, "kron: created %s\n", model.IntentPath(slug))
	return nil
}

// defaultIntentBody produces the Markdown body for a freshly scaffolded
// intent. The body follows the §三 skeleton from
// docs/abstractDesign/intent-structure.md (Title + one-line summary +
// Why + Trade-offs + Assumptions pointer). The English template is
// also used by the MCP `kron_add` handler (handlers_add.go:scaffoldedBody);
// the two access layers render the same body so the on-disk shape
// doesn't depend on which access layer wrote it.
func defaultIntentBody(slug string) string {
	title := slug
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		title = slug[i+1:]
	}
	title = strings.ReplaceAll(title, "-", " ")

	return fmt.Sprintf("# %s\n\n> One-line summary of this intent's decision.\n\n## 为什么（Why）\n记录当初为何如此决策。\n\n## 权衡（Trade-offs）\n选择与放弃的考量。\n\n<!-- 边界假设写在 frontmatter 的 assumptions 字段里，不在正文重复 -->\n", title)
}
