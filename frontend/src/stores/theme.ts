import { createSignal } from 'solid-js';

export type Theme = 'light' | 'dark';

const [theme, setThemeSignal] = createSignal<Theme>('light');

export { theme };

export function applyTheme(next: Theme): void {
  document.documentElement.setAttribute('data-theme', next);
  setThemeSignal(next);
  localStorage.setItem('kron-theme', next);
}

export function toggleTheme(): void {
  applyTheme(theme() === 'light' ? 'dark' : 'light');
}

export function initTheme(): void {
  const saved = localStorage.getItem('kron-theme');
  if (saved === 'light' || saved === 'dark') {
    applyTheme(saved);
  } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    applyTheme('dark');
  }
}
