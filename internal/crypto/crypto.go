package crypto

import (
	"encoding/base64"
	"fmt"

	"github.com/3dfosi/gocrypt"
)

// EncryptSecret encrypts plaintext using gocrypt and returns base64-encoded
// ciphertext and salt.
func EncryptSecret(plaintext, passphrase string) (encoded, saltEncoded string, err error) {
	encrypted, salt, err := gocrypt.Encrypt([]byte(plaintext), passphrase)
	if err != nil {
		return "", "", fmt.Errorf("encrypting secret: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted),
		base64.StdEncoding.EncodeToString(salt), nil
}

// DecryptSecret decodes base64-encoded ciphertext and salt, then decrypts
// using gocrypt.
func DecryptSecret(encoded, saltEncoded, passphrase string) (string, error) {
	if saltEncoded == "" {
		return "", fmt.Errorf("secret has no encryption salt")
	}
	encrypted, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding encrypted secret: %w", err)
	}
	salt, err := base64.StdEncoding.DecodeString(saltEncoded)
	if err != nil {
		return "", fmt.Errorf("decoding salt: %w", err)
	}
	decrypted, err := gocrypt.Decrypt(encrypted, salt, passphrase)
	if err != nil {
		return "", fmt.Errorf("decrypting secret: %w", err)
	}
	return string(decrypted), nil
}
