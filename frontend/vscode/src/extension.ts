import client = require('vscode-languageclient/node')
import vscode = require('vscode')

let languageClient: client.LanguageClient | undefined

/** Starts kron serve-lsp for the first workspace and completes LSP initialize. */
async function activate(context: vscode.ExtensionContext): Promise<void> {
  const workspace = vscode.workspace.workspaceFolders?.[0]
  if (!workspace) {
    void vscode.window.showWarningMessage('Kron requires an open workspace folder to start its language server.')
    return
  }
  const configuredPath = vscode.workspace.getConfiguration('kron').get<string>('serverPath', 'kron')
  const serverOptions: client.ServerOptions = {
    run: { command: configuredPath, args: ['serve-lsp', '-CWD', workspace.uri.fsPath] },
    debug: { command: configuredPath, args: ['serve-lsp', '-CWD', workspace.uri.fsPath] },
  }
  const clientOptions = { documentSelector: [{ scheme: 'file', language: 'markdown' }] }
  languageClient = new client.LanguageClient('kron', 'Kron', serverOptions, clientOptions)
  await languageClient.start()
}

/** Stops the Kron language server when the extension host shuts down. */
async function deactivate(): Promise<void> {
  await languageClient?.stop()
  languageClient = undefined
}

export = { activate, deactivate }
