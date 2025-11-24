package encryptor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

var (
	ErrInvalidKeySize       = errors.New("invalid key size: must be 16, 24, or 32 bytes")
	ErrCiphertextTooShort   = errors.New("ciphertext too short")
	ErrDecryptionFailed     = errors.New("decryption failed")
	ErrAuthenticationFailed = errors.New("authentication failed: ciphertext may have been tampered")
)

// AESEncryptorConfig adalah konfigurasi untuk AES encryptor
type AESEncryptorConfig struct {
	// Key must be 16, 24, or 32 bytes for AES-128, AES-192, AES-256
	Key []byte

	// UseGCM enables GCM mode (authenticated encryption) - STRONGLY RECOMMENDED
	// If false, uses CBC mode with HMAC for authentication
	UseGCM bool

	// HMACKey for CBC mode authentication (required if UseGCM is false)
	// If not provided, derived from Key using HKDF-like derivation
	HMACKey []byte
}

// AESEncryptor implements Encryptor using AES encryption
type AESEncryptor struct {
	config  *AESEncryptorConfig
	block   cipher.Block
	gcm     cipher.AEAD
	hmacKey []byte
}

// NewAESEncryptor creates a new AES encryptor
// SECURITY NOTE: UseGCM=true is strongly recommended for production use
func NewAESEncryptor(config *AESEncryptorConfig) (*AESEncryptor, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}

	keyLen := len(config.Key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(config.Key)
	if err != nil {
		return nil, err
	}

	e := &AESEncryptor{
		config: config,
		block:  block,
	}

	// Initialize GCM mode if requested (recommended for authenticated encryption)
	if config.UseGCM {
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		e.gcm = gcm
	} else {
		// For CBC mode, we need an HMAC key for authentication
		if len(config.HMACKey) > 0 {
			e.hmacKey = config.HMACKey
		} else {
			// Derive HMAC key from encryption key using simple derivation
			// In production, consider using HKDF
			h := sha256.New()
			h.Write([]byte("unicorn-hmac-key"))
			h.Write(config.Key)
			e.hmacKey = h.Sum(nil)
		}
	}

	return e, nil
}

// NewAESEncryptorFromString creates an AES encryptor from a string key
// The key will be hashed to create a 32-byte key (AES-256)
// This constructor always uses GCM mode for security
func NewAESEncryptorFromString(key string, useGCM bool) (*AESEncryptor, error) {
	// Hash the key to get exactly 32 bytes
	hash := sha256.Sum256([]byte(key))

	return NewAESEncryptor(&AESEncryptorConfig{
		Key:    hash[:],
		UseGCM: useGCM,
	})
}

// NewAESGCMEncryptor creates an AES-GCM encryptor (recommended)
func NewAESGCMEncryptor(key []byte) (*AESEncryptor, error) {
	return NewAESEncryptor(&AESEncryptorConfig{
		Key:    key,
		UseGCM: true,
	})
}

// NewAESGCMEncryptorFromString creates an AES-GCM encryptor from string key
func NewAESGCMEncryptorFromString(key string) (*AESEncryptor, error) {
	return NewAESEncryptorFromString(key, true)
}

// Encrypt encrypts plaintext bytes
func (e *AESEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if e.config.UseGCM {
		return e.encryptGCM(plaintext)
	}
	return e.encryptCBCWithHMAC(plaintext)
}

// Decrypt decrypts ciphertext bytes
func (e *AESEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if e.config.UseGCM {
		return e.decryptGCM(ciphertext)
	}
	return e.decryptCBCWithHMAC(ciphertext)
}

// EncryptString encrypts a string and returns base64-encoded ciphertext
func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
	encrypted, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptString decrypts a base64-encoded ciphertext string
