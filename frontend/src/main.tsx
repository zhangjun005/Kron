import { render } from 'solid-js/web';
import { App } from './App';
import './styles/global.css';
import { initTheme } from './stores/theme';

// Initialize theme from localStorage / OS preference before mount
initTheme();

const root = document.getElementById('root');
if (!root) throw new Error('Root element not found');

render(() => <App />, root);
