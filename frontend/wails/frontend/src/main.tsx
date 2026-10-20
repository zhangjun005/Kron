import React from 'react'
import {createRoot} from 'react-dom/client'
import './index.css'
import App from './App'

// Force dark mode — Kron GUI uses a dark surface.
document.documentElement.classList.add('dark')

const container = document.getElementById('root')
if (container === null) {
    throw new Error('Root container #root not found in index.html')
}

const root = createRoot(container)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
