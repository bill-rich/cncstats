package main

import (
	"archive/zip"
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/bill-rich/cncstats/docs"
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/mapfile"
	"github.com/bill-rich/cncstats/pkg/statsfile"
	"github.com/bill-rich/cncstats/pkg/zhreplay"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// StatsUploadResponse represents the response for a successful stats upload.
type StatsUploadResponse struct {
	Message string `json:"message" example:"Stats stored successfully"`
	Seed    string `json:"seed" example:"12345"`
	Size    int    `json:"size" example:"8192"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error" example:"Something went wrong"`
	Details string `json:"details,omitempty" example:"underlying error message"`
}

// @title CNC Stats API
// @version 2.0
// @description Replay parser and stats API for Command & Conquer Generals / Zero Hour
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description API key supplied as "Bearer <key>". Required on write endpoints when CNC_AUTH_REQUIRED is true.
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key supplied in the X-API-Key header. Alternative to the Authorization header.
func main() {
	// Parse command line arguments
	var (
		objData    = flag.String("objdata", "", "Path to CNC INI data directory")
		local      = flag.Bool("local", false, "Run in local mode (process single file)")
		trace      = flag.Bool("trace", false, "Enable trace logging")
		help       = flag.Bool("help", false, "Show help information")
		replayFile = flag.String("file", "", "Replay file to process (required in local mode)")
		noStores   = flag.Bool("no-stores", false, "Run without INI stores (fields will be blank)")
	)
	flag.Parse()

	// Show help if requested
	if *help {
		showHelp()
		return
	}

	// Determine objData path
	objDataPath := getObjDataPath(*objData)

	// Configure logrus for Heroku (output to stderr, JSON format in production)
	// Heroku captures stderr automatically
	log.SetOutput(os.Stderr)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		DisableColors: os.Getenv("DYNO") != "", // Disable colors on Heroku
	})

	// Set log level
	if *trace || len(os.Getenv("TRACE")) > 0 {
		log.SetLevel(log.TraceLevel)
	} else {
		log.SetLevel(log.InfoLevel) // Ensure InfoLevel is set explicitly
	}

	log.Info("CNC Stats application starting...")

	// Handle local mode
	if *local || len(os.Getenv("LOCAL")) > 0 {
		var objectStore *iniparse.ObjectStore
		var powerStore *iniparse.PowerStore
		var upgradeStore *iniparse.UpgradeStore
		var err error

		// Initialize stores for local mode unless no-stores flag is set
		var colorStore *iniparse.ColorStore
		if !*noStores {
			objectStore, powerStore, upgradeStore, colorStore, err = initializeStores(objDataPath)
			if err != nil {
				log.WithError(err).Fatal("could not initialize stores")
			}
		}

		handleLocalMode(*replayFile, objectStore, powerStore, upgradeStore, colorStore)
		return
	}

	// Initialize stores for server mode unless no-stores flag is set
	var objectStore *iniparse.ObjectStore
	var powerStore *iniparse.PowerStore
	var upgradeStore *iniparse.UpgradeStore
	var colorStore *iniparse.ColorStore

	if !*noStores {
		log.Info("Initializing INI stores...")
		var err error
		objectStore, powerStore, upgradeStore, colorStore, err = initializeStores(objDataPath)
		if err != nil {
			log.WithError(err).Fatal("could not initialize stores")
		}
		log.Info("INI stores initialized successfully")
	} else {
		log.Info("Running without INI stores")
	}

	// Start web server
	log.Info("Starting web server...")
	startWebServer(objectStore, powerStore, upgradeStore, colorStore)
}

// Helper functions

func showHelp() {
	fmt.Println("CNC Stats - Command and Conquer Replay Analyzer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cncstats [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -objdata string")
	fmt.Println("        Path to CNC INI data directory (default: /var/Data/INI or ./inizh/Data/INI in local mode)")
	fmt.Println("  -local")
	fmt.Println("        Run in local mode (process single file)")
	fmt.Println("  -file string")
	fmt.Println("        Replay file to process (required in local mode)")
	fmt.Println("  -trace")
	fmt.Println("        Enable trace logging")
	fmt.Println("  -no-stores")
	fmt.Println("        Run without INI stores (fields will be blank)")
	fmt.Println("  -help")
	fmt.Println("        Show this help information")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  CNC_INI    Path to CNC INI data directory")
	fmt.Println("  LOCAL      Set to any value to enable local mode")
	fmt.Println("  TRACE      Set to any value to enable trace logging")
}

