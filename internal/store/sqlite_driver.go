package store

import (
	"context"
	"database/sql"
	"fmt"

	"modernc.org/sqlite"
)

const storeDriverName = "focalspan-store"

func init() {
	driver := &sqlite.Driver{}
	driver.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		// These settings belong to a physical connection, not to sql.DB. Keep
		// the default SQLite driver untouched for cancellable schema probes.
		for _, pragma := range []string{"PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON", "PRAGMA synchronous=NORMAL"} {
			if _, err := conn.ExecContext(context.Background(), pragma, nil); err != nil {
				return fmt.Errorf("configure sqlite connection: %w", err)
			}
		}
		return nil
	})
	sql.Register(storeDriverName, driver)
}
