// Package sharecode turns a folder key into the string a person sends to a
// friend, and back again.
package sharecode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
)

// KeyLen is the length of a folder key in bytes.
const KeyLen = 32

// Prefix starts every share code.
const Prefix = "lynx-"

const checksumLen = 4

var enc = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// ErrBadCode reports a share code that is damaged or mistyped.
var ErrBadCode = errors.New("share code is not valid")

// NewKey returns a new random folder key.
func NewKey() ([]byte, error) {
	key := make([]byte, KeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	return key, nil
}

// Encode returns the share code for a key.
func Encode(key []byte) string {
	body := make([]byte, 0, KeyLen+checksumLen)
	body = append(body, key...)
	body = append(body, checksum(key)...)
	return Prefix + enc.EncodeToString(body)
}

// Decode returns the key inside a share code. It accepts upper case, spaces
// and dashes, because people retype codes by hand.
func Decode(code string) ([]byte, error) {
	clean := strings.ToLower(strings.TrimSpace(code))
	clean = strings.TrimPrefix(clean, Prefix)
	clean = strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "").Replace(clean)
	if clean == "" {
		return nil, ErrBadCode
	}
	body, err := enc.DecodeString(clean)
	if err != nil {
		return nil, ErrBadCode
	}
	if len(body) != KeyLen+checksumLen {
		return nil, ErrBadCode
	}
	key := body[:KeyLen]
	if !equal(checksum(key), body[KeyLen:]) {
		return nil, ErrBadCode
	}
	return key, nil
}

func checksum(key []byte) []byte {
	sum := sha256.Sum256(append([]byte("lynx-share-code:"), key...))
	return sum[:checksumLen]
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
