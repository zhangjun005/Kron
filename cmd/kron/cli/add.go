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
//
// Flags (RFC 2026-10-08-writer-readme-symmetry §3.4):
//
//	--kind leaf|node   On-disk form; default "leaf". "node" writes
//	                    <slug>/README.md (a module-level intent that
//	                    can co-exist with child leaf intents under the
//	                    same directory).
//	--parent <slug>    Optional. When set, the parent must already
//	                    exist as a node intent. This is the explicit
//	                    way to create a child under a known node —
//	                    slug-string form ("auth/jwt") is the implicit
//	                    way. The two are equivalent when the parent's
//	                    slug is a prefix of the child slug.
func runAdd(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: kron add <slug> [-CWD <root>] [-created-by <handle>] [-kind leaf|node] [-parent <slug>]")
		fs.PrintDefaults()
	}
	cwd := fs.String("CWD", "", "Repository root (default: current working directory)")
	createdBy := fs.String("created-by", "", "Override created_by handle (default: from git config user.name)")
	kindStr := fs.String("kind", "leaf", "On-disk form: \"leaf\" writes <slug>.md (default); \"node\" writes <slug>/README.md (module-level intent)")
	parent := fs.String("parent", "", "Parent node intent slug (must exist; explicit alternative to <parent>/<slug> slug form)")
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

	// Parse --kind into the model type. model.IntentKind("")
	// is "valid" (treated as leaf) so we accept an explicit
	// empty string from the flag default; anything else not in
	// the enum is rejected here.
	kind := model.IntentKind(*kindStr)
	if !kind.Valid() {
		fmt.Fprintf(errOut, "kron add: invalid --kind %q (must be leaf or node)\n", *kindStr)
		return errUsage
	}
	if kind == "" {
		kind = model.IntentKindLeaf
	}

	root, err := resolveRoot(*cwd)
	if err != nil {
		fmt.Fprintf(errOut, "kron add: %v\n", err)
		return errUsage
	}

	// Require an initialised .kron/ tree.
	kronDir := filepath.Join(root, model.KronDir)
	if _, err := os.Stat(kronDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(errOut, "kron add: .kron/ not found; run `kron init` first")
			return errUsage
		}
		fmt.Fprintf(errOut, "kron add: %v\n", err)
		return err
	}

	w, err := store.NewWriter(root)
	if err != nil {
		fmt.Fprintf(errOut, "kron add: %v\n", err)
		return errUsage
	}
	// Exists check is reader-side (both leaf and node forms
	// resolve) so the collision surface is consistent with the
	// read path. Without this, "kron add auth" after a manual
	// touch of auth/README.md would silently overwrite.
	if w.Exists(context.Background(), slug) {
		fmt.Fprintf(errOut, "kron add: intent %q already exists\n", slug)
		return errUsage
	}

	// --parent: when explicit, the parent MUST already exist as
	// a node intent. Exists is the cheap "is this slug known to
	// the store?" check; we don't separately verify it's a
	// node-form file because either form is acceptable as a
	// parent for the purposes of "this slug is known" — the
	// only practical case is a node form (you can't be a parent
	// of a child unless the parent's directory is the child's
	// directory, which is exactly the node form).
	if *parent != "" {
		if err := parser.ValidateSlug(*parent); err != nil {
			fmt.Fprintf(errOut, "kron add: invalid --parent %q: %v\n", *parent, err)
			return errUsage
		}
		if !w.Exists(context.Background(), *parent) {
			fmt.Fprintf(errOut, "kron add: parent intent %q does not exist\n", *parent)
			return errUsage
		}
	}

	handle := identity.Handle(*createdBy, identity.GitUser())

	intent := &model.Intent{
		Slug: slug,
		Kind: kind,
		Frontmatter: model.Frontmatter{
			CreatedBy: handle,
			UpdatedAt: time.Now().UTC(),
		},
		Body: parser.IntentBodyTemplateCN(slug),
	}
	if err := w.Write(context.Background(), intent); err != nil {
		// Surface the write error verbatim (already wrapped with
		// context at the store layer; ErrSlugCollision's message
		// already names the slug and form).
		fmt.Fprintf(errOut, "kron add: %v\n", err)
		return errUsage
	}

	// Echo the on-disk path the user actually got — for node
	// form this is <slug>/README.md, not <slug>.md. Leaf form
	// falls through to model.IntentPath which is the canonical
	// <slug>.md.
	if intent.Kind == model.IntentKindNode {
		fmt.Fprintf(out, "kron: created %s\n",
			filepath.Join(model.KronDir, model.IntentDir, slug, "README.md"))
	} else {
		fmt.Fprintf(out, "kron: created %s\n", model.IntentPath(slug))
	}
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
