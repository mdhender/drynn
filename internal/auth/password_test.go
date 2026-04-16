package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}
	if hash == "correct-horse" {
		t.Fatal("hash must not equal plaintext")
	}
}

func TestHashPassword_UniquePerCall(t *testing.T) {
	h1, _ := HashPassword("same-input")
	h2, _ := HashPassword("same-input")
	if h1 == h2 {
		t.Fatal("two calls with same password should produce different hashes (bcrypt salt)")
	}
}

func TestComparePassword_Correct(t *testing.T) {
	hash, _ := HashPassword("my-secret")
	if err := ComparePassword(hash, "my-secret"); err != nil {
		t.Fatalf("ComparePassword should accept correct password: %v", err)
	}
}

func TestComparePassword_Wrong(t *testing.T) {
	hash, _ := HashPassword("my-secret")
	if err := ComparePassword(hash, "wrong"); err == nil {
		t.Fatal("ComparePassword should reject wrong password")
	}
}

func TestComparePassword_Empty(t *testing.T) {
	hash, _ := HashPassword("non-empty")
	if err := ComparePassword(hash, ""); err == nil {
		t.Fatal("ComparePassword should reject empty password")
	}
}
