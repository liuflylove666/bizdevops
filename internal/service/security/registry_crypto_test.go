package security

import "testing"

func TestRegistryPasswordCryptoRoundTrip(t *testing.T) {
	plaintext := "registry-secret-password"
	ciphertext, err := encryptRegistryPassword(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if ciphertext == "" || ciphertext == plaintext {
		t.Fatalf("ciphertext should be non-empty and different from plaintext")
	}

	decrypted, err := decryptRegistryPassword(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted mismatch, got %q want %q", decrypted, plaintext)
	}
}

func TestRegistryPasswordDecryptLegacyPlaintext(t *testing.T) {
	legacy := "plain-password"
	if _, err := decryptRegistryPassword(legacy); err == nil {
		t.Fatalf("expected base64 decode error for legacy plaintext")
	}
}
