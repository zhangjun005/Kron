import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

/**
 * App.tsx — Kron VSCode webview entry.
 *
 * PR 1 (this commit) renders only the shadcn 5 first-batch components
 * (Button / Card / Input / Badge / Separator) as a smoke test for
 * RFC 2026-10-18-vscode-skeleton-sdk.md §4 #1-#4 (skeleton + webview
 * scaffold + shadcn init + config). Real webview business logic
 * (`kron_tree` MCP call, intent list, intent detail) is PR 2 / #9
 * (lanyue811 domain) and is intentionally NOT implemented here.
 */
function App(): JSX.Element {
    return (
        <div className="p-6 space-y-4">
            <header className="space-y-1">
                <h1 className="text-2xl font-semibold tracking-tight">Kron Webview Skeleton</h1>
                <p className="text-sm text-muted-foreground">
                    PR 1 smoke test — shadcn/ui 5 first-batch components wired up.
                </p>
            </header>

            <Separator />

            <div className="flex items-center gap-2">
                <Badge variant="default">Webview</Badge>
                <Badge variant="secondary">Skeleton</Badge>
                <Badge variant="outline">PR 1</Badge>
            </div>

            <Card>
                <CardHeader>
                    <CardTitle>Intent Search (placeholder)</CardTitle>
                </CardHeader>
                <CardContent className="space-y-2">
                    <Input placeholder="Search intents… (PR 2: wire to kron_tree MCP tool)" />
                </CardContent>
                <CardFooter className="justify-end gap-2">
                    <Button variant="outline">Cancel</Button>
                    <Button>Search</Button>
                </CardFooter>
            </Card>
        </div>
    )
}

export default App
