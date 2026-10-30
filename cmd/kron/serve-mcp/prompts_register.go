package servemcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// registerPrompts installs the 3 prompt templates on server.
//
// RFC docs/rfc/2026-10-14-mcp-prompts.md §2.1 captures the contract;
// the templates wire into handlers_prompts.go which builds the
// rendered messages from internal/ data (LoadAllCached +
// relations.Prerequisites + lint.RunWith / ComputeStaleReport).
//
// Order matches the canonical "AI development loop" sequence:
// implement → drift-check → review. The wire order does not
// affect behaviour (prompts/list is a list, not a sequence) but
// keeps the human-readable list aligned with the loop the AI
// should walk.
//
// The SDK auto-infers the prompts capability from these
// AddPrompt calls; capabilities_test.go asserts the wire
// `initialize` response includes `prompts: { listChanged: true }`.
func registerPrompts(server *mcp.Server) {
	// kron-implement — pick the right intent for a new feature.
	server.AddPrompt(&mcp.Prompt{
		Name:        "kron-implement",
		Description: "Pick the right intent to implement a new feature. Pass `feature`; the prompt ranks existing intents by slug/symbol/title, and surfaces each match's prerequisites plus assumption links (severity/rationale/expiry/verification) so the AI reads and verifies them before editing.",
		Arguments: []*mcp.PromptArgument{{
			Name:        "feature",
			Description: "Free-form description of the feature or change the user wants to implement. Required.",
			Required:    true,
		}},
	}, kronImplementPrompt)

	// kron-drift-check — reconcile code changes against intent state.
	server.AddPrompt(&mcp.Prompt{
		Name:        "kron-drift-check",
		Description: "Surface code-vs-intent drift: superseded candidates and expired hard assumptions (rendered with intent:assumption, days overdue and expires_at). Optional `days_threshold` (default 90).",
		Arguments: []*mcp.PromptArgument{{
			Name:        "days_threshold",
			Description: "Minimum age in days for an active intent to be a superseded candidate. Optional; default 90.",
			Required:    false,
		}},
	}, kronDriftCheckPrompt)

	// kron-review — full active-intent audit for PR review.
	server.AddPrompt(&mcp.Prompt{
		Name:        "kron-review",
		Description: "Full active-intent audit for PR review. Lists every in-scope intent and groups lint findings by rule with location, detail and severity. Optional `status` (default 'active').",
		Arguments: []*mcp.PromptArgument{{
			Name:        "status",
			Description: "Filter the in-scope intent list by status. Optional; default 'active'.",
			Required:    false,
		}},
	}, kronReviewPrompt)
}
