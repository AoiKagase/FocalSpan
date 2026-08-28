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
			parts := strings.Fields(line[len("diff --git "):])
			if len(parts) < 2 {
				return nil, fmt.Errorf("malformed diff header")
			}
			oldPath := stripDiffPrefix(parts[0])
			newPath := stripDiffPrefix(parts[1])
			files = append(files, ChangedFile{Path: newPath, OldPath: oldPath, Kind: ChangeModify})
			current = len(files) - 1
		case current >= 0 && strings.HasPrefix(line, "new file mode"):
			files[current].Kind = ChangeAdd
		case current >= 0 && strings.HasPrefix(line, "deleted file mode"):
			files[current].Kind = ChangeDelete
		case current >= 0 && strings.HasPrefix(line, "rename from "):
			files[current].OldPath = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
			files[current].Kind = ChangeRename
		case current >= 0 && strings.HasPrefix(line, "rename to "):
			files[current].Path = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
			files[current].Kind = ChangeRename
		case current >= 0 && strings.HasPrefix(line, "Binary files "):
			files[current].Binary = true
		case current >= 0 && strings.HasPrefix(line, "+++ "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if path != "/dev/null" {
				files[current].Path = stripDiffPrefix(path)
			}
		case current >= 0:
			matches := hunkPattern.FindStringSubmatch(line)
			if len(matches) == 0 {
				continue
			}
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
	path = strings.TrimSpace(path)
	if len(path) > 2 && (strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/")) {
		return path[2:]
	}
	return path
}
