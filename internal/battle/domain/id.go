package domain

import (
	"crypto/rand"
	"encoding/hex"
)

// NextID generates a new battle ID. ID creation is a domain
// responsibility; repositories only store what they are given.
func NextID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing means the platform is broken
	}
	return hex.EncodeToString(b)
}
