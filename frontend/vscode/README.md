# Kron VSCode extension

This directory contains the VSCode extension client for Kron. It starts
`kron serve-lsp` and `kron serve-mcp` as stdio child processes; it does not
import Kron Go packages or call internal packages directly.

## Development

```sh
npm install
npm run compile
```

Press F5 in VSCode to start an Extension Development Host. The current
extension activation uses `demo-server.js` to exercise the
`vscode-languageclient` initialize/shutdown lifecycle. The demo is local and
does not yet launch the Kron binary; real `kron serve-lsp` integration is the
next milestone.
