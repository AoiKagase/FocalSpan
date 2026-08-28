package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid server name %q: must match [A-Za-z0-9_-]{1,64}", name)
	}
	return nil
}

func ProjectConfigPath(root string) string {
	return filepath.Join(root, ".codex", "config.toml")
}

func UserConfigPath() (string, error) {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		if strings.IndexByte(home, 0) >= 0 {
			return "", errors.New("CODEX_HOME contains NUL")
		}
		return filepath.Join(home, "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func DefaultServerName(scope, root string) string {
	if scope == ScopeProject {
		return "focalspan"
	}
	base := filepath.Base(filepath.Clean(root))
	base = sanitizeNamePart(base)
	if base == "" {
		base = "repo"
	}
	hash := sha256.Sum256([]byte(filepath.Clean(root)))
	suffix := "-" + hex.EncodeToString(hash[:])[:8]
	prefix := "focalspan-"
	maxBase := 64 - len(prefix) - len(suffix)
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return prefix + base + suffix
}

func sanitizeNamePart(value string) string {
	var b strings.Builder
	lastSeparator := false
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			lastSeparator = r == '-' || r == '_'
			continue
		}
		if !lastSeparator {
			b.WriteByte('-')
			lastSeparator = true
		}
	}
	return strings.Trim(b.String(), "-_")
}
