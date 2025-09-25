package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Parse command line arguments
	var (
		objData    = flag.String("objdata", "", "Path to CNC INI data directory")
		local      = flag.Bool("local", false, "Run in local mode (process single file)")
		trace      = flag.Bool("trace", false, "Enable trace logging")
		help       = flag.Bool("help", false, "Show help information")
		replayFile = flag.String("file", "", "Replay file to process (required in local mode)")
	)
	flag.Parse()

	// Show help if requested
	if *help {
		showHelp()
		return
	}

	// Determine objData path
	objDataPath := getObjDataPath(*objData)

	// Initialize stores
	objectStore, powerStore, upgradeStore, err := initializeStores(objDataPath)
	if err != nil {
		log.WithError(err).Fatal("could not initialize stores")
	}

	// Set log level
	if *trace || len(os.Getenv("TRACE")) > 0 {
		log.SetLevel(log.TraceLevel)
	}

	// Handle local mode
	if *local || len(os.Getenv("LOCAL")) > 0 {
		handleLocalMode(*replayFile, objectStore, powerStore, upgradeStore)
		return
	}

	// Start web server
	startWebServer(objectStore, powerStore, upgradeStore)
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

func initializeStores(objDataPath string) (*iniparse.ObjectStore, *iniparse.PowerStore, *iniparse.UpgradeStore, error) {
	objectStore, err := iniparse.NewObjectStore(objDataPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not load object store: %w", err)
	}

	powerStore, err := iniparse.NewPowerStore(objDataPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not load power store: %w", err)
	}

	upgradeStore, err := iniparse.NewUpgradeStore(objDataPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not load upgrade store: %w", err)
	}

	return objectStore, powerStore, upgradeStore, nil
}

func handleLocalMode(replayFile string, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) {
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
	}

	replay := zhreplay.NewReplay(bp)
	um, err := json.Marshal(replay)
	if err != nil {
		log.WithError(err).Fatal("could not marshal replay data")
	}

	fmt.Printf("%+v\n", string(um))
}

func startWebServer(objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) {
	router := gin.Default()
	router.POST("/replay", func(c *gin.Context) {
		saveFileHandler(c, objectStore, powerStore, upgradeStore)
	})
	port := "8080"
	if len(os.Getenv("PORT")) > 0 {
		port = os.Getenv("PORT")
	}
	router.Run(":" + port)
}

func saveFileHandler(c *gin.Context, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) {
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
	}

	replay := zhreplay.NewReplay(bp)
	c.JSON(http.StatusOK, replay)
}
