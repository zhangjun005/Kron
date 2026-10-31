Kron v0.3.0 — Windows bundle
============================

Contents
--------
  Kron-GUI.exe              Wails desktop GUI
  kron.exe                  CLI + MCP + LSP server (single binary)
  kron-vscode-0.3.0.vsix    VS Code extension
  LICENSE                   MIT license
  README.txt                this file

Quick start
-----------
1. Double-click Kron-GUI.exe (it finds kron.exe next to it).
2. Add a project folder containing .kron/, or run:
       kron.exe init -CWD <project-folder>
3. Install the VS Code extension:
       code --install-extension kron-vscode-0.3.0.vsix
   Then set `kron.serverPath` to the absolute path of kron.exe.

MCP setup (AI agents)
---------------------
kron.exe is also the MCP server (stdio). To use it from Claude Desktop,
Cursor, or another MCP client:

1) Prepare a project (skip if it already has .kron/):
       kron.exe init -CWD C:\work\my-project

2) Add this to the client's MCP config:
   {
     "mcpServers": {
       "kron": {
         "command": "C:\\Tools\\Kron\\kron.exe",
         "args": ["serve-mcp", "-CWD", "C:\\work\\my-project"]
       }
     }
   }

   Without -CWD, `serve-mcp` resolves the project from projects.json next to
   kron.exe (current_project), then from the current directory.

3) Restart the MCP client. You should see 18 kron_* tools; try kron_list.

Notes
-----
- projects.json / guiSettings.json / covers/ are created at runtime next to
  the executables.
- `serve-mcp` / `serve-lsp` use the same kron.exe.
- Requires Windows 10/11 and Microsoft Edge WebView2 Runtime for the GUI.
- Full tutorial:
  https://github.com/zhangjun005/Kron/blob/master/docs/getting-started.md
- Source / releases: https://github.com/zhangjun005/Kron
