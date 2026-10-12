import {useEffect, useState} from 'react';
import logo from './assets/images/logo-universal.png';
import './App.css';
import {GetKronInfo, ListMCPTools} from '../wailsjs/go/main/App';

// Shape of the payload returned by GetKronInfo (mirrors app.go KronInfo).
// We declare it locally instead of importing from the generated bindings
// so the TS surface stays typed even if `wails generate module` is rerun.
type KronInfo = {
    name: string;
    version: string;
    buildTime: string;
    goVersion: string;
    os: string;
    arch: string;
    description: string;
    rfc: string;
};

function App() {
    const [info, setInfo] = useState<KronInfo | null>(null);
    const [tools, setTools] = useState<string[]>([]);
    const [loadError, setLoadError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        (async (): Promise<void> => {
            try {
                const [kronInfo, kronTools] = await Promise.all([
                    GetKronInfo(),
                    ListMCPTools(),
                ]);
                if (cancelled) return;
                setInfo(kronInfo);
                setTools(kronTools);
            } catch (err: unknown) {
                if (cancelled) return;
                const msg = err instanceof Error ? err.message : String(err);
                setLoadError(msg);
            }
        })();

        return (): void => {
            cancelled = true;
        };
    }, []);

    return (
        <div id="App" className="kron-shell">
            <header className="kron-header">
                <img src={logo} id="logo" alt="Kron"/>
                <div className="kron-header-text">
                    <h1>Kron</h1>
                    <p className="kron-tagline">
                        Intent ledger — desktop overview
                    </p>
                </div>
            </header>

            <main className="kron-main">
                {loadError !== null && (
                    <section className="kron-panel kron-error" data-testid="kron-error">
                        <h2>Bridge error</h2>
                        <p>Failed to load Kron info from the Go side:</p>
                        <pre>{loadError}</pre>
                    </section>
                )}

                {info !== null && (
                    <section className="kron-panel" data-testid="kron-info">
                        <h2>Client</h2>
                        <dl className="kron-info-list">
                            <dt>Name</dt>
                            <dd>{info.name}</dd>
                            <dt>Version</dt>
                            <dd>{info.version}</dd>
                            <dt>Build</dt>
                            <dd>{info.buildTime}</dd>
                            <dt>Go</dt>
                            <dd>{info.goVersion}</dd>
                            <dt>Target</dt>
                            <dd>{info.os}/{info.arch}</dd>
                            <dt>RFC</dt>
                            <dd>{info.rfc}</dd>
                        </dl>
                        <p className="kron-description">{info.description}</p>
                    </section>
                )}

                <section className="kron-panel" data-testid="kron-tools">
                    <h2>Planned MCP tools ({tools.length})</h2>
                    <p className="kron-panel-sub">
                        Read + write tools exposed by <code>kron serve-mcp</code>.
                        The Wails GUI calls them over stdio JSON-RPC, not via
                        <code> internal/</code>.
                    </p>
                    {tools.length > 0 ? (
                        <ul className="kron-tool-list">
                            {tools.map((name) => (
                                <li key={name}>
                                    <code>{name}</code>
                                </li>
                            ))}
                        </ul>
                    ) : (
                        <p className="kron-empty">No tools reported yet.</p>
                    )}
                </section>

                <section className="kron-panel kron-panel-muted" data-testid="kron-roadmap">
                    <h2>Roadmap (work 1.3 = skeleton)</h2>
                    <ol className="kron-roadmap-list">
                        <li>1.3 — Wails skeleton (this page)</li>
                        <li>1.4 — <code>wails dev</code> live window + bridge smoke</li>
                        <li>1.5 — <code>wails build</code> cross-platform artefact</li>
                        <li>2.x — shadcn/ui + real <code>kron serve-mcp</code> plumbing</li>
                    </ol>
                </section>
            </main>
        </div>
    );
}

export default App;
