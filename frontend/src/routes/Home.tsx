import { createSignal, onMount } from 'solid-js';
import { ipc } from '../ipc/invoke';
import type { Greeting, ProjectMeta } from '../types/ipc';
import { toggleTheme } from '../stores/theme';

/**
 * V0 Home — project card grid + theme toggle.
 * Phase 1 smoke test: invokes `kron_greet` on mount to prove the IPC
 * pipeline is alive, then renders the (empty for now) project list.
 *
 * See `dev-docs/design/05-GUI设计.md` § 1 V0.
 */
export function Home() {
  const [greeting, setGreeting] = createSignal<Greeting | null>(null);
  const [projects, setProjects] = createSignal<ProjectMeta[]>([]);
  const [error, setError] = createSignal<string | null>(null);

  onMount(async () => {
    try {
      const [g, ps] = await Promise.all([
        ipc.kronGreet('Solid frontend'),
        ipc.kronProjectList(),
      ]);
      setGreeting(g);
      setProjects(ps);
    } catch (e) {
      setError(String(e));
    }
  });

  return (
    <main
      style={{
        padding: 'var(--space-8)',
        'max-width': '960px',
        margin: '0 auto',
      }}
    >
      {/* Header */}
      <header
        style={{
          'margin-bottom': 'var(--space-7)',
        }}
      >
        <h1
          style={{
            'font-family': 'var(--font-serif)',
            'font-size': 'var(--text-2xl)',
            'font-weight': 'var(--font-weight-medium)',
            color: 'var(--text-primary)',
            margin: '0 0 var(--space-2) 0',
          }}
        >
          Kron
        </h1>
        <p
          style={{
            color: 'var(--text-secondary)',
            'font-size': 'var(--text-base)',
            margin: 0,
          }}
        >
          项目管理 · Git 之上的任务追踪层
        </p>
      </header>

      {/* Theme toggle */}
      <div style={{ 'margin-bottom': 'var(--space-6)' }}>
        <button
          onClick={toggleTheme}
          style={{
            background: 'var(--bg-surface)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-default)',
            'border-radius': 'var(--radius-md)',
            padding: 'var(--space-2) var(--space-4)',
            'font-family': 'var(--font-sans)',
            'font-size': 'var(--text-sm)',
            cursor: 'pointer',
            transition: 'background var(--duration-fast) var(--ease-out)',
          }}
        >
          切换主题
        </button>
      </div>

      {/* IPC smoke-test output */}
      {error() && (
        <div
          style={{
            background: 'var(--color-danger-bg)',
            color: 'var(--color-danger)',
            padding: 'var(--space-3) var(--space-4)',
            'border-radius': 'var(--radius-md)',
            'margin-bottom': 'var(--space-5)',
            'font-family': 'var(--font-mono)',
            'font-size': 'var(--text-sm)',
          }}
        >
          IPC 错误: {error()}
        </div>
      )}

      {greeting() && (
        <section
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            'border-radius': 'var(--radius-lg)',
            padding: 'var(--space-5)',
            'box-shadow': 'var(--shadow-near)',
            'margin-bottom': 'var(--space-6)',
          }}
        >
          <h2
            style={{
              'font-family': 'var(--font-sans)',
              'font-size': 'var(--text-lg)',
              'font-weight': 'var(--font-weight-semibold)',
              color: 'var(--text-primary)',
              margin: '0 0 var(--space-3) 0',
            }}
          >
            IPC 已连通
          </h2>
          <dl
            style={{
              display: 'grid',
              'grid-template-columns': 'max-content 1fr',
              gap: 'var(--space-2) var(--space-4)',
              margin: 0,
              'font-size': 'var(--text-sm)',
            }}
          >
            <dt style={{ color: 'var(--text-muted)' }}>消息</dt>
            <dd style={{ margin: 0, color: 'var(--text-primary)' }}>
              {greeting()!.message}
            </dd>
            <dt style={{ color: 'var(--text-muted)' }}>后端</dt>
            <dd
              style={{
                margin: 0,
                color: 'var(--text-primary)',
                'font-family': 'var(--font-mono)',
              }}
            >
              {greeting()!.backend}
            </dd>
            <dt style={{ color: 'var(--text-muted)' }}>时间戳</dt>
            <dd
              style={{
                margin: 0,
                color: 'var(--text-secondary)',
                'font-family': 'var(--font-mono)',
              }}
            >
              {greeting()!.timestamp}
            </dd>
          </dl>
        </section>
      )}

      {/* Project card grid */}
      <section>
        <h2
          style={{
            'font-family': 'var(--font-sans)',
            'font-size': 'var(--text-lg)',
            'font-weight': 'var(--font-weight-semibold)',
            color: 'var(--text-primary)',
            margin: '0 0 var(--space-4) 0',
          }}
        >
          项目 ({projects().length})
        </h2>

        {projects().length === 0 ? (
          <div
            style={{
              background: 'var(--bg-surface)',
              border: '1px dashed var(--border-default)',
              'border-radius': 'var(--radius-lg)',
              padding: 'var(--space-8)',
              'text-align': 'center',
              color: 'var(--text-muted)',
              'font-size': 'var(--text-base)',
            }}
          >
            还没有注册的项目。在 Phase 2 中实现 `kron project add` 后即可添加。
          </div>
        ) : (
          <div
            style={{
              display: 'grid',
              'grid-template-columns':
                'repeat(auto-fill, minmax(var(--size-card-min), 1fr))',
              gap: 'var(--space-4)',
            }}
          >
            {projects().map((p) => (
              <article
                style={{
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  'border-radius': 'var(--radius-lg)',
                  padding: 'var(--space-5)',
                  'box-shadow': 'var(--shadow-near)',
                  transition: 'box-shadow var(--duration-fast) var(--ease-out)',
                  cursor: 'pointer',
                }}
              >
                <h3
                  style={{
                    'font-family': 'var(--font-serif)',
                    'font-size': 'var(--text-xl)',
                    'font-weight': 'var(--font-weight-medium)',
                    color: 'var(--text-primary)',
                    margin: '0 0 var(--space-2) 0',
                  }}
                >
                  {p.name}
                </h3>
                <p
                  style={{
                    'font-family': 'var(--font-mono)',
                    'font-size': 'var(--text-xs)',
                    color: 'var(--text-muted)',
                    margin: '0 0 var(--space-3) 0',
                  }}
                >
                  {p.path}
                </p>
                <p
                  style={{
                    'font-size': 'var(--text-sm)',
                    color: 'var(--text-secondary)',
                    margin: 0,
                  }}
                >
                  {p.vertexCount} vertex · {p.taskCount} tasks
                </p>
              </article>
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
