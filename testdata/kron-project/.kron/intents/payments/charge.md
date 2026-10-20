<!-- kron:frontmatter -->
symbol:
  - "payments.Charge"
created_by: "@zhangjun005"
updated_at: "2026-10-15T13:23:44Z"
status: active
reviewers:
  - "@zhangjun005"
  - "@lanyue811"
assumptions:
  - id: idempotency-key
    severity: hard
    rationale: "重复扣款是资金级事故，任何去重失效都必须先停止扣款，因此按硬约束"
    expires_at: "2026-12-31"
references:
  - payments/refund
depends_on:
  - auth/jwt
<!-- /kron:frontmatter -->
# charge

扣款请求必须携带客户端生成的幂等键，服务端按“幂等键 + 金额”去重。

## 为什么（Why）

网络重试是常态；把去重责任放在客户端生成键上，服务端无需保存请求体。

## 权衡（Tradeoffs）

- 幂等键需要与业务订单绑定，客户端实现错了会静默变成重复扣款。