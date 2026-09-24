# 错误模型实现参考

> 来源：`docs/abstractDesign/architecture.md` §4  
> 本文件是**实施建议**，不是架构真理。错误名如有变更，请同步修改此处。

---

## 1 Sentinel 错误清单

```go
var (
    ErrIntentNotFound     = errors.New("intent not found")
    ErrIntentExists       = errors.New("intent already exists")
    ErrAnchorDangling     = errors.New("anchor points to non-existent intent")
    ErrFrontmatterInvalid = errors.New("frontmatter is invalid")
    ErrSlugInvalid        = errors.New("intent slug is invalid")
    ErrConfigInvalid      = errors.New("config file is invalid")
)
```

---

## 2 包装与判别规则

| 规则 | 示例 |
|---|---|
| 所有错误用 `fmt.Errorf("...: %w", err)` 包装，附带操作上下文 | `fmt.Errorf("write intent %s: %w", slug, store.ErrIntentExists)` |
| 调用方用 `errors.Is(err, store.ErrIntentNotFound)` 判别 | — |
| 永不 `==` 比较错误 | — |
| 不丢弃错误（无 `_, _ = ...`） | — |

---

## 3 错误码与退出码对照

| 错误 | CLI 退出码 | MCP JSON-RPC error code |
|---|---|---|
| `ErrIntentNotFound` | 1 | `-32602`（Invalid params） |
| `ErrIntentExists` | 1 | `-32602` |
| `ErrAnchorDangling` | 1 | `-32603`（Internal error） |
| `ErrFrontmatterInvalid` | 1 | `-32603` |
| `ErrSlugInvalid` | 1 | `-32602` |
| `ErrConfigInvalid` | 2（内部失败） | `-32603` |
| IO / 解析异常（非 sentinel） | 2（内部失败） | `-32603` |

> CLI 退出码约定：`0` = 通过，`1` = 有 lint 错误，`2` = 内部失败。

---

## 4 错误在 lint 输出中的标注

lint 阶段发现的 `ErrAnchorDangling` 和 `ErrFrontmatterInvalid` 需要带位置信息：

```
[error] anchor dangling: @kron:intent auth/jwt at src/auth.go:42
[error] frontmatter invalid: missing required field 'created_by' in .kron/intents/auth/jwt.md
```

调用方身份（`ctx` 中的 caller key）用于：
- lint 输出里标注调用方
- 未来审计日志（v1 不实现，但接口要留）

---

## 5 与其他包的关系

- `Err*` 定义在 `internal/model/errors.go`（sentinel errors 与 domain model 同包）
- store 层 / parser 层 / lint 层用 `fmt.Errorf` 包装后返回
- CLI / MCP 访问层不定义新错误，只 wrap 和判别已有 sentinel
