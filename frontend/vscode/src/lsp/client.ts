import vscode = require('vscode')
import languageClient = require('vscode-languageclient/node')
import type { ConnectionState } from '../mcp/client'

export const DEFAULT_LSP_LANGUAGES = [
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

/** Owns one independent kron serve-lsp process for one workspace folder. */
export class KronLspClient {
  private client: languageClient.LanguageClient | undefined
  private stateSubscription: vscode.Disposable | undefined
  private currentState: ConnectionState = 'stopped'

  constructor(
    private readonly command: string,
    private readonly workspace: vscode.WorkspaceFolder,
    private readonly languages: readonly string[],
    private readonly log: (message: string) => void,
    private readonly onState: (state: ConnectionState) => void,
  ) {}

  get state(): ConnectionState {
    return this.currentState
  }

  async start(): Promise<void> {
    if (this.currentState === 'ready' || this.currentState === 'starting') return
    this.setState('starting')

    const serverOptions: languageClient.ServerOptions = {
      run: {
        command: this.command,
        args: ['serve-lsp', '-CWD', this.workspace.uri.fsPath],
        options: { cwd: this.workspace.uri.fsPath },
      },
      debug: {
        command: this.command,
        args: ['serve-lsp', '-CWD', this.workspace.uri.fsPath],
        options: { cwd: this.workspace.uri.fsPath },
      },
    }
    const clientOptions: languageClient.LanguageClientOptions = {
      documentSelector: this.languages.map((language) => ({
        scheme: this.workspace.uri.scheme,
        language,
      })),
    }
    const client = new languageClient.LanguageClient(
      `kron-lsp-${this.workspace.index}`,
      'Kron Language Server',
      serverOptions,
      clientOptions,
    )
    this.client = client
    this.stateSubscription = client.onDidChangeState(() => {
      if (!client.isRunning() && this.currentState === 'ready') {
        this.log(`LSP process exited for workspace ${this.workspace.name}.`)
        this.setState('failed')
      }
    })

    try {
      await client.start()
      this.setState('ready')
    } catch (error: unknown) {
      this.log(`LSP startup failed for workspace ${this.workspace.name}: ${error instanceof Error ? error.message : String(error)}`)
      this.setState('failed')
      try {
        await client.stop()
      } catch (stopError: unknown) {
        this.log(`LSP cleanup error: ${stopError instanceof Error ? stopError.message : String(stopError)}`)
      }
      this.stateSubscription?.dispose()
      this.stateSubscription = undefined
      this.client = undefined
      throw error
    }
  }

  async stop(): Promise<void> {
    const client = this.client
    if (client === undefined) {
      this.setState('stopped')
      return
    }
    this.setState('stopping')
    try {
      await client.stop()
    } catch (error: unknown) {
      this.log(`LSP shutdown error for workspace ${this.workspace.name}: ${error instanceof Error ? error.message : String(error)}`)
    } finally {
      this.stateSubscription?.dispose()
      this.stateSubscription = undefined
      this.client = undefined
      this.setState('stopped')
    }
  }

  private setState(state: ConnectionState): void {
    this.currentState = state
    this.onState(state)
  }
}
