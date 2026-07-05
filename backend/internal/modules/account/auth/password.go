package auth

// Local password hashing (ADR-06). Portal owns credentials: passwords are
// stored as Argon2id PHC strings in users.password_hash. Argon2id is the
// OWASP-recommended password KDF — memory-hard, resistant to GPU/ASIC cracking.
//
// Stored format (self-describing, so params can evolve without a migration):
//
//	$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 key>

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2Params tunes the cost. Defaults target interactive login on a modest
// single VPS (~64 MB, a few hundred ms). Raise memory/iterations as hardware
// allows. Because the params are encoded in every hash, changing them here
// only affects newly-set passwords; old hashes still verify.
type argon2Params struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultArgon2 = argon2Params{
	memory:      64 * 1024, // 64 MB
	iterations:  3,
	parallelism: 2,
	saltLength:  16,
	keyLength:   32,
}

var (
	// ErrInvalidPasswordHash is returned when a stored hash can't be parsed.
	ErrInvalidPasswordHash = errors.New("auth: invalid password hash format")
	// ErrIncompatibleArgon2Version guards against a future argon2 lib bump.
	ErrIncompatibleArgon2Version = errors.New("auth: incompatible argon2 version")
)

// HashPassword returns a PHC-format Argon2id hash for storage. The salt is
// random per call, so hashing the same password twice yields different strings.
func HashPassword(password string) (string, error) {
	p := defaultArgon2
	salt := make([]byte, p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	b64 := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.iterations, p.parallelism,
		b64(salt), b64(key),
	), nil
}

// VerifyPassword reports whether password matches the stored PHC hash, using a
// constant-time comparison. It returns (false, nil) on an honest mismatch and
// (false, err) only when the stored hash is malformed — callers should treat
// both as "auth failed" but may log the err case as data corruption.
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, key, err := decodeArgon2Hash(encoded)
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)
	if subtle.ConstantTimeCompare(computed, key) == 1 {
		return true, nil
	}
	return false, nil
}

func decodeArgon2Hash(encoded string) (argon2Params, []byte, []byte, error) {
	var p argon2Params
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=65536,t=3,p=2", "<salt>", "<key>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	if version != argon2.Version {
		return p, nil, nil, ErrIncompatibleArgon2Version
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	p.saltLength = uint32(len(salt))
	p.keyLength = uint32(len(key))
	return p, salt, key, nil
}
