//go:build !((darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64)))

package memory

import (
	"database/sql"

	"fmt"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func openMemoryDB(dbPath string) (*sql.DB, error) {
	db, err := driver.Open(dbPath, func(c *sqlite3.Conn) error {
		for _, pragma := range []string{
			"PRAGMA journal_mode = WAL;",
			"PRAGMA synchronous = NORMAL;",
			"PRAGMA busy_timeout = 5000;",
		} {
			if err := c.Exec(pragma); err != nil {
				return fmt.Errorf("failed to set pragma: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open memory database: %w", err)
	}
	return db, nil
}
