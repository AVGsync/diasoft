package security

type Hasher interface {
	Hash(password string) (string, error)
	Compare(plain, hashed string) bool
}
