// internal/store/db.go
package store

import (
	"database/sql"
	"embed"
	"os"
	"path/filepath"
	"runtime"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open creates/opens the SQLite DB at dbFileName. Pass an already-resolved
// path (e.g. next to the executable) to use it as-is, or a bare filename to
// fall back to a stable OS-appropriate location - see getDBPath.
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

	if err := initializeSchema(db); err != nil {
		return nil, err
	}
	return db, nil
}

// getDBPath returns the path to use for the database file.
//
// If dbFileName already has a directory component (main.go now passes an
// executable-relative path, e.g. "<exe dir>/data/ragapp.db"), it's used
// as-is - that's what puts the db next to the .exe instead of in the OS
// user-data folder.
//
// If dbFileName is just a bare name with no directory (e.g. "ragapp.db"),
// falls back to the previous stable OS-appropriate location, so any
// existing caller that only ever passed a filename keeps working exactly
// as before:
// Windows: %AppData%/LocalRAGChatBot/data/ragapp.db
// Linux/Mac: ~/.local/share/LocalRAGChatBot/data/ragapp.db
func getDBPath(dbFileName string) (string, error) {
	if dir := filepath.Dir(dbFileName); dir != "." && dir != string(filepath.Separator) {
		return dbFileName, nil
	}

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

func initializeSchema(db *sql.DB) error {
	files := []string{
		"0001_init.sql",
		"0002_fts5.sql",
		"0003_vec.sql",
		"0004_ingestion_queue.sql",
		"0005_workspace_memory.sql",
		"0006_extension_hooks.sql",
		"0007_event_log.sql",
	}

	for _, f := range files {
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return err
		}

		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}