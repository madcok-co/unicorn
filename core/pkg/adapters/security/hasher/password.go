package hasher

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidHash      = errors.New("invalid hash format")
	ErrIncompatibleHash = errors.New("incompatible hash version")
	ErrMismatch         = errors.New("password does not match")
)

// PasswordHasher is the interface for password hashing
type PasswordHasher interface {
	// Hash hashes a password
	Hash(password string) (string, error)

	// Verify verifies a password against a hash
	Verify(password, hash string) error

	// NeedsRehash checks if the hash needs to be rehashed (e.g., cost changed)
	NeedsRehash(hash string) bool
}

// ============ Bcrypt Implementation ============

// BcryptConfig adalah konfigurasi untuk bcrypt hasher
type BcryptConfig struct {
	// Cost factor (4-31, default 10)
	Cost int
}

// DefaultBcryptConfig returns default configuration
func DefaultBcryptConfig() *BcryptConfig {
	return &BcryptConfig{
		Cost: bcrypt.DefaultCost,
	}
}

// BcryptHasher implements password hashing using bcrypt
type BcryptHasher struct {
	config *BcryptConfig
}

// NewBcryptHasher creates a new bcrypt hasher
func NewBcryptHasher(config *BcryptConfig) *BcryptHasher {
	if config == nil {
		config = DefaultBcryptConfig()
	}

	// Ensure cost is within valid range
	if config.Cost < bcrypt.MinCost {
		config.Cost = bcrypt.MinCost
	}
	if config.Cost > bcrypt.MaxCost {
		config.Cost = bcrypt.MaxCost
	}

	return &BcryptHasher{
		config: config,
	}
}

// Hash hashes a password using bcrypt
func (h *BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.config.Cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Verify verifies a password against a bcrypt hash
func (h *BcryptHasher) Verify(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrMismatch
		}
		return err
	}
	return nil
}

// NeedsRehash checks if the hash needs to be rehashed
func (h *BcryptHasher) NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true
	}
	return cost != h.config.Cost
}

// GetCost returns the current cost factor
func (h *BcryptHasher) GetCost() int {
	return h.config.Cost
}

// ============ Argon2 Implementation ============

// Argon2Config adalah konfigurasi untuk argon2 hasher
type Argon2Config struct {
	// Memory in KiB
	Memory uint32

	// Number of iterations
	Iterations uint32

	// Parallelism (number of threads)
	Parallelism uint8

	// Salt length in bytes
	SaltLength uint32

	// Key length in bytes
	KeyLength uint32

	// Use Argon2id (default) or Argon2i
	UseArgon2i bool
}

// DefaultArgon2Config returns OWASP recommended configuration
func DefaultArgon2Config() *Argon2Config {
	return &Argon2Config{
		Memory:      64 * 1024, // 64 MiB
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
		UseArgon2i:  false, // Use Argon2id by default
	}
}

// LowMemoryArgon2Config returns configuration for low-memory environments
func LowMemoryArgon2Config() *Argon2Config {
	return &Argon2Config{
		Memory:      32 * 1024, // 32 MiB
		Iterations:  4,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
		UseArgon2i:  false,
	}
}

// Argon2Hasher implements password hashing using Argon2
type Argon2Hasher struct {
	config *Argon2Config
}

// NewArgon2Hasher creates a new Argon2 hasher
func NewArgon2Hasher(config *Argon2Config) *Argon2Hasher {
	if config == nil {
		config = DefaultArgon2Config()
	}

	return &Argon2Hasher{
		config: config,
	}
}

// Hash hashes a password using Argon2
func (h *Argon2Hasher) Hash(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, h.config.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	var hash []byte
	if h.config.UseArgon2i {
		hash = argon2.Key([]byte(password), salt, h.config.Iterations, h.config.Memory, h.config.Parallelism, h.config.KeyLength)
	} else {
		hash = argon2.IDKey([]byte(password), salt, h.config.Iterations, h.config.Memory, h.config.Parallelism, h.config.KeyLength)
	}

	// Encode in PHC format
	// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
	variant := "argon2id"
	if h.config.UseArgon2i {
		variant = "argon2i"
	}

	encoded := fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		variant,
		argon2.Version,
		h.config.Memory,
		h.config.Iterations,
		h.config.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)

	return encoded, nil
}

