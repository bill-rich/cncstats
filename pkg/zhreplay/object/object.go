package object

type Object interface {
	GetName() string
	GetCost() int
}

type Unit struct {
	Name string
	Cost int
}

type Building struct {
	Name string
	Cost int
}

type Power struct {
	Name string
}

type Upgrade struct {
	Name string
	Cost int
}

type ObjectSummary struct {
	Count      int `json:"count"`
	TotalSpent int `json:"totalSpent"`
}

type PlayerSummary struct {
	Name           string                    `json:"name"`
	Side           string                    `json:"side"`
	Team           int                       `json:"team"`
	Win            bool                      `json:"win"`
	MoneySpent     int                       `json:"moneySpent"`
	UnitsCreated   map[string]*ObjectSummary `json:"unitsCreated"`
	BuildingsBuilt map[string]*ObjectSummary `json:"buildingsBuilt"`
	UpgradesBuilt  map[string]*ObjectSummary `json:"upgradesBuilt"`
	PowersUsed     map[string]int            `json:"powersUsed"`
}

func (u *Unit) GetName() string {
	return u.Name
}

func (u *Unit) GetCost() int {
	return u.Cost
}

func (b *Building) GetName() string {
	return b.Name
}

func (b *Building) GetCost() int {
	return b.Cost
}

func (p *Power) GetName() string {
	return p.Name
}

func (p *Power) GetCost() int {
	return 0
}

func (u *Upgrade) GetName() string {
	return u.Name
}

func (u *Upgrade) GetCost() int {
	return u.Cost
}
