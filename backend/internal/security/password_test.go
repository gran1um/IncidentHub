package security

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("CorrectHorseBatteryStaple123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	ok, err := VerifyPassword("CorrectHorseBatteryStaple123!", hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatalf("expected password to verify")
	}

	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if ok {
		t.Fatalf("expected wrong password to fail")
	}
}
