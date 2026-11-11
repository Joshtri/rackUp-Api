package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Passwords are stored as Argon2id PHC strings:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
//
// These are the defaults of the Rust `argon2` crate the API used before,
// so existing hashes keep working. Verification reads the parameters from
// the stored string, never from these constants.
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16
)

var b64 = base64.RawStdEncoding

// HashPassword returns an Argon2id PHC string with a random salt.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("random source failed: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks password against a stored PHC string.
// An error means the stored string itself is broken.
func VerifyPassword(password, phc string) (bool, error) {
	p, err := parsePHC(phc)
	if err != nil {
		return false, fmt.Errorf("stored password hash is invalid: %w", err)
	}
	key := argon2.IDKey([]byte(password), p.salt, p.time, p.memory, p.threads, uint32(len(p.key)))
	return subtle.ConstantTimeCompare(key, p.key) == 1, nil
}

type phcParams struct {
	memory, time uint32
	threads      uint8
	salt, key    []byte
}

func parsePHC(s string) (phcParams, error) {
	var p phcParams
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[0] != "" {
		return p, errors.New("not a PHC string")
	}
	if parts[1] != "argon2id" {
		return p, fmt.Errorf("unsupported algorithm %q", parts[1])
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, fmt.Errorf("unsupported version %q", parts[2])
	}
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return p, fmt.Errorf("bad parameter %q", kv)
		}
		var n uint64
		if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
			return p, fmt.Errorf("bad parameter %q", kv)
		}
		switch k {
		case "m":
			p.memory = uint32(n)
		case "t":
			p.time = uint32(n)
		case "p":
			p.threads = uint8(n)
		}
	}
	if p.memory == 0 || p.time == 0 || p.threads == 0 {
		return p, errors.New("missing parameters")
	}
	var err error
	if p.salt, err = b64.DecodeString(parts[4]); err != nil {
		return p, errors.New("bad salt")
	}
	if p.key, err = b64.DecodeString(parts[5]); err != nil || len(p.key) == 0 {
		return p, errors.New("bad hash")
	}
	return p, nil
}

// dummyHash is checked when the username is unknown, so the response time
// doesn't reveal which usernames exist.
var dummyHash = sync.OnceValue(func() string {
	h, err := HashPassword("not-a-real-password")
	if err != nil {
		panic(err)
	}
	return h
})
