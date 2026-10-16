# Kron VSCode extension

This directory contains the VSCode extension client for Kron. It starts
`kron serve-lsp` as a stdio child process; it does not import Kron Go packages
or call internal packages directly.

## Development

```sh
npm install
npm run compile
```

Open a workspace in the Extension Development Host and ensure `kron` is on
its PATH. Otherwise set `kron.serverPath` to the executable path in VSCode
settings (for Windows, use the path to `kron.exe`). Activation launches
`kron serve-lsp -CWD <workspace-root>` and performs the LanguageClient
initialize handshake. Activation without a workspace shows a warning and
does not start a process. The client registers all local files so LSP hover
and definition can inspect source anchors of the form
`// @kron:intent <slug>`.
