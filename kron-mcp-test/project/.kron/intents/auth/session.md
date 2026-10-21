<!-- kron:frontmatter -->
symbol:
  - "auth.SessionStore"
created_by: "agent:claude-3.7-sonnet"
updated_at: "2026-10-16T09:10:00Z"
status: draft
reviewers:
  - "@zhangjun005"
assumptions:
  - id: single-region
    severity: soft
    rationale: "会话缓存只影响冷启动耗时，功能仍可用，所以这里降级为软约束"
references:
  - auth/jwt
depends_on:
  - auth/jwt
<!-- /kron:frontmatter -->
# session

会话只存引用（session id → 令牌指纹），不复制令牌本身。

## 为什么（Why）

会话表泄露不应等于令牌泄露；存取分离让审计粒度更细。

## 权衡（Tradeoffs）

- 多一次间接寻址（会话 → 指纹 → 校验），换更小的爆炸半径。