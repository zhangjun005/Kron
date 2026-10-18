import { useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import type { TreeNode, TreeResult, WebviewInbound, WebviewOutbound } from '@/lib/mcp-types'

// VSCode webview API — only available at runtime inside the webview.
// During plain Vite dev (no extension host), `acquireVsCodeApi` is
// undefined; the guard below falls back to a no-op shim so the page
// still renders the empty state.
declare global {
    interface Window {
        acquireVsCodeApi?: () => {
            postMessage: (message: WebviewOutbound) => void
            getState: () => unknown
            setState: (state: unknown) => void
        }
    }
}

const vscodeApi = typeof window.acquireVsCodeApi === 'function'
    ? window.acquireVsCodeApi()
    : null

/**
 * flattenTree — depth-first walk for the tree view rendered by
 * shadcn Card + Badge. Used to fold TreeNode.children into a flat
 * list with depth markers for the basic indent visualization PR 2
 * ships. A real tree component (e.g. via shadcn Collapsible) is
 * v.s. PR scope per RFC 2026-10-18 §3.4.
 */
function flattenTree(node: TreeNode, depth: number, out: Array<{ node: TreeNode; depth: number }>): void {
    out.push({ node, depth })
    if (node.is_dir && node.children !== undefined) {
        for (const child of node.children) {
            flattenTree(child, depth + 1, out)
        }
    }
}

function App(): JSX.Element {
    const [tree, setTree] = useState<TreeResult | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [loading, setLoading] = useState<boolean>(false)
    const [filter, setFilter] = useState<string>('')

    const requestTree = useCallback((): void => {
        if (vscodeApi === null) {
            setError('Not running inside VSCode (window.acquireVsCodeApi unavailable).')
            return
        }
        setLoading(true)
        setError(null)
        vscodeApi.postMessage({ type: 'kron_tree:request' })
    }, [])

    useEffect(() => {
        if (vscodeApi === null) {
            // Outside extension host — show empty state; user runs F5
            // (PR 3 / RFC #10) to test the real path.
            return
        }
        const handler = (event: MessageEvent<WebviewInbound>): void => {
            const msg = event.data
            if (msg.type === 'kron_tree:result') {
                setTree(msg.result)
                setLoading(false)
            } else if (msg.type === 'kron_tree:error') {
                setError(msg.message)
                setLoading(false)
            }
        }
        window.addEventListener('message', handler)
        requestTree()
        return () => {
            window.removeEventListener('message', handler)
        }
    }, [requestTree])

    const flat: Array<{ node: TreeNode; depth: number }> = []
    if (tree !== null) {
        flattenTree(tree.root, 0, flat)
    }
    const filterLc = filter.toLowerCase()
    const visible = filter === ''
        ? flat
        : flat.filter(({ node }) =>
            node.name.toLowerCase().includes(filterLc) ||
            (node.title?.toLowerCase().includes(filterLc) ?? false)
        )

    return (
        <div className="p-6 space-y-4">
            <header className="space-y-1">
                <h1 className="text-2xl font-semibold tracking-tight">Kron Intents</h1>
                <p className="text-sm text-muted-foreground">
                    PR 2 — webview business wired to <code>kron_tree</code> MCP tool via extension host bridge.
                </p>
            </header>

            <div className="flex items-center gap-2">
                <Badge variant="default">Webview</Badge>
                <Badge variant="secondary">{flat.length} nodes</Badge>
                {error !== null ? <Badge variant="destructive">Error</Badge> : null}
                {loading ? <Badge variant="outline">Loading…</Badge> : null}
            </div>

            <Separator />

            <Card>
                <CardHeader>
                    <CardTitle>Filter</CardTitle>
                </CardHeader>
                <CardContent>
                    <Input
                        placeholder="Filter by name or title…"
                        value={filter}
                        onChange={(e) => {
                            setFilter(e.target.value)
                        }}
                    />
                </CardContent>
                <CardFooter className="justify-end gap-2">
                    <Button variant="outline" onClick={requestTree} disabled={loading}>
                        Refresh
                    </Button>
                </CardFooter>
            </Card>

            {error !== null ? (
                <Card>
                    <CardHeader>
                        <CardTitle className="text-destructive">Error</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <p className="text-sm">{error}</p>
                    </CardContent>
                </Card>
            ) : null}

            {tree === null && error === null ? (
                <p className="text-sm text-muted-foreground">
                    {vscodeApi === null
                        ? 'Open this extension in VSCode (F5) to fetch the intent tree.'
                        : 'No intents loaded yet.'}
                </p>
            ) : null}

            <div className="space-y-1">
                {visible.map(({ node, depth }) => (
                    <div
                        key={node.slug}
                        className="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-accent"
                        style={{ paddingLeft: `${depth * 16 + 8}px` }}
                    >
                        <span className="text-sm font-medium">
                            {node.is_dir ? '📁' : '📄'} {node.title ?? node.name}
                        </span>
                        {node.status !== undefined ? (
                            <Badge variant="outline" className="text-xs">
                                {node.status}
                            </Badge>
                        ) : null}
                        <code className="ml-auto text-xs text-muted-foreground">{node.slug}</code>
                    </div>
                ))}
            </div>
        </div>
    )
}

export default App
