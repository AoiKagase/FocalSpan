package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/model"
)

type Scanner struct {
	Root   string
	Config config.Config
}

func NewScanner(root string, cfg config.Config) *Scanner {
	return &Scanner{Root: filepath.Clean(root), Config: cfg}
}

func (s *Scanner) Scan(ctx context.Context) ([]model.SourceFile, []model.Diagnostic, error) {
	paths, err := s.paths(ctx)
	if err != nil {
		return nil, nil, err
	}
	files := make([]model.SourceFile, 0, len(paths))
	diagnostics := make([]model.Diagnostic, 0)
	for _, rel := range paths {
		if err := ctx.Err(); err != nil {
			return files, diagnostics, err
		}
		if s.excluded(rel) {
			continue
		}
		full := filepath.Join(s.Root, filepath.FromSlash(rel))
		contained, err := ContainedPath(s.Root, full)
		if err != nil {
			diagnostics = append(diagnostics, model.Diagnostic{FilePath: rel, Level: "warning", Code: "path_escape", Message: "file resolved outside repository root"})
			continue
		}
		info, err := os.Stat(contained)
		if err != nil {
			diagnostics = append(diagnostics, model.Diagnostic{FilePath: rel, Level: "warning", Code: "stat_failed", Message: "unable to stat file"})
			continue
		}
		if info.Size() > s.maxFileBytes() {
			diagnostics = append(diagnostics, model.Diagnostic{FilePath: rel, Level: "info", Code: "file_too_large", Message: "file exceeds configured maximum size"})
			continue
		}
		content, err := os.ReadFile(contained)
		if err != nil {
			diagnostics = append(diagnostics, model.Diagnostic{FilePath: rel, Level: "warning", Code: "read_failed", Message: "unable to read file"})
			continue
		}
		if hasNUL(content) {
			diagnostics = append(diagnostics, model.Diagnostic{FilePath: rel, Level: "info", Code: "binary_skipped", Message: "binary file skipped"})
			continue
		}
		if bytesHasBOM(content) {
			content = content[3:]
		}
		if !utf8.Valid(content) {
			diagnostics = append(diagnostics, model.Diagnostic{FilePath: rel, Level: "warning", Code: "invalid_utf8", Message: "invalid UTF-8 file skipped"})
			continue
		}
		digest := sha256.Sum256(content)
		files = append(files, model.SourceFile{Path: rel, Language: DetectLanguageContent(rel, content), Content: content, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content))})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, diagnostics, nil
}

func (s *Scanner) paths(ctx context.Context) ([]string, error) {
	if IsGitRepository(ctx, s.Root) {
		cmd := exec.CommandContext(ctx, "git", "-C", s.Root, "ls-files", "-co", "--exclude-standard", "-z")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git file listing: %w", err)
		}
		parts := strings.Split(string(out), "\x00")
		paths := make([]string, 0, len(parts))
		for _, part := range parts {
			if part != "" {
				paths = append(paths, filepath.ToSlash(filepath.Clean(part)))
			}
		}
		return paths, nil
	}
	paths := make([]string, 0)
	err := filepath.WalkDir(s.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == s.Root {
			return nil
		}
		rel, err := filepath.Rel(s.Root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if s.excluded(rel + "/") {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, fmt.Errorf("walk repository: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func (s *Scanner) maxFileBytes() int64 {
	if s.Config.MaxFileBytes > 0 {
		return s.Config.MaxFileBytes
	}
	return 2 << 20
}

func (s *Scanner) excluded(path string) bool {
	p := filepath.ToSlash(strings.TrimSuffix(path, "/"))
	if p == "" {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		switch part {
		case ".git", ".focalspan", "node_modules", "vendor", "third_party", "dist", "build", "target", "bin", "obj", ".idea", ".vs", ".vscode", "coverage", "generated":
			return true
		}
	}
	base := filepath.Base(p)
	for _, pattern := range []string{"*.min.js", "*.map", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum"} {
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
	}
	secret := base == ".env" || strings.HasPrefix(base, ".env.") || base == "id_rsa" || base == "id_ed25519"
	if !secret {
		for _, pattern := range []string{"*.pem", "*.key", "credentials.json", "secrets.json"} {
			if ok, _ := filepath.Match(pattern, base); ok {
				secret = true
				break
			}
		}
	}
	if secret && s.Config.SecretExcludesEnabled && !s.explicitlyIncluded(p) {
		return true
	}
	for _, pattern := range s.Config.Exclude {
		if matchPath(pattern, p) {
			return true
		}
	}
	return false
}

func (s *Scanner) explicitlyIncluded(path string) bool {
	for _, pattern := range s.Config.Include {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

func matchPath(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "/")+"/")
	}
	return strings.HasPrefix(path, strings.TrimSuffix(pattern, "/")+"/")
}

func hasNUL(b []byte) bool {
	limit := len(b)
	if limit > 8192 {
		limit = 8192
	}
	for _, v := range b[:limit] {
		if v == 0 {
			return true
		}
	}
	return false
}

func bytesHasBOM(b []byte) bool { return len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf }

func DetectLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".c":
		return "c"
	case ".cc", ".cpp", ".cxx", ".c++", ".h", ".hh", ".hpp", ".hxx", ".inl", ".ipp", ".tpp", ".ixx", ".cppm":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".php":
		return "php"
	case ".phtml", ".php3", ".php4", ".php5", ".php7", ".php8", ".phps":
		return "php"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".ps1":
		return "powershell"
	case ".sh", ".bash":
		return "shell"
	case ".md", ".markdown":
		return "markdown"
	case ".json", ".yaml", ".yml", ".toml", ".xml":
		return "config"
	default:
		return "text"
	}
}

func DetectLanguageContent(path string, content []byte) string {
	if !strings.EqualFold(filepath.Ext(path), ".inc") {
		return DetectLanguage(path)
	}
	if containsPHPTag(content) {
		return "php"
	}
	return "text"
}

func containsPHPTag(content []byte) bool {
	lower := strings.ToLower(string(content))
	for offset := 0; offset+1 < len(lower); offset++ {
		if lower[offset:offset+2] != "<?" {
			continue
		}
		rest := lower[offset+2:]
		if strings.HasPrefix(rest, "php") {
			if len(rest) == 3 || !isIdentifierByte(rest[3]) {
				return true
			}
			continue
		}
		if strings.HasPrefix(rest, "xml") {
			continue
		}
		return true
	}
	return false
}

func isIdentifierByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_'
}
