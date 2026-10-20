<!-- kron:frontmatter -->
symbol:
  - "payments.Refund"
created_by: "@zhangjun005"
updated_at: "2026-10-15T13:23:44Z"
status: superseded
reviewers:
  - "@zhangjun005"
depends_on:
  - payments/charge
<!-- /kron:frontmatter -->
# refund

退款按“原扣款 ID”全额退回；部分退款拆成多条退款意图后再落地。

## 为什么（Why）

全额退款是唯一不引入资金对账歧义的最小语义；部分退款需要单独的对账规则。

## 权衡（Tradeoffs）

- 放弃部分退款的首发支持，用户需要拆单。

## 备选（Alternatives）

- 直接支持部分退款：对账规则复杂度过高，已标记 superseded，等支付网关支持分摊后再评估。