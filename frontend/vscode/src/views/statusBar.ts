import vscode = require('vscode')
import type { ConnectionState } from '../mcp/client'

const NO_PROJECT_TEXT = '$(folder) Kron: no .kron project'
const NO_PROJECT_TOOLTIP = 'Open a folder that contains a .kron directory, or check kron.serverPath and the Kron output channel.'

/** Displays the active Kron project, MCP/LSP health, and the restart action. */
export class KronStatusBar implements vscode.Disposable {
  private readonly item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 20)
  private mcpStates: readonly ConnectionState[] = []
  private lspStates: readonly ConnectionState[] = []
  private projectName: string | undefined

  constructor() {
    this.item.command = 'kron.restartConnections'
    this.item.show()
    this.update()
  }

  /** Marks the active single Kron project for this window. */
  setProject(name: string): void {
    this.projectName = name
    this.update()
  }

  /** No workspace folder in this window contains a Kron project. */
  setNoProject(): void {
    this.projectName = undefined
    this.mcpStates = []
    this.lspStates = []
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

  dispose(): void {
    this.item.dispose()
  }

  private update(): void {
    if (this.projectName === undefined) {
      this.item.text = NO_PROJECT_TEXT
      this.item.tooltip = NO_PROJECT_TOOLTIP
      return
    }

    const allStates = [...this.mcpStates, ...this.lspStates]
    const tooltip = [`Project: ${this.projectName}`]
    if (allStates.length === 0) {
      this.item.text = '$(sync~spin) Kron: starting'
      tooltip.push('MCP: starting', 'LSP: starting', 'Click to restart connections.')
      this.item.tooltip = tooltip.join('\n')
      return
    }

    const hasFailure = allStates.includes('failed')
    const isStarting = allStates.some((state) => state === 'starting' || state === 'stopping')
    this.item.text = hasFailure
      ? '$(warning) Kron: connection issue'
      : isStarting
        ? '$(sync~spin) Kron: starting'
        : '$(check) Kron: connected'
    tooltip.push(
      `MCP: ${this.summarize(this.mcpStates)}`,
      `LSP: ${this.summarize(this.lspStates)}`,
      'Click to restart connections.',
    )
    this.item.tooltip = tooltip.join('\n')
  }

  private summarize(states: readonly ConnectionState[]): string {
    if (states.length === 0) return 'stopped'
    if (states.includes('failed')) return 'failed'
    if (states.every((state) => state === 'ready')) return 'ready'
    return states.includes('starting') || states.includes('stopping') ? 'starting' : 'stopped'
  }
}
