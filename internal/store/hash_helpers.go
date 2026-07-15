package store

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func HashNormalizedText(text string) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
