package security

import "testing"

func TestHashPasswordDoesNotStorePlaintextAndChecksPassword(t *testing.T) {
	hashed, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hashed == "correct horse battery staple" {
		t.Fatal("HashPassword returned plaintext")
	}
	if err := CheckPassword(hashed, "correct horse battery staple"); err != nil {
		t.Fatalf("CheckPassword rejected the original password: %v", err)
	}
	if err := CheckPassword(hashed, "wrong password"); err == nil {
		t.Fatal("CheckPassword accepted a wrong password")
	}
}

func TestHashPasswordUsesDifferentSalt(t *testing.T) {
	first, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("first HashPassword failed: %v", err)
	}
	second, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("second HashPassword failed: %v", err)
	}
	if first == second {
		t.Fatal("HashPassword produced identical hashes for the same password")
	}
}
