<!-- kron:frontmatter -->
symbol:
  - "auth.SignIn"
  - "auth.VerifyToken"
created_by: "@zhangjun005"
updated_at: "2026-10-15T13:23:43Z"
status: active
reviewers:
  - "@zhangjun005"
  - "@lanyue811"
assumptions:
  - id: single-region
    severity: hard
    rationale: "吊销窗口对存储写入延迟极敏感，延迟即可视为假设破裂，因此按硬约束处理"
    expires_at: "2027-01-01"
    verified_at: "2026-10-01"
    verified_by: "@zhangjun005"
references:
  - payments/charge
<!-- /kron:frontmatter -->
# jwt

用 Ed25519 签发短期访问令牌，签名密钥每 30 天轮换一次。

<!-- @kron:assumes single-region -->

## 为什么（Why）

短过期 + 非对称签名让吊销成本降到最低：验证方只需公钥，不需要写路径。

## 权衡（Tradeoffs）

- 放弃了“即时全局吊销”：过期前旧令牌仍然有效。
- 代价：每次轮换要给所有验证方分发新公钥（已有 JWKS 端点）。

## 备选（Alternatives）

- 不透明会话 ID + 中心化校验：吊销即时，但每次请求都要回源，放弃无状态。