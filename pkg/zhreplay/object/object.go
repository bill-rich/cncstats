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

type ObjectSummary struct {
	Count      int
	TotalSpent int
}

type PlayerInfo struct {
	Name           string
	Side           string
	Team           int
	Win            bool
	MoneySpent     int
	UnitsCreated   map[string]*ObjectSummary
	BuildingsBuilt map[string]*ObjectSummary
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
