// Package diagnostic configures process logging for development and packaged builds.
package diagnostic

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Configure sends logs to stderr (so `> exe.log 2>&1` always works). Persistent
// file logging is opt-in: set LOG_FILE to an absolute or relative filename.
func Configure(baseDir string) func() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("")

	path := strings.TrimSpace(os.Getenv("LOG_FILE"))
	if path == "" || strings.EqualFold(path, "off") || strings.EqualFold(path, "stderr") {
		log.SetOutput(os.Stderr)
		return func() {}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("[startup] unable to create log directory for %s: %v", path, err)
		return func() {}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("[startup] unable to open log file %s: %v", path, err)
		return func() {}
	}
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	log.Printf("[startup] logging configured, file=%s", path)
	return func() {
		log.Printf("[shutdown] closing log file")
		_ = file.Close()
	}
}
