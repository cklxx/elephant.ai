package crypto

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected hash to be returned")
	}
	ok, err := VerifyPassword("secret", hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatalf("expected password to match")
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("verify password with wrong input: %v", err)
	}
	if ok {
		t.Fatalf("expected mismatch for wrong password")
	}
}

func TestDecodeHashRejectsInvalidFormat(t *testing.T) {
	if _, err := decodeHash("invalid"); err == nil {
		t.Fatalf("expected error for invalid hash format")
	}
}
