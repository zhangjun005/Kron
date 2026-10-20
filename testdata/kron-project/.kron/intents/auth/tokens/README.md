<!-- kron:frontmatter -->
symbol:
  - "auth.tokens"
created_by: "agent:claude-3.7-sonnet"
updated_at: "2026-10-17T11:45:00Z"
status: draft
references:
  - auth/tokens/refresh
depends_on:
  - auth/jwt
<!-- /kron:frontmatter -->
# tokens

令牌家族：签发（auth/jwt）与刷新（auth/tokens/refresh）的边界。

## 模块边界

本节点只收口“令牌形态”，签发策略在 auth/jwt，会话在 auth/session。