func getObjDataPath(cliObjData string) string {
	// Command line argument takes precedence
	if cliObjData != "" {
		return cliObjData
	}

	// Check environment variable
	if envObjData := os.Getenv("CNC_INI"); envObjData != "" {
		return envObjData
	}

	// Default based on mode
	if len(os.Getenv("LOCAL")) > 0 {
		return "./inizh/Data/INI"
	}

	return "/var/Data/INI"
}

func initializeStores(objDataPath string) (*iniparse.ObjectStore, *iniparse.PowerStore, *iniparse.UpgradeStore, *iniparse.ColorStore, error) {
	objectStore, err := iniparse.NewObjectStore(objDataPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not load object store: %w", err)
	}

	powerStore, err := iniparse.NewPowerStore(objDataPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not load power store: %w", err)
	}

	upgradeStore, err := iniparse.NewUpgradeStore(objDataPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not load upgrade store: %w", err)
	}

	colorStore, err := iniparse.NewColorStore(objDataPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not load color store: %w", err)
	}

	return objectStore, powerStore, upgradeStore, colorStore, nil
}

func handleLocalMode(replayFile string, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, colorStore *iniparse.ColorStore) {
	// Use command line argument or fall back to first non-flag argument
	if replayFile == "" && flag.NArg() > 0 {
		replayFile = flag.Arg(0)
	}

	if replayFile == "" {
		log.Fatal("replay file is required in local mode. Use -file flag or provide as first argument")
	}

	file, err := os.Open(replayFile)
	if err != nil {
		log.WithError(err).Fatal("could not open file")
	}
	defer file.Close()

	bp := &bitparse.BitParser{
		Source:       file,
		ObjectStore:  objectStore,
		PowerStore:   powerStore,
		UpgradeStore: upgradeStore,
		ColorStore:   colorStore,
	}

	replay := zhreplay.NewReplay(bp)
	v2 := zhreplay.ConvertToBasicEnhancedReplayV2(replay)
	um, err := json.Marshal(v2)
	if err != nil {
		log.WithError(err).Fatal("could not marshal replay data")
	}

	fmt.Printf("%+v\n", string(um))
}

// apiKeyStore maps a valid API key to the name of the client it belongs to.
// Names are used for logging only; keys are the secret.
type apiKeyStore map[string]string

// loadAPIKeys parses CNC_API_KEYS, a comma-separated list of "name:key" pairs
// (e.g. "zulu:abc123,radarvan:def456,dev:ghi789"). Add, replace, or remove
// clients by editing the env var; no code change is needed. Malformed entries
// are logged and skipped.
func loadAPIKeys(raw string) apiKeyStore {
	store := apiKeyStore{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, key, ok := strings.Cut(pair, ":")
		name = strings.TrimSpace(name)
		key = strings.TrimSpace(key)
		if !ok || name == "" || key == "" {
			log.WithField("entry", pair).Warn("ignoring malformed CNC_API_KEYS entry; expected name:key")
			continue
		}
		store[key] = name
	}
	return store
}

// match returns the client name for a provided key, or "" if none match. All
// entries are checked with a constant-time compare and without an early exit
// so response timing doesn't reveal which (or whether a) key matched.
func (s apiKeyStore) match(provided string) string {
	got := []byte(provided)
	matched := ""
	for key, name := range s {
		if subtle.ConstantTimeCompare(got, []byte(key)) == 1 {
			matched = name
		}
	}
	return matched
}

// names returns the configured client names (no keys), for startup logging.
func (s apiKeyStore) names() []string {
	out := make([]string, 0, len(s))
	for _, name := range s {
		out = append(out, name)
	}
	return out
}

// authIsRequired reports whether write endpoints should enforce API keys. It
// reads CNC_AUTH_REQUIRED and defaults to true (secure by default) when unset
// or unparseable.
func authIsRequired() bool {
	v := os.Getenv("CNC_AUTH_REQUIRED")
	if v == "" {
		return true
	}
	required, err := strconv.ParseBool(v)
	if err != nil {
		log.WithField("value", v).Warn("could not parse CNC_AUTH_REQUIRED; defaulting to true")
		return true
	}
	return required
}

// apiKeyAuth returns Gin middleware that requires a valid API key on the
// request. The key may be supplied as "Authorization: Bearer <key>" or in the
// "X-API-Key" header. On success the matched client name is stored on the
// context under "client".
func apiKeyAuth(keys apiKeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := keys.match(extractAPIKey(c))
		if name == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Error: "invalid or missing API key",
			})
			return
		}
		c.Set("client", name)
		c.Next()
	}
}

