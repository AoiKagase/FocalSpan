package model

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
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

type HandleAllocator struct {
	used        map[string]string
	occurrences map[string]int
}

func NewHandleAllocator() *HandleAllocator {
	return &HandleAllocator{used: make(map[string]string), occurrences: make(map[string]int)}
}

func (a *HandleAllocator) Allocate(prefix string, fields ...string) string {
	identity := strings.Join(fields, "\x00")
	allocationKey := prefix + "\x00" + identity
	occurrence := a.occurrences[allocationKey]
	for {
		candidateFields := fields
		if occurrence > 0 {
			candidateFields = append(append([]string(nil), fields...), strconv.Itoa(occurrence))
		}
		for length := 16; length <= 43; length += 4 {
			handle := StableHandleWithLength(prefix, length, candidateFields...)
			if _, ok := a.used[handle]; !ok {
				a.used[handle] = identity
				a.occurrences[allocationKey] = occurrence + 1
				return handle
			}
		}
		occurrence++
	}
}
