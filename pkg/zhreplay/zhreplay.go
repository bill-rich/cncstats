package zhreplay

import (
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
	"strconv"
)

type Replay struct {
	Header     *header.GeneralsHeader
	Body       []*body.BodyChunk
	PlayerInfo []*object.PlayerInfo
}

func NewReplay(bp *bitparse.BitParser) *Replay {
	replay := &Replay{}
	replay.Header = header.NewHeader(bp)
	replay.CreatePlayerList()
	replay.Body = body.ParseBody(bp, replay.PlayerInfo, bp.ObjectStore)
	replay.GenerateData()
	return replay
}

func (r *Replay) CreatePlayerList() {
	for _, playerMd := range r.Header.Metadata.Players {
		team, _ := strconv.Atoi(playerMd.Team)
		player := &object.PlayerInfo{
			Name: playerMd.Name,
			Team: team + 1,
			Win:  true,
		}
		r.PlayerInfo = append(r.PlayerInfo, player)
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
	for _, player := range r.PlayerInfo {
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
				player.UnitsCreated = append(player.UnitsCreated, unit)
				player.MoneySpent += unit.Cost
			}
			if order.OrderCode == 1049 {
				building := object.Building{
					Name: order.Details.GetName(),
					Cost: order.Details.GetCost(),
				}
				player.BuildingsBuilt = append(player.BuildingsBuilt, building)
				player.MoneySpent += building.Cost
			}
			if order.OrderCode == 1093 {
				player.Win = false
			}
		}
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
