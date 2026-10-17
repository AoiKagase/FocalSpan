package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/focalspan/focalspan/internal/indexlock"
)

// Every SQLite handle is opened under a lifecycle lease. Changing modes always
// closes SQLite, releases the old lease, acquires anew, and rechecks disk state.
func openWithLease(ctx context.Context, root, directory string, allowUpgrade bool) (result *Store, resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var lease *indexlock.Lock
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, lease.Close())
		}
	}()
	mode := indexlock.Shared
	initializing := false
	dbPath := filepath.Join(directory, "index.db")
	for {
		var err error
		// Another initializer can finish and retain a shared lease while this
		// opener waits. Periodically recheck under shared access instead of waiting
		// exclusively for that perfectly usable reader to close.
		acquireCtx := ctx
		stopAttempt := func() {}
		if mode == indexlock.Exclusive && initializing {
			acquireCtx, stopAttempt = context.WithTimeout(ctx, 100*time.Millisecond)
		}
		lease, err = indexlock.Acquire(acquireCtx, directory, indexlock.Lifecycle, mode)
		stopAttempt()
		if err != nil && mode == indexlock.Exclusive && initializing && errors.Is(err, indexlock.ErrBusy) && ctx.Err() == nil {
			mode = indexlock.Shared
			continue
		}
		if err != nil {
			return nil, err
		}
		if mode == indexlock.Exclusive && allowUpgrade {
			if err := recoverUpgradeArtifacts(dbPath); err != nil {
				return nil, err
			}
		}
		exists := false
		if _, err := os.Stat(dbPath); err == nil {
			exists = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat index database: %w", err)
		}
		version := ""
		if exists {
			version, err = readSchemaVersionPath(ctx, dbPath)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
		artifacts := fileExists(dbPath+".v1.bak") || fileExists(dbPath+".v2.tmp")
		needsExclusive := !exists || (allowUpgrade && (version == "1" || artifacts))
		if mode == indexlock.Shared && needsExclusive {
			if err := lease.Close(); err != nil {
				return nil, err
			}
			mode = indexlock.Exclusive
			initializing = !exists
			continue
		}
		if !exists {
			if artifacts && !allowUpgrade {
				return nil, fmt.Errorf("interrupted index migration; run focalspan update --rebuild")
			}
			created, err := openDatabase(root, dbPath, false)
			if err != nil {
				return nil, err
			}
			if err := created.Close(); err != nil {
				return nil, err
			}
		} else if version == "1" {
			if !allowUpgrade {
				return nil, &SchemaUpgradeRequiredError{Path: dbPath, Version: version}
			}
			tempPath := dbPath + ".v2.tmp"
			if err := removeSQLiteFiles(tempPath); err != nil {
				return nil, fmt.Errorf("remove stale schema v2 temporary database: %w", err)
			}
			s, err := openDatabase(root, tempPath, false)
			if err != nil {
				return nil, errors.Join(err, removeSQLiteFiles(tempPath))
			}
			s.liveDBPath, s.upgradePending, s.lifecycle = dbPath, true, lease
			return s, nil
		} else if version != schemaVersion {
			return nil, unsupportedSchemaVersionError(dbPath, version)
		} else if mode == indexlock.Shared {
			s, err := openDatabase(root, dbPath, true)
			if err != nil {
				return nil, err
			}
			s.lifecycle = lease
			return s, nil
		}
		// Initialization/recovery completed under exclusive protection. Reopen
		// under shared protection, checking again in case another process won.
		if err := lease.Close(); err != nil {
			return nil, err
		}
		mode = indexlock.Shared
	}
}