// apiKeyIdentify returns middleware that matches a provided API key and sets
// the client name on the context, but never aborts. Used in dry-run mode
// (CNC_AUTH_REQUIRED=false with CNC_API_KEYS set) to verify clients are
// sending keys correctly before enforcing auth.
func apiKeyIdentify(keys apiKeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if name := keys.match(extractAPIKey(c)); name != "" {
			c.Set("client", name)
		}
		c.Next()
	}
}

// clientName returns the authenticated client name set by apiKeyAuth, or
// "anonymous" when auth is disabled and no client was matched.
func clientName(c *gin.Context) string {
	if name := c.GetString("client"); name != "" {
		return name
	}
	return "anonymous"
}

// extractAPIKey pulls the API key from the Authorization or X-API-Key header.
func extractAPIKey(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return after
		}
		return h
	}
	return c.GetHeader("X-API-Key")
}

func startWebServer(objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, colorStore *iniparse.ColorStore) {
	router := gin.Default()

	// Write endpoints are grouped behind a shared API key. Read endpoints
	// (map downloads, docs) stay open because game peers need them mid-lobby
	// and they're not destructive.
	writes := router.Group("/")
	if authIsRequired() {
		keys := loadAPIKeys(os.Getenv("CNC_API_KEYS"))
		if len(keys) == 0 {
			log.Fatal("CNC_AUTH_REQUIRED is true but no valid keys found in CNC_API_KEYS")
		}
		writes.Use(apiKeyAuth(keys))
		log.WithField("clients", keys.names()).Info("API key auth enabled for write endpoints")
	} else {
		keys := loadAPIKeys(os.Getenv("CNC_API_KEYS"))
		if len(keys) > 0 {
			writes.Use(apiKeyIdentify(keys))
			log.WithField("clients", keys.names()).Warn("authentication disabled (CNC_AUTH_REQUIRED=false); identifying clients for logging only. Write endpoints (/replay, /stats, /add_map) are UNAUTHENTICATED")
		} else {
			log.Warn("authentication disabled (CNC_AUTH_REQUIRED=false); write endpoints (/replay, /stats, /add_map) are UNAUTHENTICATED")
		}
	}

	// Replay endpoint
	writes.POST("/replay", func(c *gin.Context) {
		saveFileHandler(c, objectStore, powerStore, upgradeStore, colorStore)
	})

	// Stats upload endpoint - receives gzip-compressed JSON stats from Generals
	writes.POST("/stats", func(c *gin.Context) {
		uploadStatsHandler(c)
	})

	// Map endpoints
	router.GET("/map_exists", mapExistsHandler)
	writes.POST("/add_map", addMapHandler)
	router.GET("/get_map", getMapHandler)
	router.GET("/get_map_file", getMapFileHandler)
	router.GET("/list_map_assets", listMapAssetsHandler)

	// Swagger UI (serves Swagger 2.0 interactive docs)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// OpenAPI v3 spec (JSON and YAML)
	router.GET("/openapi3.json", func(c *gin.Context) {
		c.File("docs/openapi3.json")
	})
	router.GET("/openapi3.yaml", func(c *gin.Context) {
		c.File("docs/openapi3.yaml")
	})

	port := "8080"
	if len(os.Getenv("PORT")) > 0 {
		port = os.Getenv("PORT")
	}

	log.WithField("port", port).Info("Server starting")
	if err := router.Run(":" + port); err != nil {
		log.WithError(err).Fatal("Failed to start server")
	}
}

