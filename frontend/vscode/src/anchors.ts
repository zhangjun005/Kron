/*
 * Anchor grammar for the editor side.
 *
 * Truth source: internal/parser/anchors.go (parseAnchorLine) and
 * internal/parser/markdown_anchors.go (fence handling). The Go parser is the
 * authority: it accepts any comment prefix, an anchor anywhere on a line, and
 * optional trailing text. This module mirrors that behaviour so the extension's
 * underline / quick-create never disagrees with `kron lint` or `kron serve-lsp`.
 *
 * Keep the two implementations in sync; see docs/implementation/vscode-extension.md.
 */

const MARKER = '@kron:intent'
const INTENT_EXTENSION = '.md'
const SEGMENT = /^[a-z0-9][a-z0-9_-]*$/

export type AnchorHit = {
  readonly slug: string
  /** 0-based line index. */
  readonly line: number
  /** 0-based UTF-16 character index where the slug starts. */
  readonly character: number
}

type Fence = { readonly marker: '`' | '~'; readonly length: number }

function isIdentCharacter(character: string | undefined): boolean {
  return character !== undefined && /[A-Za-z0-9_]/.test(character)
}

/** Mirrors parser.ValidateSlug for the anchor subset. */
export function isValidAnchorSlug(slug: string): boolean {
  if (slug === '' || slug.includes('\\') || slug.includes('..') || slug.endsWith(INTENT_EXTENSION)) return false
  if (slug === 'README') return true
  return slug.split('/').every((segment) => SEGMENT.test(segment))
}

/** Mirrors parser.findAnchorMarker: the marker must be a whole token. */
function findMarker(line: string): number {
  let from = 0
  for (;;) {
    const index = line.indexOf(MARKER, from)
    if (index < 0) return -1
    const leftOk = index === 0 || !isIdentCharacter(line[index - 1])
    const after = line.slice(index + MARKER.length)
    const rightOk = after[0] === ' ' || after[0] === '\t'
    if (leftOk && rightOk) return index
    from = index + MARKER.length
  }
}

/** Mirrors parser.parseAnchorLine: (slug, start character) or null. */
function parseAnchorLine(line: string): { slug: string; character: number } | null {
  const markerAt = findMarker(line)
  if (markerAt < 0) return null
  let cursor = markerAt + MARKER.length
  while (cursor < line.length && (line[cursor] === ' ' || line[cursor] === '\t')) cursor += 1
  const start = cursor
  while (cursor < line.length && line[cursor] !== ' ' && line[cursor] !== '\t') cursor += 1
  const slug = line.slice(start, cursor)
  if (!isValidAnchorSlug(slug)) return null
  return { slug, character: start }
}

/**
 * Mirrors parser.parseFenceLine.
 *
 * - A fence is three or more backticks/tildes as the first non-whitespace run.
 * - Opening fences may carry an info string; closing fences must not.
 * - Nested fences of a different length are not supported (CommonMark §4.5).
 */
function parseFenceLine(line: string): { fence: Fence; hasInfo: boolean } | null {
  const trimmed = line.replace(/^[ \t]+/, '')
  if (trimmed.length < 3) return null
  const marker = trimmed[0]
  if (marker !== '`' && marker !== '~') return null
  let count = 0
  while (count < trimmed.length && trimmed[count] === marker) count += 1
  if (count < 3) return null
  const rest = trimmed.slice(count).trim()
  return { fence: { marker, length: count }, hasInfo: rest !== '' }
}

/**
 * Returns every anchor outside Markdown code fences. Non-Markdown documents
 * have no fence concept, matching the Go parser.
 */
export function findAnchors(text: string, markdown: boolean): AnchorHit[] {
  const lines = text.split(/\r?\n/)
  const hits: AnchorHit[] = []
  let fence: Fence | null = null
  for (let line = 0; line < lines.length; line += 1) {
    const row = lines[line] ?? ''
    if (markdown) {
      const parsed = parseFenceLine(row)
      if (fence === null) {
        if (parsed !== null) {
          fence = parsed.fence
          continue
        }
      } else if (parsed !== null && parsed.fence.marker === fence.marker
        && parsed.fence.length >= fence.length && !parsed.hasInfo) {
        fence = null
        continue
      } else {
        continue
      }
    }
    const anchor = parseAnchorLine(row)
    if (anchor !== null) hits.push({ slug: anchor.slug, line, character: anchor.character })
  }
  return hits
}
