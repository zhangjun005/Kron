/*
 * Shared font-size plumbing for the Webview.
 *
 * The extension host renders `kron.webviewFontSize` as `data-kron-font-size` on
 * <body>. Tailwind sizes are rem-based, so the setting is applied by scaling the
 * document root; the default (14) maps to a 16px root, i.e. the pre-setting look.
 */

const BASE_WEBVIEW_FONT_PX = 14
const DEFAULT_ROOT_FONT_PX = 16
export const MIN_WEBVIEW_FONT_PX = 10
export const MAX_WEBVIEW_FONT_PX = 24

/** Reads the host-provided setting; falls back to the documented default. */
export function webviewFontScaleFromBody(): number {
    const raw = document.body.dataset.kronFontSize
    if (raw === undefined) return 1
    const size = Number(raw)
    if (!Number.isFinite(size) || size < MIN_WEBVIEW_FONT_PX || size > MAX_WEBVIEW_FONT_PX) return 1
    return size / BASE_WEBVIEW_FONT_PX
}

/** Applies a scale where 1 == the documented default font size. */
export function applyWebviewFontScale(scale: number): void {
    const clamped = Math.min(MAX_WEBVIEW_FONT_PX / BASE_WEBVIEW_FONT_PX, Math.max(MIN_WEBVIEW_FONT_PX / BASE_WEBVIEW_FONT_PX, scale))
    document.documentElement.style.fontSize = `${(clamped * DEFAULT_ROOT_FONT_PX).toFixed(2)}px`
}
