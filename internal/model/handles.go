package model

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"unicode"
)

func NormalizeSignature(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

func StableHandle(prefix string, fields ...string) string {
	return stableHandle(prefix, 16, fields...)
}

func StableHandleWithLength(prefix string, length int, fields ...string) string {
	return stableHandle(prefix, length, fields...)
}

func stableHandle(prefix string, length int, fields ...string) string {
	h := sha256.New()
	for _, field := range fields {
		h.Write([]byte{0})
		h.Write([]byte(field))
	}
	digest := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	if length < 8 {
		length = 8
	}
	if length > len(digest) {
		length = len(digest)
	}
	return prefix + "_" + digest[:length]
}

type HandleAllocator struct{ used map[string]string }

func NewHandleAllocator() *HandleAllocator { return &HandleAllocator{used: make(map[string]string)} }

func (a *HandleAllocator) Allocate(prefix string, fields ...string) string {
	identity := strings.Join(fields, "\x00")
	for length := 16; length <= 43; length += 4 {
		handle := StableHandleWithLength(prefix, length, fields...)
		if old, ok := a.used[handle]; !ok || old == identity {
			a.used[handle] = identity
			return handle
		}
	}
	return StableHandleWithLength(prefix, 43, fields...)
}
