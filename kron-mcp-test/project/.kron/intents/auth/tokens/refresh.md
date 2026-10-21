<!-- kron:frontmatter -->
symbol:
  - "auth.RefreshToken"
created_by: "agent:claude-3.7-sonnet"
updated_at: "2026-10-17T11:45:00Z"
status: draft
references:
  - auth/session
depends_on:
  - auth/jwt
<!-- /kron:frontmatter -->
# refresh

刷新令牌采用一次性轮换：每次使用后旧 refresh 立即作废。

## 为什么（Why）

一次性轮换让重放攻击只能成功一次，且能被下一次刷新发现。

## 权衡（Tradeoffs）

- 客户端必须有并发刷新的去重（否则并发请求互相作废）。