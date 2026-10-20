<!-- kron:frontmatter -->
symbol:
  - "auth"
created_by: "@zhangjun005"
updated_at: "2026-10-15T13:23:42Z"
status: active
reviewers:
  - "@zhangjun005"
references:
  - auth/jwt
  - auth/session
  - auth/tokens/refresh
<!-- /kron:frontmatter -->
# auth

auth 模块的意图总览：令牌签发、会话存储、刷新策略。

## 模块边界

本模块只负责“认证事实”：谁是谁、凭证何时过期、刷新时如何轮换。授权（谁能做什么）
不在本模块内。