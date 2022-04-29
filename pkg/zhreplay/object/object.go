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
	Count      int
	TotalSpent int
}

type PlayerSummary struct {
	Name           string
	Side           string
	Team           int
	Win            bool
	MoneySpent     int
	UnitsCreated   map[string]*ObjectSummary
	BuildingsBuilt map[string]*ObjectSummary
	UpgradesBuilt  map[string]*ObjectSummary
	PowersUsed     map[string]int
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
