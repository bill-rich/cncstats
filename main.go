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
	objectStore, err := iniparse.NewObjectStore("/var/Data/INI/Object")
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

		/*
			totalSpent := map[string]int{}
			for _, order := range replay.Body {
				if order.OrderCode == 1047 {
					object := objectStore.GetObject(order.Args[0].Args[0].(int))
					playerName := replay.Players[order.PlayerID-2]
					totalSpent[playerName] += object.Cost
					fmt.Printf("%d: %s queued up a %s for %d\n",
						order.TimeCode,
						replay.Players[order.PlayerID-2],
						object.Name,
						object.Cost,
					)
				}
			}
			fmt.Printf("Total spent:%+v\n", totalSpent)
		*/

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

	/*

		// The file is received, so let's save it
		if err := c.SaveUploadedFile(file, "/tmp/"+file.Filename); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "Unable to save the file",
				"error":   err,
			})
			return
		}

		fileIn, err := os.Open("/tmp/" + file.Filename)
		if err != nil {
			log.WithError(err).Fatal("could not open file")
		}
	*/
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
