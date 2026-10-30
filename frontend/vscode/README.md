# Kron for VS Code

**Kron** is a markdown intent ledger for software projects. Intents are small, reviewable documents
(`.kron/intents/<slug>.md`) that record *what* a piece of the product must do, *why* it exists, which
assumptions it leans on, and which code it owns. This extension brings that ledger into the editor:
browse the tree, read and edit intents in a canvas, follow code anchors, and keep assumptions honest.

- Repository: [github.com/zhangjun005/Kron](https://github.com/zhangjun005/Kron)
- Authors: **zhangjun005**, **HomoFluctus**, **lanyue811**

## Features

- **Intent tree in the sidebar** — a dedicated Kron Activity Bar view plus an optional Explorer view.
  Status is colour-coded (active / draft / superseded), with intent/folder counts in the view title.
  The dedicated sidebar also lists **Assumptions** (open the registry `.md` with one click) and
  **Superseded** intents.
- **Canvas workspace** — select an intent to open its card in a Webview: title, summary, symbols,
  status, reviewers, references, prerequisites, assumptions and body. Create child intents and edit
  frontmatter/body through forms (backed by `kron_add` / `kron_update`).
- **Assumption registry panel** — manage shared assumptions (`.kron/assumptions/<id>.md`): create,
  edit text/severity/status/expiry/reviewers, delete, and see which intents use each one.
- **Code anchors** — `@kron:intent <slug>` comments are underlined and Ctrl+click jumps to the intent
  document (served by `kron serve-lsp`). The editor context menu offers open/edit/impact actions for
  valid anchors, and create/attach/retarget/remove actions for missing or dangling ones.
- **Quick intent creation** — finish an anchor for a slug that does not exist yet and Kron offers to
  create it through `kron_add`.

## Requirements

The extension starts `kron serve-mcp` and `kron serve-lsp` for the current project, so a `kron`
executable is required:

1. Build or install Kron (Go 1.24+): `go build ./cmd/kron` or download a release binary.
2. Point the extension at it with the `kron.serverPath` setting (default: `kron` from `PATH`).
3. Open a workspace folder that contains a `.kron/` directory. The first folder with `.kron/` is the
   active project (single-project policy).

The extension only activates in workspaces containing `.kron/**`, or when you open a Kron view — it
does not spawn background processes for unrelated projects.

## Settings

| Setting | Default | Description |
|---|---|---|
| `kron.serverPath` | `kron` | Path to the Kron executable used for MCP and LSP. |
| `kron.webviewFontSize` | `14` | Base font size for the intent canvas Webview. |
| `kron.sidebar.independent` | `true` | Show the dedicated Kron view in the Activity Bar. |
| `kron.sidebar.explorer` | `false` | Also show the intent tree inside the Explorer sidebar. |

Both sidebar options can be enabled or disabled independently.

Anchor scanning is configured per project in `.kron/anchor-scan.json` (created by `kron init`). It maps
languages to file extensions and optional comment syntax, so React/Vue/Svelte files get LSP jump and
name-informed anchor insertion without a separate VS Code setting. Saving that file (or
`.kron/templates/intent-body.md`) restarts the Kron connections automatically. After changing
`kron.serverPath`, run **Kron: Restart Connections**; the font size applies to open canvases immediately.

## Commands

- **Kron: Open Intent Workspace** — open the canvas for the current project.
- **Kron: Refresh Intents** — re-read the tree, assumptions and superseded list.
- **Kron: Restart Connections** — restart the MCP/LSP subprocesses (after settings changes).
- **Kron: Open Intent Details / Edit Intent / Open Intent Markdown / Show Intent Impact** — anchor
  context menu actions for a valid anchor.
- **Kron: Create Intent and Attach Anchor / Attach Existing Intent Anchor** — add an anchor at the cursor.

## Development

```bash
cd frontend/vscode
npm install
npm run compile          # extension host → out/
cd webview && npm run build   # Webview → dist/webview/
cd .. && npm run package      # VSIX (also rebuilds Webview + host)
```

Press **F5** to launch the Extension Development Host. The Webview can also be previewed without
VS Code: `cd webview && npm run dev` renders the mock tree.

See [`docs/implementation/vscode-extension.md`](../../docs/implementation/vscode-extension.md) for the
host/Webview contract, anchor grammar and current limitations.
