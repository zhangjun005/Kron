import { marked } from 'marked'
import { useMemo } from 'react'

export function MarkdownPreview({ body }: { body: string }): JSX.Element {
  const html = useMemo(() => {
    // Escape raw HTML from repository content before parsing in the Webview.
    const safeSource = body.replace(/</g, '&lt;')
    return marked.parse(safeSource, { gfm: true, breaks: true })
  }, [body])
  return <div className="markdown-body" dangerouslySetInnerHTML={{ __html: html }} />
}
