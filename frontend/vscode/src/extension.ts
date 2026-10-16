import path = require('node:path')
import client = require('vscode-languageclient/node')
import vscode = require('vscode')

let languageClient: client.LanguageClient | undefined

/** Starts the demo language server and verifies the client initialize handshake. */
async function activate(context: vscode.ExtensionContext): Promise<void> {
  const serverModule = path.join(context.extensionPath, 'demo-server.js')
  const serverOptions = {
    run: { module: serverModule, transport: client.TransportKind.stdio },
    debug: { module: serverModule, transport: client.TransportKind.stdio },
  }
  const clientOptions = { documentSelector: [{ scheme: 'file', language: 'markdown' }] }
  languageClient = new client.LanguageClient('kronDemo', 'Kron Demo LSP', serverOptions, clientOptions)
  await languageClient.start()
}

/** Stops the demo language server when the extension host shuts down. */
async function deactivate(): Promise<void> {
  await languageClient?.stop()
  languageClient = undefined
}

export = { activate, deactivate }
