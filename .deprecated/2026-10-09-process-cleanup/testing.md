# 测试策略

> 来源：`docs/abstractDesign/architecture.md` §6  
> 本文件是**实施建议**，不是架构真理。测试策略如有变更，请同步修改此处。

---

## 1 分层测试策略

| 层 | 包 | 策略 | 工具 |
|---|---|---|---|
| 对象层 | `internal/model/` | 纯单元测试 | `testing` + `testify/assert` |
| 业务层 | `internal/store/` | tempdir 集成测试（`t.TempDir()`） | 同上 |
| 业务层 | `internal/parser/` | 表驱动单元测试 | 同上 |
| 业务层 | `internal/lint/` | tempdir + 表驱动 | 同上 |
| 访问层 | `cmd/kron/cli/` | cobra `cmd.Execute()` 端到端 | 同上 |
| 访问层 | `cmd/kron/serve-mcp/` | JSON-RPC mock 端到端 | 同上 |

---

## 2 测试 fixture 管理

- 测试 fixture 放在 `testdata/` 目录，git 跟踪
- `testdata/` 下的 `.md` 文件必须是合法的 intent 文件（frontmatter 完整）
- 每个 `internal/` 包有自己的 `testdata/` 子目录

```
internal/
├── model/
│   ├── model_test.go
│   └── testdata/
│       ├── valid-intent.md
│       └── invalid-frontmatter.md
├── store/
│   ├── store_test.go
│   └── testdata/
│       └── .kron/
│           └── intents/
│               └── auth/
│                   └── jwt.md
└── parser/
    ├── parser_test.go
    └── testdata/
        └── anchors.go
```

---

## 3 表驱动测试模板

```go
func TestParseSlug(t *testing.T) {
    cases := []struct {
        name    string
        input   string
        want    string
        wantErr error
    }{
        {"valid single segment", "auth", "auth", nil},
        {"valid multi segment", "auth/jwt", "auth/jwt", nil},
        {"invalid uppercase", "Auth/JWT", "", model.ErrSlugInvalid},
        {"empty", "", "", model.ErrSlugInvalid},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, err := ParseSlug(tc.input)
            if !errors.Is(err, tc.wantErr) {
                t.Errorf("ParseSlug(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
                return
            }
            if got != tc.want {
                t.Errorf("ParseSlug(%q) = %q, want %q", tc.input, got, tc.want)
            }
        })
    }
}
```

---

## 4 集成测试（端到端）

覆盖完整闭环：

```bash
kron init
kron add auth/jwt
# → 手动编辑 .kron/intents/auth/jwt.md
kron lint
```

用 Go 测试实现：

```go
func TestInitAddLint(t *testing.T) {
    tmp := t.TempDir()
    os.Chdir(tmp)

    // kron init
    rootCmd.SetArgs([]string{"init"})
    if err := rootCmd.Execute(); err != nil {
        t.Fatalf("kron init: %v", err)
    }

    // kron add
    rootCmd.SetArgs([]string{"add", "auth/jwt"})
    if err := rootCmd.Execute(); err != nil {
        t.Fatalf("kron add: %v", err)
    }

    // kron lint
    rootCmd.SetArgs([]string{"lint"})
    if err := rootCmd.Execute(); err != nil {
        t.Fatalf("kron lint: %v", err)
    }
}
```

---

## 5 CI 门禁

| 检查 | 命令 | 门禁条件 |
|---|---|---|
| Vet | `go vet ./...` | 0 errors |
| Format | `gofmt -l .` | 0 files |
| Unit + Integration | `go test ./...` | 0 failures |
| Lint（自身） | `go run ./cmd/kron lint` | 0 errors（用 `kron lint` 检查 Kron 自己） |

在 `.github/workflows/ci.yml` 中串联：

```yaml
- run: go vet ./...
- run: gofmt -l .
- run: go test ./...
- run: go run ./cmd/kron lint
```

---

## 6 禁止事项

| 禁止 | 原因 |
|---|---|
| `testing.T.Skip()` 用于跳过已知 bug | 已知 bug 必须在 issue 里追踪，不是跳过测试 |
| 外部网络调用（`net/http` 等）在单元测试中 | 集成测试才允许网络调用 |
| `time.Sleep` 控制时序 | 用 channel / `condition variable` / `httptest.Server` 代替 |
| `reflect.DeepEqual` 比较结构体 | 用 `testify/assert` 的 `Equal` / `EqualValues` 代替 |