// Verify verifies a password against an Argon2 hash
func (h *Argon2Hasher) Verify(password, encodedHash string) error {
	// Parse the hash
	params, salt, hash, err := h.decodeHash(encodedHash)
	if err != nil {
		return err
	}

	// Hash the password with the same parameters
	var computed []byte
	if params.UseArgon2i {
		computed = argon2.Key([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	} else {
		computed = argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	}

	// Constant time comparison
	if subtle.ConstantTimeCompare(hash, computed) != 1 {
		return ErrMismatch
	}

	return nil
}

// NeedsRehash checks if the hash needs to be rehashed
func (h *Argon2Hasher) NeedsRehash(encodedHash string) bool {
	params, _, _, err := h.decodeHash(encodedHash)
	if err != nil {
		return true
	}

	return params.Memory != h.config.Memory ||
		params.Iterations != h.config.Iterations ||
		params.Parallelism != h.config.Parallelism ||
		params.KeyLength != h.config.KeyLength
}

// decodeHash decodes an Argon2 hash in PHC format
func (h *Argon2Hasher) decodeHash(encodedHash string) (*Argon2Config, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	// parts[0] is empty (before first $)
	variant := parts[1]
	if variant != "argon2id" && variant != "argon2i" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleHash
	}

	params := &Argon2Config{
		UseArgon2i: variant == "argon2i",
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Iterations, &params.Parallelism); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	params.SaltLength = uint32(len(salt))

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	params.KeyLength = uint32(len(hash))

	return params, salt, hash, nil
}

// GetConfig returns the current configuration
func (h *Argon2Hasher) GetConfig() *Argon2Config {
	return h.config
}

// ============ Multi-Algorithm Hasher ============

// MultiHasher supports multiple hashing algorithms and automatic migration
type MultiHasher struct {
	primary   PasswordHasher
	fallbacks []PasswordHasher
}

// NewMultiHasher creates a hasher that supports multiple algorithms
func NewMultiHasher(primary PasswordHasher, fallbacks ...PasswordHasher) *MultiHasher {
	return &MultiHasher{
		primary:   primary,
		fallbacks: fallbacks,
	}
}

// Hash hashes using the primary algorithm
func (h *MultiHasher) Hash(password string) (string, error) {
	return h.primary.Hash(password)
}

// Verify tries all algorithms
func (h *MultiHasher) Verify(password, hash string) error {
	// Try primary first
	if err := h.primary.Verify(password, hash); err == nil {
		return nil
	}

	// Try fallbacks
	for _, fb := range h.fallbacks {
		if err := fb.Verify(password, hash); err == nil {
			return nil
		}
	}

	return ErrMismatch
}

// NeedsRehash checks if hash is not from primary algorithm
func (h *MultiHasher) NeedsRehash(hash string) bool {
	// If primary can verify, check if it needs rehash
	if h.primary.Verify("", hash) == nil || h.primary.NeedsRehash(hash) {
		return h.primary.NeedsRehash(hash)
	}

	// If hash is from fallback, needs rehash to migrate to primary
	for _, fb := range h.fallbacks {
		if !fb.NeedsRehash(hash) {
			return true // Needs migration to primary
		}
	}

	return true
}

// ============ Simple SHA256 Hasher (for non-password use) ============

// SHA256Hasher is a simple hasher for non-password use cases
type SHA256Hasher struct{}

// NewSHA256Hasher creates a new SHA256 hasher
func NewSHA256Hasher() *SHA256Hasher {
	return &SHA256Hasher{}
}

// Hash hashes data using SHA256 (NOT for passwords!)
func (h *SHA256Hasher) Hash(data string) (string, error) {
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// Verify verifies data against a SHA256 hash
func (h *SHA256Hasher) Verify(data, hash string) error {
	computed, err := h.Hash(data)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) != 1 {
		return ErrMismatch
	}

	return nil
}

// NeedsRehash always returns false for SHA256
func (h *SHA256Hasher) NeedsRehash(hash string) bool {
	return false
}

// Ensure implementations satisfy PasswordHasher interface
var _ PasswordHasher = (*BcryptHasher)(nil)
var _ PasswordHasher = (*Argon2Hasher)(nil)
var _ PasswordHasher = (*MultiHasher)(nil)
var _ PasswordHasher = (*SHA256Hasher)(nil)
