package gitx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ChangeKind string

const (
	ChangeModify ChangeKind = "modify"
	ChangeAdd    ChangeKind = "add"
	ChangeDelete ChangeKind = "delete"
	ChangeRename ChangeKind = "rename"
)

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ChangedFile struct {
	Path    string      `json:"path"`
	OldPath string      `json:"old_path,omitempty"`
	Kind    ChangeKind  `json:"kind"`
	Binary  bool        `json:"binary"`
	Ranges  []LineRange `json:"ranges,omitempty"`
}

var hunkPattern = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

func ParseUnifiedZeroDiff(data []byte) ([]ChangedFile, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	files := make([]ChangedFile, 0)
	current := -1
	inHunk := false
	finish := func() {
		if current < 0 {
			return
		}
		if files[current].Kind == "" {
			files[current].Kind = ChangeModify
		}
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			finish()
			inHunk = false
			oldPath, newPath, err := diffHeaderPaths(line[len("diff --git "):])
			if err != nil {
				return nil, err
			}
			files = append(files, ChangedFile{Path: newPath, OldPath: oldPath, Kind: ChangeModify})
			current = len(files) - 1
		case current >= 0 && strings.HasPrefix(line, "new file mode"):
			files[current].Kind = ChangeAdd
		case current >= 0 && strings.HasPrefix(line, "deleted file mode"):
			files[current].Kind = ChangeDelete
		case current >= 0 && strings.HasPrefix(line, "rename from "):
			path, err := decodeGitPath(strings.TrimPrefix(line, "rename from "))
			if err != nil {
				return nil, err
			}
			files[current].OldPath = path
			files[current].Kind = ChangeRename
		case current >= 0 && strings.HasPrefix(line, "rename to "):
			path, err := decodeGitPath(strings.TrimPrefix(line, "rename to "))
			if err != nil {
				return nil, err
			}
			files[current].Path = path
			files[current].Kind = ChangeRename
		case current >= 0 && strings.HasPrefix(line, "Binary files "):
			files[current].Binary = true
		case current >= 0 && !inHunk && (strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ")):
			path, err := decodeGitPath(line[4:])
			if err != nil {
				return nil, err
			}
			if path != "/dev/null" {
				if line[0] == '+' {
					files[current].Path = stripDiffPrefix(path)
				} else {
					files[current].OldPath = stripDiffPrefix(path)
				}
			}
		case current >= 0:
			matches := hunkPattern.FindStringSubmatch(line)
			if len(matches) == 0 {
				continue
			}
			inHunk = true
			start, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("parse hunk start: %w", err)
			}
			count := 1
			if matches[2] != "" {
				count, err = strconv.Atoi(matches[2])
				if err != nil {
					return nil, fmt.Errorf("parse hunk count: %w", err)
				}
			}
			if count > 0 {
				files[current].Ranges = append(files[current].Ranges, LineRange{Start: start, End: start + count - 1})
			}
		}
	}
	finish()
	return files, nil
}

func stripDiffPrefix(path string) string {
	if len(path) > 2 && (strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/")) {
		return path[2:]
	}
	return path
}

func decodeGitPath(raw string) (string, error) {
	if strings.HasPrefix(raw, "\"") {
		path, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("decode Git path: %w", err)
		}
		return path, nil
	}
	// Git terminates unquoted ---/+++ paths containing spaces with a tab.
	return strings.TrimSuffix(raw, "\t"), nil
}

func diffHeaderPaths(header string) (string, string, error) {
	var firstOld, firstNew string
	for i := 0; i < len(header); i++ {
		if header[i] != ' ' {
			continue
		}
		oldPath, oldErr := decodeGitPath(header[:i])
		newPath, newErr := decodeGitPath(header[i+1:])
		if oldErr != nil || newErr != nil || !strings.HasPrefix(oldPath, "a/") || !strings.HasPrefix(newPath, "b/") {
			continue
		}
		oldPath, newPath = stripDiffPrefix(oldPath), stripDiffPrefix(newPath)
		if oldPath == newPath {
			return oldPath, newPath, nil
		}
		if firstOld == "" {
			firstOld, firstNew = oldPath, newPath
		}
	}
	if firstOld != "" {
		// Rename metadata or ---/+++ headers disambiguate paths with spaces.
		return firstOld, firstNew, nil
	}
	return "", "", fmt.Errorf("malformed diff header")
}
