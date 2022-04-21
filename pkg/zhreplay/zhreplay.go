package zhreplay

import (
	"fmt"
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
	"strconv"
)

type Replay struct {
	Header  *header.GeneralsHeader
	Body    []*body.BodyChunk
	Summary []*object.PlayerSummary
	Offset  int
}

func NewReplay(bp *bitparse.BitParser) *Replay {
	replay := &Replay{
		Offset: 2,
	}
	replay.Header = header.NewHeader(bp)
	replay.CreatePlayerList()
	replay.Body = body.ParseBody(bp, replay.Summary, bp.ObjectStore)
	replay.AdjustOffset()
	replay.AddUserNames()
	replay.GenerateData()
	return replay
}

func (r *Replay) AddUserNames() {
	for _, chunk := range r.Body {
		if chunk.PlayerID >= r.Offset && chunk.PlayerID-r.Offset < len(r.Summary) {
			fmt.Printf("%+v\n", chunk.PlayerID)
			chunk.PlayerName = r.Summary[chunk.PlayerID-r.Offset].Name
		}
	}
}

func (r *Replay) AdjustOffset() {
	lowest := 1000
	for _, chunk := range r.Body {
		if chunk.PlayerID < lowest {
			lowest = chunk.PlayerID
		}
	}
	r.Offset = lowest
}

func (r *Replay) CreatePlayerList() {
	for _, playerMd := range r.Header.Metadata.Players {
		team, _ := strconv.Atoi(playerMd.Team)
		player := &object.PlayerSummary{
			Name:           playerMd.Name,
			Team:           team + 1,
			Win:            true,
			BuildingsBuilt: map[string]*object.ObjectSummary{},
			UnitsCreated:   map[string]*object.ObjectSummary{},
		}
		r.Summary = append(r.Summary, player)
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

func (r *Replay) GenerateData() {
	for _, player := range r.Summary {
		for _, order := range r.Body {
			if order.PlayerName != player.Name {
				continue
			}
			if order.OrderCode == 1047 {
				if side, ok := ConstructorMap[order.Details.GetName()]; ok {
					if player.Side == "" {
						player.Side = side
					}
				}
				unit := object.Unit{
					Name: order.Details.GetName(),
					Cost: order.Details.GetCost(),
				}
				summary, ok := player.UnitsCreated[order.Details.GetName()]
				if !ok {
					summary = &object.ObjectSummary{}
					player.UnitsCreated[order.Details.GetName()] = summary
				}
				summary.Count++
				summary.TotalSpent += unit.Cost
				player.MoneySpent += unit.Cost
			}
			if order.OrderCode == 1049 {
				building := object.Building{
					Name: order.Details.GetName(),
					Cost: order.Details.GetCost(),
				}
				summary, ok := player.BuildingsBuilt[order.Details.GetName()]
				if !ok {
					summary = &object.ObjectSummary{}
					player.BuildingsBuilt[order.Details.GetName()] = summary
				}
				summary.Count++
				summary.TotalSpent += building.Cost
				player.MoneySpent += building.Cost
			}
			if order.OrderCode == 1093 {
				player.Win = false
			}
		}
	}

	// Hacky way to check results. Both players losing by selling or getting fully destroyed will break detection.
	teamWins := map[int]bool{}
	for _, p := range r.Summary {
		teamWins[p.Team] = true
	}
	for p, _ := range r.Summary {
		if !r.Summary[p].Win {
			teamWins[r.Summary[p].Team] = false
		}
	}
	for p, _ := range r.Summary {
		if !teamWins[r.Summary[p].Team] {
			r.Summary[p].Win = false
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
		for p, _ := range r.Summary {
			r.Summary[p].Win = false
		}
		for i := len(r.Body) - 1; i >= 0; i-- {
			chunk := r.Body[i]
			if chunk.OrderCode != 1095 && chunk.OrderCode != 1003 && chunk.OrderCode != 1092 && chunk.OrderCode != 27 && chunk.OrderCode != 1052 {
				team := 0
				for _, p := range r.Summary {
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
		for p, _ := range r.Summary {
			if teamWins[r.Summary[p].Team] {
				r.Summary[p].Win = true
			}
		}
	}
}
