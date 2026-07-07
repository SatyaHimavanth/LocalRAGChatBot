// internal/store/db.go
package store

import (
    "database/sql"
    "embed"
    "os"
    "path/filepath"

    sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
    _ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open resolves dbFileName next to the running executable — NOT the current
// working directory. Same reasoning as the exe/gguf portability discussion
// earlier: a shortcut or double-click won't guarantee your working directory,
// but os.Executable() always points at where the binary actually lives.
func Open(dbFileName string) (*sql.DB, error) {
    sqlite_vec.Auto()

    exePath, err := os.Executable()
    if err != nil {
        return nil, err
    }
    dbPath := filepath.Join(filepath.Dir(exePath), "data", dbFileName)
    if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
        return nil, err
    }

    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }
    if err := runMigrations(db); err != nil {
        return nil, err
    }
    return db, nil
}

func runMigrations(db *sql.DB) error {
    files := []string{"migrations/0001_init.sql", "migrations/0002_fts5.sql", "migrations/0003_vec.sql"}
    for _, f := range files {
        sqlBytes, err := migrationsFS.ReadFile(f)
        if err != nil {
            return err
        }
        if _, err := db.Exec(string(sqlBytes)); err != nil {
            return err
        }
    }
    return nil
}