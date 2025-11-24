package hasher

import (
	"strings"
	"testing"
)

func TestBcryptHasher(t *testing.T) {
	hasher := NewBcryptHasher(&BcryptConfig{Cost: 4}) // Low cost for faster tests

	t.Run("hashes password", func(t *testing.T) {
		hash, err := hasher.Hash("password123")
		if err != nil {
			t.Fatalf("failed to hash: %v", err)
		}

		if hash == "password123" {
			t.Error("hash should not equal password")
		}

		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			t.Error("should be bcrypt hash format")
		}
	})

	t.Run("verifies correct password", func(t *testing.T) {
		hash, _ := hasher.Hash("mypassword")

		err := hasher.Verify("mypassword", hash)
		if err != nil {
			t.Errorf("should verify correct password: %v", err)
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		hash, _ := hasher.Hash("correct")

		err := hasher.Verify("wrong", hash)
		if err != ErrMismatch {
			t.Errorf("expected ErrMismatch, got %v", err)
		}
	})

	t.Run("produces different hashes for same password", func(t *testing.T) {
		hash1, _ := hasher.Hash("password")
		hash2, _ := hasher.Hash("password")

		if hash1 == hash2 {
			t.Error("hashes should be different (different salts)")
		}
	})

	t.Run("NeedsRehash detects different cost", func(t *testing.T) {
		lowCost := NewBcryptHasher(&BcryptConfig{Cost: 4})
		highCost := NewBcryptHasher(&BcryptConfig{Cost: 10})

		hash, _ := lowCost.Hash("password")

		if !highCost.NeedsRehash(hash) {
			t.Error("should need rehash with different cost")
		}

		if lowCost.NeedsRehash(hash) {
			t.Error("should not need rehash with same cost")
		}
	})
}

func TestArgon2Hasher(t *testing.T) {
	hasher := NewArgon2Hasher(&Argon2Config{
		Memory:      32 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})

	t.Run("hashes password", func(t *testing.T) {
		hash, err := hasher.Hash("password123")
		if err != nil {
			t.Fatalf("failed to hash: %v", err)
		}

		if !strings.HasPrefix(hash, "$argon2id$") {
			t.Errorf("should be argon2id format, got %s", hash[:20])
		}
	})

	t.Run("verifies correct password", func(t *testing.T) {
		hash, _ := hasher.Hash("mypassword")

		err := hasher.Verify("mypassword", hash)
		if err != nil {
			t.Errorf("should verify correct password: %v", err)
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		hash, _ := hasher.Hash("correct")

		err := hasher.Verify("wrong", hash)
		if err != ErrMismatch {
			t.Errorf("expected ErrMismatch, got %v", err)
		}
	})

	t.Run("produces different hashes for same password", func(t *testing.T) {
		hash1, _ := hasher.Hash("password")
		hash2, _ := hasher.Hash("password")

		if hash1 == hash2 {
			t.Error("hashes should be different")
		}
	})

	t.Run("NeedsRehash detects parameter changes", func(t *testing.T) {
		lowMem := NewArgon2Hasher(&Argon2Config{
			Memory:      16 * 1024,
			Iterations:  1,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		})

		highMem := NewArgon2Hasher(&Argon2Config{
			Memory:      64 * 1024,
			Iterations:  1,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		})

		hash, _ := lowMem.Hash("password")

		if !highMem.NeedsRehash(hash) {
			t.Error("should need rehash with different memory")
		}
	})

	t.Run("handles argon2i variant", func(t *testing.T) {
		argon2i := NewArgon2Hasher(&Argon2Config{
			Memory:      32 * 1024,
			Iterations:  1,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
			UseArgon2i:  true,
		})

		hash, err := argon2i.Hash("password")
		if err != nil {
			t.Fatalf("failed to hash: %v", err)
		}

		if !strings.HasPrefix(hash, "$argon2i$") {
			t.Errorf("should be argon2i format, got %s", hash[:20])
		}

		err = argon2i.Verify("password", hash)
		if err != nil {
			t.Errorf("should verify: %v", err)
		}
	})
}

func TestMultiHasher(t *testing.T) {
	bcrypt := NewBcryptHasher(&BcryptConfig{Cost: 4})
	argon2 := NewArgon2Hasher(&Argon2Config{
		Memory:      32 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})

	// Primary is argon2, fallback is bcrypt
	multi := NewMultiHasher(argon2, bcrypt)

	t.Run("hashes with primary algorithm", func(t *testing.T) {
		hash, _ := multi.Hash("password")

		if !strings.HasPrefix(hash, "$argon2id$") {
			t.Error("should use primary (argon2) for hashing")
		}
	})

	t.Run("verifies with primary algorithm", func(t *testing.T) {
		hash, _ := argon2.Hash("password")

		err := multi.Verify("password", hash)
		if err != nil {
			t.Errorf("should verify argon2 hash: %v", err)
		}
	})

	t.Run("verifies with fallback algorithm", func(t *testing.T) {
		hash, _ := bcrypt.Hash("password")

		err := multi.Verify("password", hash)
		if err != nil {
			t.Errorf("should verify bcrypt hash via fallback: %v", err)
		}
	})

	t.Run("NeedsRehash true for fallback hashes", func(t *testing.T) {
		bcryptHash, _ := bcrypt.Hash("password")

		if !multi.NeedsRehash(bcryptHash) {
			t.Error("should need rehash for fallback algorithm")
		}
	})
}

func TestSHA256Hasher(t *testing.T) {
	hasher := NewSHA256Hasher()

	t.Run("hashes data", func(t *testing.T) {
		hash, err := hasher.Hash("data")
		if err != nil {
			t.Fatalf("failed to hash: %v", err)
		}

		if hash == "data" {
			t.Error("hash should not equal input")
		}
	})

	t.Run("produces consistent hash", func(t *testing.T) {
		hash1, _ := hasher.Hash("same data")
		hash2, _ := hasher.Hash("same data")

		if hash1 != hash2 {
			t.Error("SHA256 should produce consistent hashes")
		}
	})

	t.Run("verifies correctly", func(t *testing.T) {
		hash, _ := hasher.Hash("test")

		err := hasher.Verify("test", hash)
		if err != nil {
			t.Errorf("should verify: %v", err)
		}

		err = hasher.Verify("wrong", hash)
		if err != ErrMismatch {
			t.Errorf("expected ErrMismatch, got %v", err)
		}
	})
}

func BenchmarkHashers(b *testing.B) {
	bcrypt := NewBcryptHasher(&BcryptConfig{Cost: 10})
	argon2 := NewArgon2Hasher(DefaultArgon2Config())

	b.Run("Bcrypt/Hash", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bcrypt.Hash("password123")
		}
	})

	b.Run("Argon2/Hash", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			argon2.Hash("password123")
		}
	})

	bcryptHash, _ := bcrypt.Hash("password123")
	argon2Hash, _ := argon2.Hash("password123")

	b.Run("Bcrypt/Verify", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bcrypt.Verify("password123", bcryptHash)
		}
	})

	b.Run("Argon2/Verify", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			argon2.Verify("password123", argon2Hash)
		}
	})
}
