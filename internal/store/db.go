// internal/store/db.go
package store

import (
	"database/sql"
	"embed"
	"os"
	"path/filepath"
	"runtime"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open creates/opens the SQLite DB at a stable OS-appropriate location
// so data survives rebuilds and binary relocations.
func Open(dbFileName string) (*sql.DB, error) {
	sqlite_vec.Auto()

	dbPath, err := getDBPath(dbFileName)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, err
	}
	// Enable WAL mode explicitly
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	if err := runMigrations(db); err != nil {
		return nil, err
	}
	return db, nil
}

// getDBPath returns a stable path for the database file.
// Windows: %AppData%/LocalRAGChatBot/data/ragapp.db
// Linux/Mac: ~/.local/share/LocalRAGChatBot/data/ragapp.db
func getDBPath(dbFileName string) (string, error) {
	var base string

	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		// XDG_DATA_HOME or ~/.local/share
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}

	appDir := filepath.Join(base, "LocalRAGChatBot", "data")
	return filepath.Join(appDir, dbFileName), nil
}

func runMigrations(db *sql.DB) error {
	// Use a tracking table so migrations only run once
	db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY, applied_at INTEGER)`)

	files := []string{"0001_init.sql", "0002_fts5.sql", "0003_vec.sql", "0004_workspace.sql", "0005_chunks_meta.sql"}
	for _, f := range files {
		// Check if already applied
		var count int
		db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = ?", f).Scan(&count)
		if count > 0 {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return err
		}
		// Mark as applied
		db.Exec("INSERT OR IGNORE INTO _migrations (name, applied_at) VALUES (?, ?)", f, time.Now().Unix())
	}
	return nil
}
