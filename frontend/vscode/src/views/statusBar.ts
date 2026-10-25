import vscode = require('vscode')
import type { ConnectionState } from '../mcp/client'

/** Displays independent MCP/LSP health and offers a single explicit restart action. */
export class KronStatusBar implements vscode.Disposable {
  private readonly item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 20)
  private mcpStates: readonly ConnectionState[] = []
  private lspStates: readonly ConnectionState[] = []

  constructor() {
    this.item.command = 'kron.restartConnections'
    this.item.show()
    this.update()
  }

  setMcpStates(states: readonly ConnectionState[]): void {
    this.mcpStates = states
    this.update()
  }

  setLspStates(states: readonly ConnectionState[]): void {
    this.lspStates = states
    this.update()
  }

  setNoWorkspace(): void {
    this.mcpStates = []
    this.lspStates = []
    this.item.text = '$(folder) Kron: open a workspace'
    this.item.tooltip = 'Open a workspace folder to start Kron MCP and LSP.'
  }

  dispose(): void {
    this.item.dispose()
  }

  private update(): void {
    const allStates = [...this.mcpStates, ...this.lspStates]
    if (allStates.length === 0) {
      this.item.text = '$(sync~spin) Kron: starting'
      this.item.tooltip = 'Starting Kron connections. Click to restart both connections.'
      return
    }

    const hasFailure = allStates.includes('failed')
    const isStarting = allStates.some((state) => state === 'starting' || state === 'stopping')
    this.item.text = hasFailure
      ? '$(warning) Kron: connection issue'
      : isStarting
        ? '$(sync~spin) Kron: starting'
        : '$(check) Kron: connected'
    this.item.tooltip = [
      `MCP: ${this.summarize(this.mcpStates)}`,
      `LSP: ${this.summarize(this.lspStates)}`,
      'Click to restart connections.',
    ].join('\n')
  }

  private summarize(states: readonly ConnectionState[]): string {
    if (states.length === 0) return 'stopped'
    if (states.includes('failed')) return 'failed'
    if (states.every((state) => state === 'ready')) return 'ready'
    return states.includes('starting') || states.includes('stopping') ? 'starting' : 'stopped'
  }
}
