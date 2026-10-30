import React from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'
import { applyWebviewFontScale, webviewFontScaleFromBody } from '@/lib/font-size'

// Force dark mode — Kron GUI/IDE use a dark surface.
// TODO(vs.theme-sync): replace with VSCode --vscode-* CSS variables
// when RFC 2026-10-18 §3.4 D2 (v.s. PR) lands.
document.documentElement.classList.add('dark')

// The host renders `kron.webviewFontSize` as data-kron-font-size on <body>.
applyWebviewFontScale(webviewFontScaleFromBody())

const container = document.getElementById('root')
if (container === null) {
    throw new Error('Root container #root not found in index.html')
}

const root = createRoot(container)

root.render(
    <React.StrictMode>
        <App />
    </React.StrictMode>
)
