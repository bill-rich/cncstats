package header

import (
	"reflect"
	"testing"
)

func TestMetadataParse(t *testing.T) {
	input := "US=1;M=07maps/tournament island;MC=12BE477C;MS=130668;SD=6449734;C=100;SR=0;SC=10000;O=N;S=HModus,17F04000,8088,FT,7,-1,-1,0,1:HYe_Ole_Seans,48595000,8088,FT,0,-1,-1,2,1:HOneThree111,49DDD000,8088,FT,6,-1,-1,2,1:Hjbb,18099000,8088,FT,3,-1,-1,0,1:X:X:X:X:;"
	mdOut := parseMetadata(input)
	mdExpected := Metadata{
		MapFile:         "07maps/tournament island",
		MapCRC:          "12BE477C",
		MapSize:         "130668",
		Seed:            "6449734",
		C:               "100",
		SR:              "0",
		StartingCredits: "10000",
		O:               "N",
		Players: []Player{
			{
				Type:             "H",
				Name:             "Modus",
				IP:               "17F04000",
				Port:             "8088",
				FT:               "FT",
				Color:            "7",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "0",
				Unknown:          "1",
			},
			{
				Type:             "H",
				Name:             "Ye_Ole_Seans",
				IP:               "48595000",
				Port:             "8088",
				FT:               "FT",
				Color:            "0",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "2",
				Unknown:          "1",
			},
			{
				Type:             "H",
				Name:             "OneThree111",
				IP:               "49DDD000",
				Port:             "8088",
				FT:               "FT",
				Color:            "6",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "2",
				Unknown:          "1",
			},
			{
				Type:             "H",
				Name:             "jbb",
				IP:               "18099000",
				Port:             "8088",
				FT:               "FT",
				Color:            "3",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "0",
				Unknown:          "1",
			},
		},
	}
	if !reflect.DeepEqual(mdOut, mdExpected) {
		t.Errorf("unexpected metadata parsing.\n got:%+v\n expected:%+v\n", mdOut, mdExpected)
	}
}
