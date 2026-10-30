// @kron:intent auth/jwt
package auth

// Claims is the decoded access-token payload.
type Claims struct {
	Subject string
	Expiry  int64
}
