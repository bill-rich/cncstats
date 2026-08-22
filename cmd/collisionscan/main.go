// collisionscan looks for game seeds where the stats this server stores and
// the match radarvan stores under the same id describe different matches.
//
// The storage key on both servers is the game seed, which for a LAN game is
// GetTickCount() on the host: milliseconds since that machine booted. That is
// not a unique identifier, and until the upload handler learned to compare
// match identity, a colliding upload was arbitrated purely on file size, so
// the larger payload silently replaced an unrelated match.
//
// The upload guard stops new collisions. This finds ones that already
// happened, by asking a second source what it thinks the same id is. A
// disagreement on map or roster means the two servers are describing
// different games under one key.
//
// Usage:
//
//	STATS_DIR=/app/stats collisionscan -radarvan-key "$KEY"
//
// Reads the stats directory directly, so run it where the volume is mounted.
// Serial requests with a delay by default: radarvan is a single Heroku dyno
// and has fallen over under parallel pulls before.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bill-rich/cncstats/pkg/statsfile"
)

// Shape of the parts of /api/details we compare against. Capitalised JSON
// keys inside player_summary are radarvan's, not a typo.
type radarvanPlayer struct {
	Name string `json:"Name"`
}

type radarvanMatch struct {
	MatchID       int              `json:"matchId"`
	MapName       string           `json:"mapName"`
	PlayerSummary []radarvanPlayer `json:"player_summary"`
}

func main() {
	var (
		baseURL = flag.String("radarvan", "https://www.radarvan.com", "radarvan base URL")
		key     = flag.String("radarvan-key", os.Getenv("RADARVAN_API_KEY"), "radarvan X-API-Key (or RADARVAN_API_KEY)")
		delay   = flag.Duration("delay", 300*time.Millisecond, "pause between radarvan requests")
		limit   = flag.Int("limit", 0, "stop after this many seeds (0 = all)")
		// A LAN seed is the host's uptime in milliseconds, so a low seed means
		// a freshly booted host. That band is where collisions concentrate:
		// GetTickCount ticks about every 15.6ms, so the first hour of uptime
		// holds only ~230k distinct values, and every machine passes through
		// it. Scanning just that band is the cheap, high-yield run.
		maxSeed = flag.Uint64("max-seed", 0, "only check seeds <= this value (0 = no limit); 3600000 is the first hour of host uptime")
	)
	flag.Parse()

	if *key == "" {
		fmt.Fprintln(os.Stderr, "a radarvan API key is required (-radarvan-key or RADARVAN_API_KEY)")
		os.Exit(2)
	}

	seeds, err := storedSeeds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stats dir: %v\n", err)
		os.Exit(1)
	}
	total := len(seeds)
	if *maxSeed > 0 {
		var kept []string
		for _, s := range seeds {
			// Non-numeric keys (the connfail-YYYYMMDD log buckets) are not
			// seeds at all and have no radarvan match to compare against.
			if n, err := strconv.ParseUint(s, 10, 64); err == nil && n <= *maxSeed {
				kept = append(kept, s)
			}
		}
		seeds = kept
	}
	fmt.Printf("stats dir %s: %d stored seeds, %d selected\n", statsfile.StatsDir, total, len(seeds))

	client := &http.Client{Timeout: 30 * time.Second}
	var checked, skipped, mismatched int

	for i, seed := range seeds {
		if *limit > 0 && checked >= *limit {
			break
		}
		if i > 0 {
			time.Sleep(*delay)
		}

		stored, err := statsfile.Load(seed)
		if err != nil {
			skipped++
			continue
		}
		// Normalize our map the same way as radarvan's: the two servers
		// spell the same map differently, and only the leaf name is
		// comparable between them.
		ours := statsfile.IdentityOf(stored)
		ours.Map = mapLeaf(ours.Map)

		theirs, ok, err := fetchMatch(client, *baseURL, *key, seed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "seed %s: radarvan: %v\n", seed, err)
			skipped++
			continue
		}
		if !ok {
			// radarvan has never seen this id. Nothing to compare against,
			// which is not evidence either way.
			skipped++
			continue
		}

		checked++
		if !ours.SameMatch(theirs) {
			mismatched++
			fmt.Printf("MISMATCH seed=%s\n  cncstats: %s\n  radarvan: %s\n", seed, ours, theirs)
		}
	}

	fmt.Printf("\nchecked %d, skipped %d (unparseable or unknown to radarvan), mismatched %d\n",
		checked, skipped, mismatched)
	if mismatched > 0 {
		os.Exit(1)
	}
}

// storedSeeds lists every seed with a stats file, sorted so runs are
// reproducible and progress is easy to eyeball.
func storedSeeds() ([]string, error) {
	entries, err := os.ReadDir(statsfile.StatsDir)
	if err != nil {
		return nil, err
	}
	var seeds []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json.gz") {
			continue
		}
		seeds = append(seeds, strings.TrimSuffix(name, ".json.gz"))
	}
	sort.Strings(seeds)
	return seeds, nil
}

// fetchMatch asks radarvan for one match. ok is false when radarvan has no
// such id, which is different from an error talking to it.
func fetchMatch(client *http.Client, baseURL, key, seed string) (statsfile.Identity, bool, error) {
	url := fmt.Sprintf("%s/api/details/%s", strings.TrimRight(baseURL, "/"), seed)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return statsfile.Identity{}, false, err
	}
	req.Header.Set("X-API-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		return statsfile.Identity{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return statsfile.Identity{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return statsfile.Identity{}, false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var m radarvanMatch
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return statsfile.Identity{}, false, fmt.Errorf("decode: %w", err)
	}

	id := statsfile.Identity{Map: mapLeaf(m.MapName)}
	for _, p := range m.PlayerSummary {
		id.Players = append(id.Players, p.Name)
	}
	sort.Strings(id.Players)
	return id, true, nil
}

// mapLeaf reduces a map path to its final component. The two servers spell
// the same map differently ("maps/alpine assault" against
// "userdata/maps/Alpine Assault/Alpine Assault.map"), and only the leaf is
// comparable. Lowercased because the case differs too.
func mapLeaf(path string) string {
	p := strings.ReplaceAll(path, "\\", "/")
	p = strings.TrimSuffix(p, "/")
	leaf := filepath.Base(p)
	leaf = strings.TrimSuffix(leaf, filepath.Ext(leaf))
	return strings.ToLower(leaf)
}
