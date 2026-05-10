package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	password := "my-secure-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if err := CheckPassword(password, hash); err != nil {
		t.Errorf("CheckPassword() error = %v", err)
	}
}

func TestCheckPasswordWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPassword("wrong-password", hash); err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestCheckPasswordInvalidHash(t *testing.T) {
	if err := CheckPassword("password", "not-a-valid-hash"); err == nil {
		t.Error("expected error for invalid hash")
	}
}

func TestHashPasswordEmptyString(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword('') error = %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
}
