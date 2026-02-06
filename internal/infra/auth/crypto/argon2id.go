package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2idParams defines the tuning parameters for Argon2id hashing.
type Argon2idParams struct {
	Time       uint32
	Memory     uint32
	Threads    uint8
	KeyLength  uint32
	SaltLength uint32
}

// DefaultParams matches the settings used for refresh-token hashing.
var DefaultParams = Argon2idParams{
	Time:       1,
	Memory:     64 * 1024,
	Threads:    4,
	KeyLength:  32,
	SaltLength: 16,
}

// HashArgon2id hashes the supplied value with Argon2id using the provided parameters.
func HashArgon2id(value string, params Argon2idParams) (string, error) {
	if params.Time == 0 {
		params.Time = DefaultParams.Time
	}
	if params.Memory == 0 {
		params.Memory = DefaultParams.Memory
	}
	if params.Threads == 0 {
		params.Threads = DefaultParams.Threads
	}
	if params.KeyLength == 0 {
		params.KeyLength = DefaultParams.KeyLength
	}
	if params.SaltLength == 0 {
		params.SaltLength = DefaultParams.SaltLength
	}

	salt := make([]byte, int(params.SaltLength))
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(value), salt, params.Time, params.Memory, params.Threads, params.KeyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s", params.Time, params.Memory, params.Threads, b64Salt, b64Hash), nil
}

// HashValue hashes the value using DefaultParams.
func HashValue(value string) (string, error) {
	return HashArgon2id(value, DefaultParams)
}

// VerifyArgon2id compares a plain value against an encoded Argon2id hash.
func VerifyArgon2id(value, encoded string) (bool, error) {
	decoded, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(value), decoded.salt, decoded.params.Time, decoded.params.Memory, decoded.params.Threads, uint32(len(decoded.hash)))
	if len(computed) != len(decoded.hash) {
		return false, nil
	}
	var diff byte
	for i := range computed {
		diff |= computed[i] ^ decoded.hash[i]
	}
	return diff == 0, nil
}

type decodedHash struct {
	params Argon2idParams
	salt   []byte
	hash   []byte
}

func decodeHash(encoded string) (decodedHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return decodedHash{}, fmt.Errorf("invalid hash format")
	}
	if parts[0] != "argon2id" {
		return decodedHash{}, fmt.Errorf("unsupported hash algorithm: %s", parts[0])
	}
	timeValue, err := parseUint32(parts[1])
	if err != nil {
		return decodedHash{}, fmt.Errorf("invalid time parameter: %w", err)
	}
	memoryValue, err := parseUint32(parts[2])
	if err != nil {
		return decodedHash{}, fmt.Errorf("invalid memory parameter: %w", err)
	}
	threadsValue, err := parseUint32(parts[3])
	if err != nil {
		return decodedHash{}, fmt.Errorf("invalid threads parameter: %w", err)
	}
	if threadsValue == 0 || threadsValue > 255 {
		return decodedHash{}, errors.New("invalid thread count: must be between 1 and 255")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return decodedHash{}, fmt.Errorf("decode salt: %w", err)
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return decodedHash{}, fmt.Errorf("decode hash: %w", err)
	}
	return decodedHash{
		params: Argon2idParams{
			Time:       timeValue,
			Memory:     memoryValue,
			Threads:    uint8(threadsValue),
			KeyLength:  uint32(len(hash)),
			SaltLength: uint32(len(salt)),
		},
		salt: salt,
		hash: hash,
	}, nil
}

func parseUint32(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}

// HashPassword hashes an end-user password using Argon2id.
func HashPassword(password string) (string, error) {
	return HashValue(password)
}

// VerifyPassword checks whether the plain password matches the encoded hash.
func VerifyPassword(password, encoded string) (bool, error) {
	return VerifyArgon2id(password, encoded)
}