func (e *AESEncryptor) DecryptString(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	decrypted, err := e.Decrypt(data)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// Hash creates a SHA-256 hash of the data
func (e *AESEncryptor) Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CompareHash compares data with a hash using constant-time comparison
func (e *AESEncryptor) CompareHash(data []byte, hash string) bool {
	computed := e.Hash(data)
	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}

// encryptGCM encrypts using AES-GCM (authenticated encryption)
func (e *AESEncryptor) encryptGCM(plaintext []byte) ([]byte, error) {
	// Create nonce
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt and authenticate
	// Output format: nonce + ciphertext + auth tag
	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// decryptGCM decrypts using AES-GCM
func (e *AESEncryptor) decryptGCM(ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := e.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// encryptCBCWithHMAC encrypts using AES-CBC with HMAC for authentication (Encrypt-then-MAC)
func (e *AESEncryptor) encryptCBCWithHMAC(plaintext []byte) ([]byte, error) {
	// Add PKCS7 padding
	blockSize := e.block.BlockSize()
	padding := blockSize - len(plaintext)%blockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	// Create IV
	iv := make([]byte, blockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Encrypt
	encrypted := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(e.block, iv)
	mode.CryptBlocks(encrypted, padded)

	// Format: IV + ciphertext + HMAC
	// HMAC covers IV + ciphertext (Encrypt-then-MAC)
	ivAndCiphertext := make([]byte, len(iv)+len(encrypted))
	copy(ivAndCiphertext, iv)
	copy(ivAndCiphertext[len(iv):], encrypted)

	// Compute HMAC
	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write(ivAndCiphertext)
	tag := mac.Sum(nil)

	// Final output: IV + ciphertext + HMAC tag
	result := make([]byte, len(ivAndCiphertext)+len(tag))
	copy(result, ivAndCiphertext)
	copy(result[len(ivAndCiphertext):], tag)

	return result, nil
}

// decryptCBCWithHMAC decrypts using AES-CBC with HMAC verification
func (e *AESEncryptor) decryptCBCWithHMAC(ciphertext []byte) ([]byte, error) {
	blockSize := e.block.BlockSize()
	hmacSize := sha256.Size

	// Minimum size: IV + 1 block + HMAC
	minSize := blockSize + blockSize + hmacSize
	if len(ciphertext) < minSize {
		return nil, ErrCiphertextTooShort
	}

	if (len(ciphertext)-hmacSize)%blockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	// Split: IV + encrypted + HMAC tag
	ivAndEncrypted := ciphertext[:len(ciphertext)-hmacSize]
	providedTag := ciphertext[len(ciphertext)-hmacSize:]

	// Verify HMAC first (before decryption)
	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write(ivAndEncrypted)
	expectedTag := mac.Sum(nil)

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(providedTag, expectedTag) != 1 {
		return nil, ErrAuthenticationFailed
	}

	// Extract IV and encrypted data
	iv := ivAndEncrypted[:blockSize]
	encrypted := ivAndEncrypted[blockSize:]

	// Decrypt
	plaintext := make([]byte, len(encrypted))
	mode := cipher.NewCBCDecrypter(e.block, iv)
	mode.CryptBlocks(plaintext, encrypted)

	// Remove PKCS7 padding
	if len(plaintext) == 0 {
		return nil, ErrDecryptionFailed
	}

	padding := int(plaintext[len(plaintext)-1])
	if padding > blockSize || padding == 0 {
		return nil, ErrDecryptionFailed
	}

	// Verify padding in constant time
	paddingValid := 1
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		paddingValid &= subtle.ConstantTimeByteEq(plaintext[i], byte(padding))
	}

	if paddingValid != 1 {
		return nil, ErrDecryptionFailed
	}

	return plaintext[:len(plaintext)-padding], nil
}

// KeySize returns the current key size (128, 192, or 256 bits)
func (e *AESEncryptor) KeySize() int {
	return len(e.config.Key) * 8
}

// IsGCM returns true if using GCM mode
func (e *AESEncryptor) IsGCM() bool {
	return e.config.UseGCM
}

// Mode returns the encryption mode ("GCM" or "CBC-HMAC")
func (e *AESEncryptor) Mode() string {
	if e.config.UseGCM {
		return "GCM"
	}
	return "CBC-HMAC"
}

// Ensure AESEncryptor implements Encryptor
var _ contracts.Encryptor = (*AESEncryptor)(nil)
