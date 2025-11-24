package encryptor

import (
	"bytes"
	"testing"
)

func TestNewAESEncryptor(t *testing.T) {
	t.Run("creates encryptor with valid key sizes", func(t *testing.T) {
		keySizes := []int{16, 24, 32}
		for _, size := range keySizes {
			key := make([]byte, size)
			_, err := NewAESEncryptor(&AESEncryptorConfig{
				Key:    key,
				UseGCM: true,
			})
			if err != nil {
				t.Errorf("failed to create encryptor with %d byte key: %v", size, err)
			}
		}
	})

	t.Run("rejects invalid key sizes", func(t *testing.T) {
		invalidSizes := []int{8, 15, 17, 31, 33, 64}
		for _, size := range invalidSizes {
			key := make([]byte, size)
			_, err := NewAESEncryptor(&AESEncryptorConfig{
				Key:    key,
				UseGCM: true,
			})
			if err != ErrInvalidKeySize {
				t.Errorf("expected ErrInvalidKeySize for %d byte key, got %v", size, err)
			}
		}
	})

	t.Run("creates from string", func(t *testing.T) {
		enc, err := NewAESEncryptorFromString("my-secret-key", true)
		if err != nil {
			t.Fatalf("failed to create from string: %v", err)
		}
		if enc.KeySize() != 256 {
			t.Errorf("expected 256-bit key, got %d", enc.KeySize())
		}
	})
}

func TestAESEncryptor_GCM(t *testing.T) {
	enc, _ := NewAESEncryptorFromString("test-secret-key", true)

	t.Run("encrypts and decrypts bytes", func(t *testing.T) {
		plaintext := []byte("Hello, World!")

		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		if bytes.Equal(plaintext, ciphertext) {
			t.Error("ciphertext should not equal plaintext")
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("decrypted text doesn't match: got %s, want %s", decrypted, plaintext)
		}
	})

	t.Run("encrypts and decrypts strings", func(t *testing.T) {
		plaintext := "Secret message!"

		ciphertext, err := enc.EncryptString(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		if ciphertext == plaintext {
			t.Error("ciphertext should not equal plaintext")
		}

		decrypted, err := enc.DecryptString(ciphertext)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("decrypted text doesn't match: got %s, want %s", decrypted, plaintext)
		}
	})

	t.Run("produces different ciphertext for same plaintext", func(t *testing.T) {
		plaintext := []byte("Same message")

		ciphertext1, _ := enc.Encrypt(plaintext)
		ciphertext2, _ := enc.Encrypt(plaintext)

		if bytes.Equal(ciphertext1, ciphertext2) {
			t.Error("ciphertexts should be different (different nonces)")
		}
	})

	t.Run("detects tampered ciphertext", func(t *testing.T) {
		plaintext := []byte("Original message")
		ciphertext, _ := enc.Encrypt(plaintext)

		// Tamper with ciphertext
		ciphertext[len(ciphertext)-1] ^= 0xFF

		_, err := enc.Decrypt(ciphertext)
		if err == nil {
			t.Error("should detect tampering")
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		ciphertext, err := enc.Encrypt([]byte{})
		if err != nil {
			t.Fatalf("encryption of empty data failed: %v", err)
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if len(decrypted) != 0 {
			t.Error("decrypted empty data should be empty")
		}
	})

	t.Run("handles large input", func(t *testing.T) {
		plaintext := make([]byte, 1024*1024) // 1MB
		for i := range plaintext {
			plaintext[i] = byte(i % 256)
		}

		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Error("large data decryption mismatch")
		}
	})
}

func TestAESEncryptor_CBC(t *testing.T) {
	enc, _ := NewAESEncryptorFromString("test-secret-key", false) // CBC mode

	t.Run("encrypts and decrypts with CBC-HMAC", func(t *testing.T) {
		plaintext := []byte("Hello, CBC World!")

		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("decrypted text doesn't match")
		}
	})

	t.Run("detects tampered ciphertext in CBC mode", func(t *testing.T) {
		plaintext := []byte("Original message")
		ciphertext, _ := enc.Encrypt(plaintext)

		// Tamper with ciphertext (not the HMAC)
		ciphertext[20] ^= 0xFF

		_, err := enc.Decrypt(ciphertext)
		if err != ErrAuthenticationFailed {
			t.Errorf("expected ErrAuthenticationFailed, got %v", err)
		}
	})

	t.Run("rejects truncated ciphertext", func(t *testing.T) {
		plaintext := []byte("Test message")
		ciphertext, _ := enc.Encrypt(plaintext)

		// Truncate ciphertext
		_, err := enc.Decrypt(ciphertext[:10])
		if err == nil {
			t.Error("should reject truncated ciphertext")
		}
	})
}

func TestAESEncryptor_Hash(t *testing.T) {
	enc, _ := NewAESEncryptorFromString("test-key", true)

	t.Run("produces consistent hash", func(t *testing.T) {
		data := []byte("test data")

		hash1 := enc.Hash(data)
		hash2 := enc.Hash(data)

		if hash1 != hash2 {
			t.Error("hash should be consistent")
		}
	})

	t.Run("produces different hash for different data", func(t *testing.T) {
		hash1 := enc.Hash([]byte("data1"))
		hash2 := enc.Hash([]byte("data2"))

		if hash1 == hash2 {
			t.Error("different data should produce different hashes")
		}
	})

	t.Run("CompareHash works correctly", func(t *testing.T) {
		data := []byte("test data")
		hash := enc.Hash(data)

		if !enc.CompareHash(data, hash) {
			t.Error("CompareHash should return true for matching data")
		}

		if enc.CompareHash([]byte("other data"), hash) {
			t.Error("CompareHash should return false for non-matching data")
		}
	})
}

func TestAESEncryptor_Mode(t *testing.T) {
	gcmEnc, _ := NewAESEncryptorFromString("key", true)
	cbcEnc, _ := NewAESEncryptorFromString("key", false)

	if gcmEnc.Mode() != "GCM" {
		t.Errorf("expected GCM mode, got %s", gcmEnc.Mode())
	}

	if cbcEnc.Mode() != "CBC-HMAC" {
		t.Errorf("expected CBC-HMAC mode, got %s", cbcEnc.Mode())
	}
}

func TestAESEncryptor_CrossKeyDecryption(t *testing.T) {
	enc1, _ := NewAESEncryptorFromString("key1", true)
	enc2, _ := NewAESEncryptorFromString("key2", true)

	plaintext := []byte("Secret message")
	ciphertext, _ := enc1.Encrypt(plaintext)

	// Should fail to decrypt with different key
	_, err := enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("should fail to decrypt with wrong key")
	}
}

func BenchmarkAESEncryptor_GCM(b *testing.B) {
	enc, _ := NewAESEncryptorFromString("benchmark-key", true)
	plaintext := make([]byte, 1024) // 1KB

	b.Run("Encrypt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			enc.Encrypt(plaintext)
		}
	})

	ciphertext, _ := enc.Encrypt(plaintext)
	b.Run("Decrypt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			enc.Decrypt(ciphertext)
		}
	})
}

func BenchmarkAESEncryptor_CBC(b *testing.B) {
	enc, _ := NewAESEncryptorFromString("benchmark-key", false)
	plaintext := make([]byte, 1024)

	b.Run("Encrypt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			enc.Encrypt(plaintext)
		}
	})

	ciphertext, _ := enc.Encrypt(plaintext)
	b.Run("Decrypt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			enc.Decrypt(ciphertext)
		}
	})
}
