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
	Header     *header.GeneralsHeader
	Body       []*body.BodyChunk
	Players    []string
	PlayerInfo []PlayerInfo
}

func main() {
	objectStore := iniparse.NewObjectStore()
	err := objectStore.LoadObjects("/var/Data/INI/Object")
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
		replay := ReadReplay(bp)

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
			panic(err)
		}
		fmt.Printf(string(um))
		return
	}

	router := gin.Default()
	router.POST("/replay", saveFileHandler)
	router.Run()
}

func ReadReplay(bp *bitparse.BitParser) Replay {
	replay := Replay{}
	replay.Header = header.ParseHeader(bp)
	replay.PlayerMap()
	replay.Body = body.ParseBody(bp, replay.Players, bp.ObjectStore)
	replay.GenerateData()
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

var ConstructorMap = map[string]string{
	"GLAInfantryWorker":        "GLA",
	"Slth_GLAInfantryWorker":   "GLA Stealth",
	"Chem_GLAInfantryWorker":   "GLA Toxin",
	"Demo_GLAInfantryWorker":   "GLA Demo",
	"AmericaVehicleDozer":      "USA",
	"AirF_AmericaVehicleDozer": "USA Airforce",
	"Lazr_AmericaVehicleDozer": "USA Lazr",
	"SupW_AmericaVehicleDozer": "USA Superweapon",
	"ChinaVehicleDozer":        "China",
	"Infa_ChinaVehicleDozer":   "China Infantry",
	"Nuke_ChinaVehicleDozer":   "China Nuke",
	"Tank_ChinaVehicleDozer":   "China Tank",
}

type Unit struct {
	Name string
	Cost int
}

type Building struct {
	Name string
	Cost int
}

type PlayerInfo struct {
	Name           string
	Side           string
	MoneySpent     int
	UnitsCreated   []Unit
	BuildingsBuilt []Building
	Team           int
	Win            bool
}

func (r *Replay) GenerateData() {
	for _, playerMd := range r.Header.Metadata.Players {
		team, _ := strconv.Atoi(playerMd.Team)
		player := PlayerInfo{
			Name: playerMd.Name,
			Team: team + 1,
			Win:  true,
		}
		for _, order := range r.Body {
			if order.PlayerName != player.Name {
				continue
			}
			if order.OrderCode == 1047 {
				if details, ok := order.Details.(map[string]interface{}); ok {
					if side, ok := ConstructorMap[details["object"].(string)]; ok {
						if player.Side == "" {
							player.Side = side
						}
					}
					unit := Unit{
						Name: details["object"].(string),
						Cost: details["cost"].(int),
					}
					player.UnitsCreated = append(player.UnitsCreated, unit)
					player.MoneySpent += unit.Cost
				}
			}
			if order.OrderCode == 1049 {
				if details, ok := order.Details.(map[string]interface{}); ok {
					building := Building{
						Name: details["object"].(string),
						Cost: details["cost"].(int),
					}
					player.BuildingsBuilt = append(player.BuildingsBuilt, building)
					player.MoneySpent += building.Cost
				}
			}
			if order.OrderCode == 1093 {
				player.Win = false
			}
		}
		r.PlayerInfo = append(r.PlayerInfo, player)
	}

	// Hacky way to check results. Both players losing by selling or getting fully destroyed will break detection.
	teamWins := map[int]bool{}
	for _, p := range r.PlayerInfo {
		teamWins[p.Team] = true
	}
	for p, _ := range r.PlayerInfo {
		if !r.PlayerInfo[p].Win {
			teamWins[r.PlayerInfo[p].Team] = false
		}
	}
	for p, _ := range r.PlayerInfo {
		if !teamWins[r.PlayerInfo[p].Team] {
			r.PlayerInfo[p].Win = false
		}
	}
	winners := 0
	for _, t := range teamWins {
		if t {
			winners++
		}
	}

	if winners > 1 {
		// Uh oh. Hack it up real bad
		for k, _ := range teamWins {
			teamWins[k] = false
		}
		for p, _ := range r.PlayerInfo {
			r.PlayerInfo[p].Win = false
		}
		for i := len(r.Body) - 1; i >= 0; i-- {
			chunk := r.Body[i]
			if chunk.OrderCode != 1095 && chunk.OrderCode != 1003 && chunk.OrderCode != 1092 && chunk.OrderCode != 27 && chunk.OrderCode != 1052 {
				team := 0
				for _, p := range r.PlayerInfo {
					if p.Name == chunk.PlayerName {
						team = p.Team
					}
				}
				if team != 0 {
					teamWins[team] = true
					break
				}
			}
		}
		for p, _ := range r.PlayerInfo {
			if teamWins[r.PlayerInfo[p].Team] {
				r.PlayerInfo[p].Win = true
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

	objectStore := iniparse.NewObjectStore()
	err = objectStore.LoadObjects("/var/Data/INI/Object")
	if err != nil {
		log.WithError(err).Fatal("could not load object store")
	}

	bp := &bitparse.BitParser{
		Source:      fileIn,
		ObjectStore: objectStore,
	}

	// File saved successfully. Return proper result
	c.JSON(http.StatusOK, ReadReplay(bp))
}
