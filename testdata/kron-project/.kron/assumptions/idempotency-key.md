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