// saveFileHandler parses an uploaded replay file.
// @Summary Parse a replay file
// @Description Upload a .rep replay file and receive parsed replay data in v2 format. Stats fields are populated when a matching stats file exists.
// @Tags replay
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Replay file to parse"
// @Success 200 {object} zhreplay.EnhancedReplayV2
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security ApiKeyAuth
// @Router /replay [post]
func saveFileHandler(c *gin.Context, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, colorStore *iniparse.ColorStore) {
	file, err := c.FormFile("file")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "No file is received",
		})
		return
	}

	fileIn, err := file.Open()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "Could not open uploaded file",
		})
		return
	}
	defer fileIn.Close()

	bp := &bitparse.BitParser{
		Source:       fileIn,
		ObjectStore:  objectStore,
		PowerStore:   powerStore,
		UpgradeStore: upgradeStore,
		ColorStore:   colorStore,
	}

	replay := zhreplay.NewReplay(bp)

	// If a stats file exists for this seed, return enhanced v2 replay
	seed := replay.Header.Metadata.Seed

	log.WithFields(log.Fields{
		"seed":   seed,
		"map":    replay.Header.Metadata.MapPath,
		"client": clientName(c),
	}).Info("Replay parsed")
	if seed != "" && statsfile.Exists(seed) {
		stats, err := statsfile.Load(seed)
		if err != nil {
			log.WithError(err).Warn("Failed to load stats file, returning replay-only v2")
			c.JSON(http.StatusOK, zhreplay.ConvertToBasicEnhancedReplayV2(replay))
			return
		} else {
			v2Replay := zhreplay.ConvertToEnhancedReplayV2(replay, stats, objectStore)
			c.JSON(http.StatusOK, v2Replay)
			return
		}
	}

	// No stats file, return v2 with replay data only
	c.JSON(http.StatusOK, zhreplay.ConvertToBasicEnhancedReplayV2(replay))
}

// uploadStatsHandler stores a gzip-compressed stats payload.
// @Summary Upload game stats
// @Description Receive gzip-compressed JSON stats from a Generals game and store them keyed by seed.
// @Tags stats
// @Accept octet-stream
// @Produce json
// @Param X-Game-Seed header string true "Game seed identifier"
// @Success 200 {object} StatsUploadResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security ApiKeyAuth
// @Router /stats [post]
func uploadStatsHandler(c *gin.Context) {
	seed := c.GetHeader("X-Game-Seed")
	if seed == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "X-Game-Seed header is required",
		})
		return
	}

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read request body",
			"details": err.Error(),
		})
		return
	}

	if len(data) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Empty request body",
		})
		return
	}

	if err := statsfile.Store(seed, data); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to store stats file",
			"details": err.Error(),
		})
		return
	}

	log.WithField("seed", seed).WithField("size", len(data)).WithField("client", clientName(c)).Info("Stats file stored")
	c.JSON(http.StatusOK, gin.H{
		"message": "Stats stored successfully",
		"seed":    seed,
		"size":    len(data),
	})
}

// mapExistsHandler reports whether the server already has a map for the
// given CRC.
// @Summary Check whether a map exists on the server
// @Description Returns plain-text "true" if a .map file is already stored under MAPS_DIR for the given crc, "false" otherwise. Used by the Generals client right after a game ends to decide whether to upload its played map.
// @Tags maps
// @Produce plain
// @Param crc query string true "Map CRC (decimal, as emitted by MapMetaData::m_CRC)"
// @Success 200 {string} string "\"true\" or \"false\""
// @Failure 400 {object} ErrorResponse
// @Router /map_exists [get]
func mapExistsHandler(c *gin.Context) {
	crc := c.Query("crc")
	if crc == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "crc query parameter is required",
		})
		return
	}

	answer := "false"
	if mapfile.Exists(crc) {
		answer = "true"
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, answer)
}

// addMapHandler ingests one map asset and writes it into MAPS_DIR/<crc>/.
// The Generals client makes one POST per asset (X-Map-File: map, preview,
// ini, str, solo, assets, readme), all sharing the same X-Map-CRC.
// @Summary Upload a map asset (.map / .tga / sidecar)
// @Description Stores one asset that makes up a map, keyed by X-Map-CRC. Supported X-Map-File values: "map", "preview", "ini", "str", "solo", "assets", "readme". Identical CRCs overwrite silently.
// @Tags maps
// @Accept octet-stream
// @Produce json
// @Param X-Map-CRC header string true "Map CRC (decimal); identifies the map"
// @Param X-Map-Name header string false "Original map path/name from the game (e.g. \"Maps\\Tournament Desert\\Tournament Desert.map\")"
// @Param X-Map-File header string true "Asset kind: \"map\", \"preview\", \"ini\", \"str\", \"solo\", \"assets\", \"readme\""
// @Param X-Game-Seed header string false "Game seed for telemetry correlation"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Security ApiKeyAuth
// @Router /add_map [post]
func addMapHandler(c *gin.Context) {
	crc := c.GetHeader("X-Map-CRC")
	if crc == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "X-Map-CRC header is required",
		})
		return
	}
	kind := c.GetHeader("X-Map-File")
	if !mapfile.IsValidKind(kind) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":   fmt.Sprintf("X-Map-File header must be one of %v", mapfile.AllKinds),
			"details": fmt.Sprintf("got %q", kind),
		})
		return
	}
	mapName := c.GetHeader("X-Map-Name")

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read request body",
			"details": err.Error(),
		})
		return
	}
	if len(data) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Empty request body",
		})
		return
	}

	if err := mapfile.Store(crc, mapName, kind, data); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to store map asset",
			"details": err.Error(),
		})
		return
	}

	log.WithFields(log.Fields{
		"crc":    crc,
		"kind":   kind,
		"size":   len(data),
		"name":   mapName,
		"client": clientName(c),
	}).Info("Map asset stored")
	c.JSON(http.StatusOK, gin.H{
		"message": "Map asset stored successfully",
		"crc":     crc,
		"kind":    kind,
		"size":    len(data),
	})
}

