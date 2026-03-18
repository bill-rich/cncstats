package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	_ "github.com/bill-rich/cncstats/docs"
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
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
	um, err := json.Marshal(replay)
	if err != nil {
		log.WithError(err).Fatal("could not marshal replay data")
	}

	fmt.Printf("%+v\n", string(um))
}

func startWebServer(objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, colorStore *iniparse.ColorStore) {
	router := gin.Default()

	// Replay endpoint
	router.POST("/replay", func(c *gin.Context) {
		saveFileHandler(c, objectStore, powerStore, upgradeStore, colorStore)
	})

	// Stats upload endpoint - receives gzip-compressed JSON stats from Generals
	router.POST("/stats", func(c *gin.Context) {
		uploadStatsHandler(c)
	})

	// Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
// @Description Upload a .rep replay file and receive parsed replay data. Returns enhanced v2 stats if a matching stats file exists, otherwise returns the basic parsed replay.
// @Tags replay
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Replay file to parse"
// @Success 200 {object} zhreplay.EnhancedReplayV2
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
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
	if seed != "" && statsfile.Exists(seed) {
		stats, err := statsfile.Load(seed)
		if err != nil {
			log.WithError(err).Warn("Failed to load stats file, returning basic replay")
		} else {
			v2Replay := zhreplay.ConvertToEnhancedReplayV2(replay, stats, objectStore)
			c.JSON(http.StatusOK, v2Replay)
			return
		}
	}

	// No stats file — return basic parsed replay
	c.JSON(http.StatusOK, replay)
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
// @Failure 500 {object} ErrorResponse
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

	log.WithField("seed", seed).WithField("size", len(data)).Info("Stats file stored")
	c.JSON(http.StatusOK, gin.H{
		"message": "Stats stored successfully",
		"seed":    seed,
		"size":    len(data),
	})
}
