package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonKeyLength   = 32
	saltLength       = 16
)

type staticError string

func (e staticError) Error() string {
	return string(e)
}

const (
	errInvalidEncodedHashFormat staticError = "invalid encoded hash format"
	errUnsupportedHashAlgorithm staticError = "unsupported hash algorithm"
	errHashTooLong              staticError = "hash is too long"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt generation failed: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	encSalt := base64.RawStdEncoding.EncodeToString(salt)
	encHash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonIterations, argonParallelism, encSalt, encHash), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, errInvalidEncodedHashFormat
	}
	if parts[1] != "argon2id" {
		return false, errUnsupportedHashAlgorithm
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("invalid hash parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	hashLen := uint64(len(hash))
	if hashLen > uint64(^uint32(0)) {
		return false, errHashTooLong
	}

	calculated := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(hashLen))
	if subtle.ConstantTimeCompare(hash, calculated) == 1 {
		return true, nil
	}
	return false, nil
}
