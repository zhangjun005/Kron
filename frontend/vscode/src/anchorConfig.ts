import fs = require('node:fs')
import path = require('node:path')

/** Project-level anchor scanning config: `.kron/anchor-scan.json`. */
export type AnchorScanComment = {
  line?: string
  block?: [string, string] | string[]
}

export type AnchorScanLanguage = {
  extensions?: string[]
  comment?: AnchorScanComment
}

export type AnchorScanConfig = {
  version?: number
  include?: string[]
  exclude?: string[]
  languages?: Record<string, AnchorScanLanguage>
}

export const ANCHOR_SCAN_REL_PATH = '.kron/anchor-scan.json'
export const INTENT_BODY_TEMPLATE_REL_PATH = '.kron/templates/intent-body.md'

/** Languages used when the project has no anchor-scan.json (or no languages). */
export const FALLBACK_LSP_LANGUAGES = [
  'go', 'markdown', 'typescript', 'typescriptreact', 'javascript', 'javascriptreact',
  'vue', 'svelte', 'astro', 'python', 'rust', 'java', 'c', 'cpp', 'csharp', 'kotlin',
  'swift', 'ruby', 'php', 'shellscript', 'yaml',
] as const

const FALLBACK_COMMENT_SYNTAX: Record<string, AnchorScanComment> = {
  go: { line: '//', block: ['/*', '*/'] },
  typescript: { line: '//', block: ['/*', '*/'] },
  javascript: { line: '//', block: ['/*', '*/'] },
  typescriptreact: { line: '//', block: ['{/*', '*/}'] },
  javascriptreact: { line: '//', block: ['{/*', '*/}'] },
  vue: { line: '//', block: ['<!--', '-->'] },
  svelte: { line: '//', block: ['<!--', '-->'] },
  astro: { line: '//', block: ['<!--', '-->'] },
  markdown: { block: ['<!--', '-->'] },
  python: { line: '#' },
  rust: { line: '//', block: ['/*', '*/'] },
  java: { line: '//', block: ['/*', '*/'] },
  c: { line: '//', block: ['/*', '*/'] },
  cpp: { line: '//', block: ['/*', '*/'] },
  csharp: { line: '//', block: ['/*', '*/'] },
  kotlin: { line: '//', block: ['/*', '*/'] },
  swift: { line: '//', block: ['/*', '*/'] },
  ruby: { line: '#' },
  php: { line: '//', block: ['/*', '*/'] },
  shellscript: { line: '#' },
  yaml: { line: '#' },
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function toStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined
  const out = value.filter((entry): entry is string => typeof entry === 'string')
  return out.length > 0 ? out : undefined
}

function parseLanguage(value: unknown): AnchorScanLanguage | undefined {
  if (!isRecord(value)) return undefined
  const extensions = toStringArray(value.extensions)
  const language: AnchorScanLanguage = {}
  if (extensions !== undefined) language.extensions = extensions
  if (isRecord(value.comment)) {
    const comment: AnchorScanComment = {}
    if (typeof value.comment.line === 'string' && value.comment.line !== '') comment.line = value.comment.line
    const block = toStringArray(value.comment.block)
    if (block !== undefined && block.length >= 2) comment.block = [block[0]!, block[1]!]
    if (comment.line !== undefined || comment.block !== undefined) language.comment = comment
  }
  return language
}

/**
 * Reads `.kron/anchor-scan.json` from the project folder. Malformed files fall
 * back to undefined (no filtering) so the editor stays usable; `kron lint`
 * still reports the parse error on the Go side.
 */
export function loadAnchorScanConfig(folderPath: string): AnchorScanConfig | undefined {
  try {
    const raw = fs.readFileSync(path.join(folderPath, ANCHOR_SCAN_REL_PATH), 'utf8')
    const parsed: unknown = JSON.parse(raw)
    if (!isRecord(parsed)) return undefined
    const config: AnchorScanConfig = {}
    if (typeof parsed.version === 'number') config.version = parsed.version
    const include = toStringArray(parsed.include)
    if (include !== undefined) config.include = include
    const exclude = toStringArray(parsed.exclude)
    if (exclude !== undefined) config.exclude = exclude
    if (isRecord(parsed.languages)) {
      const languages: Record<string, AnchorScanLanguage> = {}
      for (const [id, value] of Object.entries(parsed.languages)) {
        const language = parseLanguage(value)
        if (language !== undefined) languages[id] = language
      }
      if (Object.keys(languages).length > 0) config.languages = languages
    }
    return config
  } catch {
    return undefined
  }
}

/** Language IDs for the LSP document selector, derived from the scan config. */
export function lspLanguages(config: AnchorScanConfig | undefined): string[] {
  const languages = config?.languages
  if (languages === undefined || Object.keys(languages).length === 0) return [...FALLBACK_LSP_LANGUAGES]
  return Object.keys(languages)
}

function matchGlob(pattern: string, candidate: string): boolean {
  let regex = '^'
  for (let index = 0; index < pattern.length; index += 1) {
    const char = pattern[index]!
    if (char === '*') {
      if (pattern[index + 1] === '*') {
        index += 1
        if (pattern[index + 1] === '/') {
          index += 1
          regex += '(?:.*/)?'
        } else {
          regex += '.*'
        }
      } else {
        regex += '[^/]*'
      }
    } else if (char === '?') {
      regex += '[^/]'
    } else {
      regex += char.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    }
  }
  regex += '$'
  return new RegExp(regex).test(candidate)
}

function repoRelative(folderPath: string, filePath: string): string {
  const rel = path.relative(folderPath, filePath)
  return rel.split(path.sep).join('/')
}

/** Whether an open document should participate in anchor scanning/decoration. */
export function isAnchorScanAllowed(
  folderPath: string,
  documentPath: string,
  languageId: string,
  config: AnchorScanConfig | undefined,
): boolean {
  if (config === undefined) return true
  const rel = repoRelative(folderPath, documentPath)
  if (config.include !== undefined && config.include.length > 0
    && !config.include.some((pattern) => matchGlob(pattern, rel))) return false
  if (config.exclude?.some((pattern) => matchGlob(pattern, rel)) === true) return false
  const languages = config.languages
  if (languages === undefined || Object.keys(languages).length === 0) return true
  if (languages[languageId] !== undefined) return true
  const lowered = rel.toLowerCase()
  return Object.values(languages).some((language) =>
    (language.extensions ?? []).some((ext) => ext !== '' && lowered.endsWith(ext.toLowerCase())))
}

/** Comment syntax for anchor insertion: project config wins, builtin fallback otherwise. */
export function commentSyntaxFor(languageId: string, config: AnchorScanConfig | undefined): AnchorScanComment | undefined {
  return config?.languages?.[languageId]?.comment ?? FALLBACK_COMMENT_SYNTAX[languageId]
}

export type { AnchorScanConfig as AnchorScanConfigType }
