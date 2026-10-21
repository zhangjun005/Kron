# Seeds the shared integration fixture at testdata/kron-project.
#
# The fixture is committed to git so every module (GUI, MCP, LSP, CLI tests)
# can use the same project. This script is the reproducible source of truth for
# its content; re-running it overwrites the fixture.
#
# The fixture deliberately exercises the *whole* intent schema, not just
# created_by/updated_at:
#   - symbol / reviewers / status (active | draft | superseded)
#   - assumptions (B-3: per-intent severity + rationale, registry text in
#     .kron/assumptions/<id>.md)
#   - references (soft) and depends_on (hard) relations
#   - a three-level tree (auth -> tokens -> refresh)
#   - source + markdown anchors
#   - exactly two intentional lint diagnostics, so GUI/MCP error paths have
#     something to render: 1 error (dangling anchor) + 1 warning (severity
#     override soft-vs-hard on auth/session).
#
# Usage:
#   powershell -NoProfile -File frontend/wails/scripts/seed-test-project.ps1
#   powershell -NoProfile -File ... -Root D:\tmp\kron-fixture
param(
    [string]$Root = (Join-Path (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')) 'testdata\kron-project')
)

$ErrorActionPreference = 'Stop'
$utf8 = New-Object System.Text.UTF8Encoding($false)

function Write-Fixture([string]$rel, [string]$content) {
    $full = Join-Path $Root $rel
    New-Item -ItemType Directory -Force -Path (Split-Path $full) | Out-Null
    [System.IO.File]::WriteAllText($full, $content, $utf8)
}

Write-Fixture '.kron/config.toml' "intents_dir = `".kron/intents`"`n"

Write-Fixture '.kron/intents/auth/README.md' @'
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
'@

Write-Fixture '.kron/intents/auth/jwt.md' @'
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
'@

Write-Fixture '.kron/intents/auth/session.md' @'
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
'@

Write-Fixture '.kron/intents/auth/tokens/README.md' @'
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
'@

Write-Fixture '.kron/intents/auth/tokens/refresh.md' @'
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
'@

Write-Fixture '.kron/intents/payments/README.md' @'
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
'@

Write-Fixture '.kron/intents/payments/charge.md' @'
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
'@

Write-Fixture '.kron/intents/payments/refund.md' @'
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
'@

Write-Fixture '.kron/assumptions/single-region.md' @'
<!-- kron:frontmatter -->
id: single-region
text: 服务只部署在单个 region，不存在跨区复制延迟与时钟漂移
default_severity: hard
status: active
created_by: "@zhangjun005"
updated_at: "2026-10-01T08:00:00Z"
expires_at: "2027-01-01"
verified_at: "2026-10-01"
verified_by: "@zhangjun005"
<!-- /kron:frontmatter -->
# Single region

服务只部署在单个 region。假设破裂意味着跨区一致性必须重新设计。
'@

Write-Fixture '.kron/assumptions/idempotency-key.md' @'
<!-- kron:frontmatter -->
id: idempotency-key
text: 支付网关按客户端提供的幂等键去重，重复请求返回首次结果
default_severity: hard
status: active
created_by: "@zhangjun005"
updated_at: "2026-10-01T08:00:00Z"
verified_at: "2026-10-15"
verified_by: "@lanyue811"
<!-- /kron:frontmatter -->
# Idempotency key

支付网关按幂等键去重。假设破裂意味着重试会造成重复扣款。
'@

Write-Fixture 'src/auth/jwt.go' @'
// @kron:intent auth/jwt
package auth

func SignIn(user string) (token string, err error) {
	return "", nil
}
'@

Write-Fixture 'src/auth/session.go' @'
// @kron:intent auth/session
package auth

type SessionStore struct{}

func (s *SessionStore) Get(id string) (any, error) { return nil, nil }
'@

Write-Fixture 'src/auth/refresh.go' @'
// @kron:intent auth/tokens/refresh
package auth

func RefreshToken(refresh string) (string, error) { return "", nil }
'@

Write-Fixture 'src/payments/charge.go' @'
// @kron:intent payments/charge
package payments

func Charge(amount int64) error { return nil }
'@

Write-Fixture 'src/payments/refund.go' @'
// @kron:intent payments/refund
package payments

func Refund(chargeID string) error { return nil }
'@

# Intentional diagnostic #1: dangling source anchor (lint error).
Write-Fixture 'src/broken/orphan.go' @'
// @kron:intent does/not/exist
package broken

func Nothing() {}
'@

Write-Fixture 'docs/adr/0001-jwt-rotation.md' @'
# ADR 0001: JWT key rotation

We rotate JWT signing keys every 30 days. The implementation lives
in auth/jwt; this ADR is the source of truth for the rotation policy.

// @kron:intent auth/jwt
'@

Write-Fixture 'docs/adr/0002-charge-idempotency.md' @'
# ADR 0002: charge idempotency

扣款请求按客户端幂等键去重。

// @kron:intent payments/charge
'@

Write-Fixture 'go.mod' @'
module example.invalid/kron-test-project

go 1.25
'@

Write-Output "seeded test project at $Root"
Write-Output 'expected lint: 1 error (dangling anchor) + 1 warning (severity override)'
