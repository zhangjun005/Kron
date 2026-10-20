<!-- kron:frontmatter -->
symbol:
  - "payments"
created_by: "@zhangjun005"
updated_at: "2026-10-15T13:23:42Z"
status: active
references:
  - payments/charge
  - payments/refund
<!-- /kron:frontmatter -->
# payments

支付模块的意图总览：扣款幂等、退款与扣款的依赖关系。

## 模块边界

只描述“钱怎么动”：幂等键、扣款与退款的状态机。不涉及风控策略。