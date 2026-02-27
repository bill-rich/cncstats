package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/database"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay"
	"github.com/bill-rich/cncstats/proto/player_money"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"gorm.io/gorm"
)

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

	// Handle local mode - skip database operations
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

	// Initialize database (only for server mode)
	log.Info("Connecting to database...")
	if err := database.Connect(); err != nil {
		log.WithError(err).Fatal("could not connect to database")
	}
	log.Info("Database connected successfully")
	defer database.Close()

	// Run database migrations
	log.Info("Running database migrations...")
	if err := database.Migrate(); err != nil {
		log.WithError(err).Fatal("could not migrate database")
	}
	log.Info("Database migrations completed")

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

	// Start web server and gRPC server
	log.Info("Starting web server and gRPC server...")
	go startGRPCServer()
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
	// Use command line argument or fall back to os.Args[1] for backward compatibility
	if replayFile == "" && len(os.Args) > 1 {
		replayFile = os.Args[1]
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

func startGRPCServer() {
	grpcPort := "9090"
	if len(os.Getenv("GRPC_PORT")) > 0 {
		grpcPort = os.Getenv("GRPC_PORT")
	}

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.WithError(err).Fatal("Failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 10 * time.Second,
			Time:                  1 * time.Minute,
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	playerMoneyGRPCServer := database.NewPlayerMoneyGRPCServer()
	player_money.RegisterPlayerMoneyServiceServer(grpcServer, playerMoneyGRPCServer)

	log.WithField("port", grpcPort).Info("gRPC server starting")
	if err := grpcServer.Serve(lis); err != nil {
		log.WithError(err).Fatal("Failed to start gRPC server")
	}
}

func startWebServer(objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, colorStore *iniparse.ColorStore) {
	router := gin.Default()

	// Existing replay endpoint
	router.POST("/replay", func(c *gin.Context) {
		saveFileHandler(c, objectStore, powerStore, upgradeStore, colorStore)
	})

	// New player money data endpoints
	playerMoneyService := database.NewPlayerMoneyService()

	// POST /player-money - Create new player money data
	router.POST("/player-money", func(c *gin.Context) {
		createPlayerMoneyHandler(c, playerMoneyService)
	})

	// GET /player-money - Get player money data with optional query parameters
	router.GET("/player-money", func(c *gin.Context) {
		getPlayerMoneyHandler(c, playerMoneyService)
	})

	// GET /player-money/:id - Get specific player money data by ID
	router.GET("/player-money/:id", func(c *gin.Context) {
		getPlayerMoneyByIDHandler(c, playerMoneyService)
	})

	// DELETE /player-money/:id - Delete player money data by ID
	router.DELETE("/player-money/:id", func(c *gin.Context) {
		deletePlayerMoneyHandler(c, playerMoneyService)
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

	// Convert to enhanced replay and add money and stats change events
	enhancedReplay := zhreplay.ConvertToEnhancedReplay(replay)
	enhancedReplay.AddMoneyChangeEvents()
	enhancedReplay.AddStatsChangeEvents()

	// Use money-based winner detection after money events are merged
	enhancedReplay.DetermineWinnersByMoney()

	c.JSON(http.StatusOK, enhancedReplay)
}

// Player money data handlers

func createPlayerMoneyHandler(c *gin.Context, service *database.PlayerMoneyService) {
	var req database.MoneyDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	_, err := service.CreatePlayerMoneyData(&req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create player money data",
			"details": err.Error(),
		})
		return
	}

	c.Status(http.StatusCreated)
}

func getPlayerMoneyHandler(c *gin.Context, service *database.PlayerMoneyService) {
	// Parse query parameters
	limit := 0
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get player money data
	results, err := service.GetAllPlayerMoneyData(limit, offset)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve player money data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  results,
		"count": len(results),
	})
}

func getPlayerMoneyByIDHandler(c *gin.Context, service *database.PlayerMoneyService) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid ID format",
		})
		return
	}

	// Get single record by ID
	var result database.PlayerMoneyData
	if err := database.DB.First(&result, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Player money data not found",
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to retrieve player money data",
				"details": err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func deletePlayerMoneyHandler(c *gin.Context, service *database.PlayerMoneyService) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid ID format",
		})
		return
	}

	if err := service.DeletePlayerMoneyData(uint(id)); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete player money data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Player money data deleted successfully",
	})
}
