// Package mapfile persists Generals .map files (and their .tga previews)
// keyed by the .map's CRC, mirroring the on-disk pattern used by
// pkg/statsfile.
//
// Layout under MapsDir:
//
//	<crc>/
//	  meta.txt        # one line: original X-Map-Name from the upload (e.g. "Maps\Tournament Desert\Tournament Desert.map")
//	  map.map         # the .map file bytes
//	  preview.tga     # the .tga preview bytes (optional; may not exist)
//
// Existence is checked by the directory: if "<MapsDir>/<crc>" exists, we
// already have that map.
package mapfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MapsDir is the directory where map files are stored.
// Set via MAPS_DIR env var or defaults to ./maps/
var MapsDir = "./maps"

func init() {
	if dir := os.Getenv("MAPS_DIR"); dir != "" {
		MapsDir = dir
	}
}

// File-kind constants matching the X-Map-File header values sent by the
// game's StatsUploader.
const (
	KindMap     = "map"
	KindPreview = "preview"
)

// On-disk filenames for the two assets. Fixed names so existence checks
// don't depend on map naming conventions; the original name is preserved
// in meta.txt for /get_map.
const (
	mapFilename     = "map.map"
	previewFilename = "preview.tga"
	metaFilename    = "meta.txt"
)

// MapDir returns the per-CRC storage directory for a given CRC.
func MapDir(crc string) string {
	return filepath.Join(MapsDir, crc)
}

// Exists reports whether we already have any files for this CRC. A
// directory with at least the .map file present counts as "have it".
func Exists(crc string) bool {
	if crc == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(MapDir(crc), mapFilename))
	return err == nil && info.Mode().IsRegular()
}

// Store writes one asset (the .map or its .tga preview) into the CRC dir
// and refreshes meta.txt with the supplied original map name. kind must
// be either KindMap or KindPreview. Calling Store with the same kind
// twice overwrites silently.
func Store(crc string, mapName string, kind string, data []byte) error {
	if crc == "" {
		return errors.New("mapfile.Store: empty crc")
	}
	target, err := assetFilename(kind)
	if err != nil {
		return err
	}

	dir := MapDir(crc)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create map dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, target), data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	// Refresh meta.txt with the most recent name we've seen for this CRC.
	// In normal operation both POSTs for the same map carry the same
	// X-Map-Name, so this is just idempotent overwrite.
	if mapName != "" {
		if err := os.WriteFile(filepath.Join(dir, metaFilename), []byte(mapName), 0644); err != nil {
			return fmt.Errorf("write meta: %w", err)
		}
	}
	return nil
}

// Load returns the stored map name (from meta.txt), the .map bytes, and
// the .tga preview bytes for the given CRC. previewData is nil when the
// preview wasn't uploaded. Returns os.ErrNotExist if the .map is missing.
func Load(crc string) (mapName string, mapData []byte, previewData []byte, err error) {
	if crc == "" {
		return "", nil, nil, errors.New("mapfile.Load: empty crc")
	}
	dir := MapDir(crc)

	mapData, err = os.ReadFile(filepath.Join(dir, mapFilename))
	if err != nil {
		return "", nil, nil, err
	}

	if metaBytes, metaErr := os.ReadFile(filepath.Join(dir, metaFilename)); metaErr == nil {
		mapName = strings.TrimSpace(string(metaBytes))
	}

	if previewBytes, previewErr := os.ReadFile(filepath.Join(dir, previewFilename)); previewErr == nil {
		previewData = previewBytes
	}
	return mapName, mapData, previewData, nil
}

// BaseName extracts the filename stem from an original X-Map-Name. The
// game sends paths with backslashes (e.g. "Maps\Foo\Foo.map"); we want
// "Foo" so /get_map can rename the zip entries to "Foo.map" / "Foo.tga".
func BaseName(mapName string) string {
	if mapName == "" {
		return "map"
	}
	// Strip directory components for both separators (Windows + POSIX).
	cleaned := strings.ReplaceAll(mapName, "\\", "/")
	last := filepath.Base(cleaned)
	// Strip extension.
	if dot := strings.LastIndex(last, "."); dot > 0 {
		last = last[:dot]
	}
	if last == "" {
		return "map"
	}
	return last
}

// assetFilename maps an X-Map-File header value to the on-disk filename.
func assetFilename(kind string) (string, error) {
	switch kind {
	case KindMap:
		return mapFilename, nil
	case KindPreview:
		return previewFilename, nil
	default:
		return "", fmt.Errorf("mapfile: unknown X-Map-File kind %q", kind)
	}
}
