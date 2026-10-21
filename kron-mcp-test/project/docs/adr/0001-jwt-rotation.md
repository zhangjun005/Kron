# ADR 0001: JWT key rotation

We rotate JWT signing keys every 30 days. The implementation lives
in auth/jwt; this ADR is the source of truth for the rotation policy.

// @kron:intent auth/jwt