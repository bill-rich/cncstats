// Package logfile persists per-match client log files, keyed by the game
// seed (the same identifier used by pkg/statsfile), grouped by the player
// that produced them. It mirrors the on-disk pattern used by pkg/mapfile.
//
// Layout under LogsDir:
//
//	<seed>/
//	  <player>/
//	    <filename>      # one uploaded log file, original basename preserved
//	    ...
//	  <player>/
//	    ...
//
// A match "exists" once its seed directory has been created by at least
// one successful upload.
package logfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LogsDir is the directory where log files are stored.
// Set via LOGS_DIR env var or defaults to ./logs/
var LogsDir = "./logs"

func init() {
	if dir := os.Getenv("LOGS_DIR"); dir != "" {
		LogsDir = dir
	}
}

// LogFile describes one stored log file for a match.
type LogFile struct {
	Player string `json:"player"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
}

// MatchDir returns the per-seed storage directory for a match. The seed is
// reduced to a safe single path segment; use safeSeed when the caller needs
// to know that the seed was unusable.
func MatchDir(seed string) string {
	safe, err := sanitizeComponent(seed)
	if err != nil {
		return filepath.Join(LogsDir, "invalid")
	}
	return filepath.Join(LogsDir, safe)
}

// safeSeed validates the seed as a storage key. Seeds are client-supplied
// (an X-Game-Seed header) and are not always numeric: connection-failure
// uploads, which have no match to key off, use a "connfail-YYYYMMDD" bucket.
// Anything that isn't a single usable path segment is rejected rather than
// silently rewritten, so a traversal attempt can never land outside LogsDir.
func safeSeed(seed string) (string, error) {
	if seed == "" {
		return "", errors.New("logfile: empty seed")
	}
	return sanitizeComponent(seed)
}

// sanitizeComponent reduces an untrusted path component (player id or
// filename) to a safe single-segment name, guarding against path
// traversal. Returns an error if nothing usable remains.
func sanitizeComponent(raw string) (string, error) {
	// The game runs on Windows and may send backslash separators; treat
	// both separators as directory boundaries before taking the basename.
	cleaned := strings.ReplaceAll(raw, "\\", "/")
	base := filepath.Base(cleaned)
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." {
		return "", fmt.Errorf("logfile: invalid path component %q", raw)
	}
	return base, nil
}

// Store writes one uploaded log file under <seed>/<player>/<filename>.
// player and filename are sanitized to their basenames; a repeated
// (seed, player, filename) overwrites silently.
func Store(seed, player, filename string, data []byte) error {
	safeSeedName, err := safeSeed(seed)
	if err != nil {
		return err
	}
	safePlayer, err := sanitizeComponent(player)
	if err != nil {
		return err
	}
	safeName, err := sanitizeComponent(filename)
	if err != nil {
		return err
	}

	dir := filepath.Join(LogsDir, safeSeedName, safePlayer)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, safeName), data, 0644); err != nil {
		return fmt.Errorf("write log file: %w", err)
	}
	return nil
}

// Exists reports whether any logs have been stored for the given seed.
func Exists(seed string) bool {
	safeSeedName, err := safeSeed(seed)
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(LogsDir, safeSeedName))
	return err == nil && info.IsDir()
}

// List returns every stored log file for a match, sorted by player then
// filename. Returns an empty slice if the match has no logs.
func List(seed string) ([]LogFile, error) {
	safeSeedName, err := safeSeed(seed)
	if err != nil {
		return nil, err
	}
	matchDir := filepath.Join(LogsDir, safeSeedName)
	players, err := os.ReadDir(matchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogFile{}, nil
		}
		return nil, fmt.Errorf("read match dir: %w", err)
	}

	out := []LogFile{}
	for _, p := range players {
		if !p.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(matchDir, p.Name()))
		if err != nil {
			return nil, fmt.Errorf("read player dir: %w", err)
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			info, err := f.Info()
			if err != nil {
				return nil, fmt.Errorf("stat log file: %w", err)
			}
			out = append(out, LogFile{
				Player: p.Name(),
				Name:   f.Name(),
				Size:   info.Size(),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Player != out[j].Player {
			return out[i].Player < out[j].Player
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Load returns the raw bytes of one stored log file. Returns
// os.ErrNotExist if the file isn't stored.
func Load(seed, player, filename string) ([]byte, error) {
	safeSeedName, err := safeSeed(seed)
	if err != nil {
		return nil, err
	}
	safePlayer, err := sanitizeComponent(player)
	if err != nil {
		return nil, err
	}
	safeName, err := sanitizeComponent(filename)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(LogsDir, safeSeedName, safePlayer, safeName))
}