// getMapHandler returns a zip archive containing the .map file and (if
// stored) the .tga preview for the given CRC. Entries inside the zip are
// renamed to use the basename from the original X-Map-Name so the zip
// can be extracted directly into a Maps/<dir>/ folder.
// @Summary Download the map and preview as a zip
// @Description Returns a zip archive whose entries are the .map file and (if available) the .tga preview, named after the original map basename. Suitable for direct extraction into a Generals Maps/ subdirectory.
// @Tags maps
// @Produce application/zip
// @Param crc query string true "Map CRC (decimal)"
// @Success 200 {file} file "Zip archive (application/zip)"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /get_map [get]
func getMapHandler(c *gin.Context) {
	crc := c.Query("crc")
	if crc == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "crc query parameter is required",
		})
		return
	}

	if !mapfile.Exists(crc) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "no map stored for that crc",
			"crc":   crc,
		})
		return
	}

	mapName := mapfile.LoadName(crc)
	base := mapfile.BaseName(mapName)

	// Build the zip in memory; map plus sidecars top out around a MB.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	var zipErr error
	for _, kind := range mapfile.AvailableKinds(crc) {
		data, err := mapfile.LoadAsset(crc, kind)
		if err != nil {
			// AvailableKinds said it's there; if LoadAsset disagrees,
			// race with concurrent deletion. Skip rather than abort.
			continue
		}
		entry, err := zw.Create(mapfile.ZipEntryName(base, kind))
		if err != nil {
			zipErr = err
			break
		}
		if _, err := entry.Write(data); err != nil {
			zipErr = err
			break
		}
	}
	if zipErr == nil {
		zipErr = zw.Close()
	}
	if zipErr != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to build zip archive",
			"details": zipErr.Error(),
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", base+".zip"))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// getMapFileHandler returns the raw bytes of one stored asset. Used by
// game peers who download maps mid-lobby and don't want to ship a zip
// parser in the client.
// @Summary Download a single map asset
// @Description Returns raw bytes for one stored asset kind. 404 if either the CRC isn't known or that kind wasn't uploaded.
// @Tags maps
// @Produce octet-stream
// @Param crc query string true "Map CRC (decimal)"
// @Param kind query string true "Asset kind: \"map\", \"preview\", \"ini\", \"str\", \"solo\", \"assets\", \"readme\""
// @Success 200 {file} file "Raw asset bytes (application/octet-stream)"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /get_map_file [get]
func getMapFileHandler(c *gin.Context) {
	crc := c.Query("crc")
	if crc == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "crc query parameter is required",
		})
		return
	}
	kind := c.Query("kind")
	if !mapfile.IsValidKind(kind) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":   fmt.Sprintf("kind query parameter must be one of %v", mapfile.AllKinds),
			"details": fmt.Sprintf("got %q", kind),
		})
		return
	}

	data, err := mapfile.LoadAsset(crc, kind)
	if err != nil {
		if os.IsNotExist(err) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "asset not stored",
				"crc":   crc,
				"kind":  kind,
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to load asset",
			"details": err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", data)
}

// listMapAssetsHandler returns the set of asset kinds present on disk
// for a given CRC, plus the stored map name. Lets a peer fetch only
// what's actually available without probing every kind with a HEAD/GET.
// @Summary List stored asset kinds for a CRC
// @Description Returns JSON: {"crc": "...", "name": "...", "kinds": ["map","preview","ini",...]}. Empty kinds array if the CRC is unknown.
// @Tags maps
// @Produce json
// @Param crc query string true "Map CRC (decimal)"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Router /list_map_assets [get]
func listMapAssetsHandler(c *gin.Context) {
	crc := c.Query("crc")
	if crc == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "crc query parameter is required",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"crc":   crc,
		"name":  mapfile.LoadName(crc),
		"kinds": mapfile.AvailableKinds(crc),
	})
}
