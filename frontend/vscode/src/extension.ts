import client = require('vscode-languageclient/node')
import vscode = require('vscode')
import intentTreeModule = require('./intent-tree')
import zod = require('zod')

const z = zod.z

const DEFAULT_LSP_LANGUAGES = [
  'go',
  'markdown',
  'typescript',
  'javascript',
  'python',
  'rust',
  'java',
  'c',
  'cpp',
  'csharp',
  'kotlin',
  'swift',
  'shellscript',
] as const

const LspLanguagesSchema = z.array(z.string().min(1))

let languageClients: client.LanguageClient[] = []
let intentTree: InstanceType<typeof intentTreeModule.IntentTreeProvider> | undefined

/** Starts independent Kron protocol clients for every workspace folder. */
async function activate(context: vscode.ExtensionContext): Promise<void> {
  const workspaces = vscode.workspace.workspaceFolders ?? []
  if (workspaces.length === 0) {
    void vscode.window.showWarningMessage('Kron requires an open workspace folder to start its language server.')
    return
  }
  const configuredPath = vscode.workspace.getConfiguration('kron').get<string>('serverPath', 'kron')
  const configuredLanguages = vscode.workspace
    .getConfiguration('kron')
    .get<unknown>('lspLanguages', DEFAULT_LSP_LANGUAGES)
  const parsedLanguages = LspLanguagesSchema.safeParse(configuredLanguages)
  const lspLanguages = parsedLanguages.success ? parsedLanguages.data : [...DEFAULT_LSP_LANGUAGES]
  for (const workspace of workspaces) {
    const serverOptions: client.ServerOptions = {
      run: { command: configuredPath, args: ['serve-lsp', '-CWD', workspace.uri.fsPath] },
      debug: { command: configuredPath, args: ['serve-lsp', '-CWD', workspace.uri.fsPath] },
    }
    const clientOptions: client.LanguageClientOptions = {
      documentSelector: lspLanguages.map((language) => ({
        scheme: 'file',
        language,
        // LSP DocumentFilter patterns are serialized as glob strings, so use
        // the absolute filesystem path and normalize Windows separators.
        pattern: `${workspace.uri.fsPath.replace(/\\/g, '/').replace(/\/$/, '')}/**`,
      })),
      outputChannelName: `Kron LSP: ${workspace.name}`,
      revealOutputChannelOn: client.RevealOutputChannelOn.Error,
    }
    const languageClient = new client.LanguageClient(`kron-${workspace.uri.toString()}`, 'Kron', serverOptions, clientOptions)
    try {
      await languageClient.start()
      languageClients.push(languageClient)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error)
      await vscode.window.showErrorMessage(`Kron LSP failed to start for ${workspace.name}: ${message}`)
      throw error
    }
  }

	intentTree = new intentTreeModule.IntentTreeProvider(configuredPath, workspaces)
  context.subscriptions.push(
    vscode.window.createTreeView('kron.intents', { treeDataProvider: intentTree }),
    vscode.commands.registerCommand('kron.refreshIntents', async () => {
      try {
        await intentTree?.refresh()
      } catch (error: unknown) {
        const message = error instanceof Error ? error.message : String(error)
        await vscode.window.showErrorMessage(`Kron: unable to refresh intents: ${message}`)
      }
    }),
  )
  try {
    await intentTree.refresh()
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error)
    await vscode.window.showErrorMessage(`Kron: unable to load intent tree: ${message}`)
  }
}

/** Stops the Kron language server when the extension host shuts down. */
async function deactivate(): Promise<void> {
	await Promise.all(languageClients.map((languageClient) => languageClient.stop()))
	languageClients = []
  intentTree?.dispose()
  intentTree = undefined
}

export = { activate, deactivate }
