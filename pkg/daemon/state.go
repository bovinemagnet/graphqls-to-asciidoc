package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Daemon modes reported on the dashboard.
const (
	ModeWatching = "watching"
	ModeBuilding = "building"
)

// historyLimit caps the number of build records kept in memory.
const historyLimit = 20

// tempFilePerm is the permission the atomically-written output file is left
// with, matching the default umask-free mode used elsewhere in the tool.
const tempFilePerm = 0o644

// BuildRecord is one completed build, successful or not.
type BuildRecord struct {
	Started  time.Time
	Duration time.Duration
	Bytes    int
	Err      string
}

// Succeeded reports whether the build produced a document.
func (r BuildRecord) Succeeded() bool { return r.Err == "" }

// KrokiStatus is the result of the most recent reachability probe.
type KrokiStatus struct {
	Enabled   bool
	Reachable bool
	LastCheck time.Time
	URL       string
}

// State is everything the dashboard renders. It is always handed out as a copy.
type State struct {
	Mode      string
	Backend   string
	LastBuild time.Time
	LastError string
	Watched   []string
	History   []BuildRecord // newest first
	Kroki     KrokiStatus
}

// writeAtomic writes data to a temporary file in the destination directory and
// renames it over the target, so a reader never sees a half-written document.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("failed to create a temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, tempFilePerm); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to set permissions on %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}
