package main

import (
	"encoding/json"
	"fmt"
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strconv"
)

type Replay struct {
	Header  *header.GeneralsHeader
	Body    []*body.BodyChunk
	Players []string
}

func main() {
	objectStore := iniparse.NewObjectStore()
	objectStore.LoadObjects("/home/hrich/Downloads/inizh/Data/INI/Object")

	if len(os.Getenv("TRACE")) > 0 {
		log.SetLevel(log.TraceLevel)
	}

	if len(os.Getenv("SERVER")) == 0 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			log.WithError(err).Fatal("could not open file")
		}

		bp := &bitparse.BitParser{
			Source:      file,
			ObjectStore: objectStore,
		}
		replay := ReadReplay(bp)

		totalSpent := map[string]int{}
		for _, order := range replay.Body {
			if order.OrderType == 1047 {
				object := objectStore.GetObject(order.Args[0].Args[0].(int))
				playerName := replay.Players[order.Number-2]
				totalSpent[playerName] += object.Cost
				fmt.Printf("%d: %s queued up a %s for %d\n",
					order.TimeCode,
					replay.Players[order.Number-2],
					object.Name,
					object.Cost,
				)
			}
		}
		fmt.Printf("Total spent:%+v\n", totalSpent)

		um, err := json.Marshal(replay)
		if err != nil {
			panic(err)
		}
		fmt.Sprintf(string(um))
		return
	}

	router := gin.Default()
	router.POST("/replay", saveFileHandler)
	router.Run()
}

func ReadReplay(bp *bitparse.BitParser) Replay {
	replay := Replay{
		header.ParseHeader(bp),
		body.ParseBody(bp),
		[]string{},
	}
	replay.PlayerMap()
	return replay
}

func (r *Replay) PlayerMap() {
	for _, player := range r.Header.Metadata.Players {
		r.Players = append(r.Players, player.Name)
	}
	return
	for i := 0; i <= 7; i++ {
		for _, player := range r.Header.Metadata.Players {
			if player.Team == strconv.Itoa(i) {
				r.Players = append(r.Players, player.Name)
			}
		}
	}
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

	bp := &bitparse.BitParser{
		Source: fileIn,
	}

	// File saved successfully. Return proper result
	c.JSON(http.StatusOK, ReadReplay(bp))
}
