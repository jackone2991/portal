package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	const pw = "correct horse battery staple"

	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Fatalf("hash not PHC argon2id: %q", h)
	}
	if strings.Contains(h, pw) {
		t.Fatal("hash must not contain the plaintext")
	}

	// correct password verifies
	if ok, err := VerifyPassword(pw, h); err != nil || !ok {
		t.Fatalf("VerifyPassword(correct) = %v, %v; want true, nil", ok, err)
	}

	// wrong password fails (false, nil) — a mismatch is not an error
	if ok, err := VerifyPassword("not the password", h); err != nil || ok {
		t.Fatalf("VerifyPassword(wrong) = %v, %v; want false, nil", ok, err)
	}

	// a second hash of the same password uses a fresh salt → differs
	h2, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if h2 == h {
		t.Fatal("expected per-call random salt to produce distinct hashes")
	}
	if ok, _ := VerifyPassword(pw, h2); !ok {
		t.Fatal("second hash should also verify")
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	for _, bad := range []string{
		"",
		"plain",
		"$argon2id$v=19$m=65536,t=3,p=2$onlysalt", // missing key segment
		"$bcrypt$whatever",                          // wrong algorithm
	} {
		if ok, err := VerifyPassword("x", bad); ok || err == nil {
			t.Errorf("VerifyPassword(_, %q) = %v, %v; want false, non-nil", bad, ok, err)
		}
	}
}
