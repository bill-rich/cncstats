package main

import (
	"encoding/json"
	"fmt"
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

func main() {
	objData := os.Getenv("CNC_INI")
	if len(objData) == 0 {
		objData = "/var/Data/INI/Object"
	}
	objectStore, err := iniparse.NewObjectStore(objData)
	if err != nil {
		log.WithError(err).Fatal("could not load object store")
	}
	if len(os.Getenv("TRACE")) > 0 {
		log.SetLevel(log.TraceLevel)
	}

	if len(os.Getenv("LOCAL")) > 0 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			log.WithError(err).Fatal("could not open file")
		}

		bp := &bitparse.BitParser{
			Source:      file,
			ObjectStore: objectStore,
		}
		replay := zhreplay.NewReplay(bp)

		um, err := json.Marshal(replay)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf(string(um))
		return
	}

	router := gin.Default()
	router.POST("/replay", saveFileHandler)
	router.Run()
}

func saveFileHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	// The file cannot be received.
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "No file is received",
		})
		return
	}

	fileIn, err := file.Open()

	objectStore, err := iniparse.NewObjectStore("/var/Data/INI/Object")
	if err != nil {
		log.WithError(err).Fatal("could not load object store")
	}

	bp := &bitparse.BitParser{
		Source:      fileIn,
		ObjectStore: objectStore,
	}

	// File saved successfully. Return proper result
	c.JSON(http.StatusOK, zhreplay.NewReplay(bp))
}
