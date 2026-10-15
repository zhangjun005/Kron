package servemcp

// C-1: register the kron://intents/<slug> resource template on the
// MCP server. One ResourceTemplate covers every intent — there is no
// need to enumerate intents into individual AddResource calls, and
// the SDK emits a resources/templates/list entry on its own that
// describes the URI pattern to the client.
//
// docs/mcp/missing-capabilities.md §C-1 / docs/mcp/backlog.md §C-1.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// kronIntentsResourceTemplate is the constant URI template published
// to clients in resources/templates/list. The {+slug} variable is
// RFC 6570 "reserved expansion" (the leading '+' instead of the
// usual '{slug}') so the matched value may contain "/" — without
// it the regex would only match single-segment slugs and
// "kron://intents/auth/jwt" would never resolve. go-sdk v1.1.0
// builds a regex from the RFC 6570 grammar (see
// server.go lookupResourceHandler → tmpl.Regexp().MatchString),
// and the SDK's own TestTemplateMatch confirms the {+var} form
// matches paths that contain "/". The slug grammar itself
// (parser.ValidateSlug) is unchanged; this URI template is a
// wire-format concern, not a slug-rule concern.
const kronIntentsResourceTemplate = "kron://intents/{+slug}"

// registerResources installs the kron://intents/{slug} template on
// server. Called once at startup, after registerTools. Idempotent on
// the Server level (we re-register per process; the SDK replaces any
// existing entry with the same URI).
//
// Why a template (not a static list of resources):
//   - 500 intents on disk would be 500 static AddResource calls; the
//     SDK re-allocates on each register and the on-the-wire
//     resources/list response grows linearly.
//   - Template is the MCP-blessed pattern for "N resources, one
//     shape, identified by a path variable". The SDK matches incoming
//     resources/read URIs against the template's pattern and dispatches
//     to the handler.
//   - Adding a new intent doesn't require a server restart — the
//     template handler re-reads on every read.
func registerResources(server *mcp.Server) {
	template := &mcp.ResourceTemplate{
		Name:        "kron-intent",
		Title:       "Kron intent",
		Description: "A Kron intent file at .kron/intents/<slug>.md. Read returns the raw markdown (frontmatter + body) so an AI client can ingest the intent directly without going through the kron_get tool.",
		MIMEType:    "text/markdown",
		// RFC 6570 URI template. The {slug} variable matches the
		// intent slug the client wants to read; slugs may contain
		// "/" (sub-intents like "auth/jwt") so we use the raw
		// RFC 6570 syntax (level 1, no URL escaping) — the SDK
		// forwards the matched value as-is to the handler. See
		// go-sdk v1.1.0 ResourceTemplate.URITemplate docs.
		URITemplate: kronIntentsResourceTemplate,
	}
	server.AddResourceTemplate(template, handleIntentResourceRead)
}

// handleIntentResourceRead is the per-read ResourceHandler. It is
// the resources/read entry point for the kron://intents/{slug}
// template. The slug arrives in req.Params.URI matched by the SDK
// against the template; we strip the kron://intents/ prefix and hand
// the rest to the store.
//
// Errors are mapped to MCP-shaped errors:
//   - empty / malformed URI       → mcp.ResourceNotFoundError (per
//     SDK convention: an unregistered
//     resource is indistinguishable from
//     a registered one whose handler
//     failed, to avoid leaking server
//     internals to the client).
//   - slug not on disk            → mcp.ResourceNotFoundError
//   - Load failure (parse / IO)   → wrapped as the raw error; the
//     SDK returns it to the client as
//     the tool result's `isError`.
//
// Per the iron rules in architecture.md §〇, this handler is the
// only place the access layer assembles "raw markdown bytes for
// consumption"; it intentionally does not reshape the frontmatter
// or filter the body — kron://intents/<slug> is a pure file
// mirror, and richer views go through the kron_get tool.
func handleIntentResourceRead(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	const prefix = "kron://intents/"
	if !strings.HasPrefix(uri, prefix) {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	slug := strings.TrimPrefix(uri, prefix)
	if slug == "" {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	// Slug normalisation is the store's job; we delegate.
	// store.Reader validates and returns a wrapped model.ErrSlugInvalid
	// for malformed slugs, which the SDK surfaces as a read error.
	// Repo root flows in via the callerInjectMiddleware (see
	// serve_mcp.go: repoRootKey + repoRootFrom). The handler reads
	// the same key every other tool handler reads, so the
	// failure mode when the middleware hasn't run is uniform
	// (empty string → store.NewReader reads from CWD).
	r, err := store.NewReader(repoRootFrom(ctx))
	if err != nil {
		return nil, fmt.Errorf("kron resources: open store: %w", err)
	}
	intent, err := r.Load(ctx, slug)
	if err != nil {
		// Map "not on disk" to the SDK's not-found shape so
		// clients see a clean 404 rather than a generic
		// "read failed" wrapper.
		if isIntentNotFound(err) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return nil, fmt.Errorf("kron resources: load %q: %w", slug, err)
	}
	text, err := parser.SerializeMarkdown(intent)
	if err != nil {
		return nil, fmt.Errorf("kron resources: marshal %q: %w", slug, err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     string(text),
		}},
	}, nil
}

// rootFromContext lives in serve_mcp.go (repoRootFrom + repoRootKey).
// Resources reuses it: the middleware sets repoRootKey on every
// incoming request, and store.NewReader reads it through the same
// path every other tool handler does.

// marshalIntentMarkdown lives in resources.go below.

// SerializeMarkdown re-emits an Intent as a .md file using the sentinel-wrapped
// frontmatter format (called from resources.go for the
// kron://intents/{slug} read handler). See internal/parser/frontmatter.go.
//
// We do not need a per-handler re-serialiser: the parser already
// guarantees the round-trip body+frontmatter shape that an editor
// would write, so reading via resources/read returns the canonical
// bytes a human would see.

// isIntentNotFound reports whether err is (or wraps) the
// model.ErrIntentNotFound sentinel. Plain errors.Is, kept as a
// one-liner for callers that prefer a bool over the (err, ok)
// two-return shape of errors.Is.
func isIntentNotFound(err error) bool {
	return errors.Is(err, model.ErrIntentNotFound)
}
