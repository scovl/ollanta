package auth

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 10

// HashPassword hashes a plain-text password using bcrypt.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compares a plain-text password against a bcrypt hash.
// Returns nil on match, an error otherwise.
func CheckPassword(plain, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
