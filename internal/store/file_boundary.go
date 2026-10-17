package store

import (
	"errors"
	"fmt"
	"github.com/focalspan/focalspan/internal/repository"
	"os"
	"path/filepath"
)

// validateIndexFiles is run under the lifecycle lease before SQLite or
// migration recovery can follow any database or sidecar path.
func validateIndexFiles(directory string) error {
	for _, base := range []string{"index.db", "index.db.v1.bak", "index.db.v2.tmp"} {
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			name := filepath.Join(directory, base+suffix)
			info, err := os.Lstat(name)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return fmt.Errorf("unsafe index file %s: %w", base+suffix, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unsafe index file %s: database and sidecars must be regular non-link files", base+suffix)
			}
			if _, err := repository.ContainedPath(directory, name); err != nil {
				return fmt.Errorf("unsafe index file %s: %w", base+suffix, err)
			}
		}
	}
	return nil
}
