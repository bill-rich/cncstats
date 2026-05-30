// Package mapfile persists Generals .map files (plus their .tga preview
// and any sidecar files) keyed by the .map's CRC, mirroring the on-disk
// pattern used by pkg/statsfile.
//
// Layout under MapsDir:
//
//	<crc>/
//	  meta.txt         # one line: original X-Map-Name from the upload (e.g. "Maps\Tournament Desert\Tournament Desert.map")
//	  map.map          # the .map file bytes (required)
//	  preview.tga      # optional .tga preview
//	  map.ini          # optional per-map INI overrides
//	  map.str          # optional per-map string table
//	  solo.ini         # optional solo-mode INI
//	  assetusage.txt   # optional asset usage report
//	  readme.txt       # optional readme
//
// Existence is checked by the .map file: if "<MapsDir>/<crc>/map.map"
// exists, we already have that map (sidecars are best-effort and may be
// absent even when the map is present).
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
// game's StatsUploader. The first two are the original kinds; the
// sidecar kinds were added when map distribution moved to lobby time
// and the full FileTransfer set (matching bits 4/8/16/32/64 in
// FileTransfer::DoAnyMapTransfers) had to round-trip the server.
const (
	KindMap     = "map"
	KindPreview = "preview"
	KindINI     = "ini"    // map.ini  (FileTransfer mask bit 4)
	KindStr     = "str"    // map.str  (mask bit 8)
	KindSolo    = "solo"   // solo.ini (mask bit 16)
	KindAssets  = "assets" // assetusage.txt (mask bit 32)
	KindReadme  = "readme" // readme.txt (mask bit 64)
)

// AllKinds lists every supported asset kind in upload/download order.
// Order matches FileTransfer.cpp:264-277 so the wire behavior on both
// sides traverses sidecars the same way.
var AllKinds = []string{
	KindMap, KindPreview, KindINI, KindStr, KindSolo, KindAssets, KindReadme,
}

// On-disk filenames for each asset kind. Fixed names so existence checks
// don't depend on map naming conventions; the original .map name is
// preserved in meta.txt for /get_map.
var kindFilename = map[string]string{
	KindMap:     "map.map",
	KindPreview: "preview.tga",
	KindINI:     "map.ini",
	KindStr:     "map.str",
	KindSolo:    "solo.ini",
	KindAssets:  "assetusage.txt",
	KindReadme:  "readme.txt",
}

const metaFilename = "meta.txt"

// MapDir returns the per-CRC storage directory for a given CRC.
func MapDir(crc string) string {
	return filepath.Join(MapsDir, crc)
}

// Exists reports whether we already have the .map file for this CRC.
// Sidecars are best-effort; missing sidecars do not flip Exists to false.
func Exists(crc string) bool {
	if crc == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(MapDir(crc), kindFilename[KindMap]))
	return err == nil && info.Mode().IsRegular()
}

// IsValidKind reports whether the given X-Map-File value is a kind we
// know how to store.
func IsValidKind(kind string) bool {
	_, ok := kindFilename[kind]
	return ok
}

// Store writes one asset into the CRC dir and refreshes meta.txt with
// the supplied original map name. kind must be a value from AllKinds.
// Calling Store with the same kind twice overwrites silently.
func Store(crc string, mapName string, kind string, data []byte) error {
	if crc == "" {
		return errors.New("mapfile.Store: empty crc")
	}
	target, ok := kindFilename[kind]
	if !ok {
		return fmt.Errorf("mapfile: unknown X-Map-File kind %q", kind)
	}

	dir := MapDir(crc)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create map dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, target), data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	// Refresh meta.txt with the most recent name we've seen for this CRC.
	// In normal operation all uploads for the same map carry the same
	// X-Map-Name, so this is just idempotent overwrite.
	if mapName != "" {
		if err := os.WriteFile(filepath.Join(dir, metaFilename), []byte(mapName), 0644); err != nil {
			return fmt.Errorf("write meta: %w", err)
		}
	}
	return nil
}

// LoadAsset returns the raw bytes of one asset kind for a CRC. Returns
// os.ErrNotExist if the asset isn't stored.
func LoadAsset(crc string, kind string) ([]byte, error) {
	if crc == "" {
		return nil, errors.New("mapfile.LoadAsset: empty crc")
	}
	target, ok := kindFilename[kind]
	if !ok {
		return nil, fmt.Errorf("mapfile: unknown kind %q", kind)
	}
	return os.ReadFile(filepath.Join(MapDir(crc), target))
}

// LoadName returns the original X-Map-Name stored alongside the assets,
// or an empty string if meta.txt is missing.
func LoadName(crc string) string {
	if crc == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(MapDir(crc), metaFilename))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// AvailableKinds returns the set of asset kinds actually present on
// disk for the given CRC, in AllKinds order. Useful for /list_map_assets
// so a peer can request only what's there.
func AvailableKinds(crc string) []string {
	if crc == "" {
		return nil
	}
	dir := MapDir(crc)
	out := make([]string, 0, len(AllKinds))
	for _, k := range AllKinds {
		fn, ok := kindFilename[k]
		if !ok {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, fn)); err == nil && info.Mode().IsRegular() {
			out = append(out, k)
		}
	}
	return out
}

// Load returns the stored map name (from meta.txt), the .map bytes, and
// the .tga preview bytes for the given CRC. previewData is nil when the
// preview wasn't uploaded. Returns os.ErrNotExist if the .map is missing.
//
// Kept for backwards compatibility with the original /get_map zip
// endpoint, which only ever bundled map + preview. New callers should
// prefer LoadAsset for per-kind access.
func Load(crc string) (mapName string, mapData []byte, previewData []byte, err error) {
	if crc == "" {
		return "", nil, nil, errors.New("mapfile.Load: empty crc")
	}

	mapData, err = LoadAsset(crc, KindMap)
	if err != nil {
		return "", nil, nil, err
	}

	mapName = LoadName(crc)

	if previewBytes, previewErr := LoadAsset(crc, KindPreview); previewErr == nil {
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

// ZipEntryName returns the filename to use inside a /get_map zip for
// the given asset kind. The .map and .tga get the BaseName-derived stem
// so they extract into a Maps/<base>/ folder cleanly; sidecars keep
// their canonical names since the game looks for them by literal
// filename next to the .map.
func ZipEntryName(base string, kind string) string {
	switch kind {
	case KindMap:
		return base + ".map"
	case KindPreview:
		return base + ".tga"
	}
	// Sidecars: return the canonical on-disk filename (map.ini etc).
	return kindFilename[kind]
}
