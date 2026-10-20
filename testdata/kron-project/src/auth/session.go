// @kron:intent auth/session
package auth

type SessionStore struct{}

func (s *SessionStore) Get(id string) (any, error) { return nil, nil }