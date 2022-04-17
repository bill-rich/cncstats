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

type PlayerInfo struct {
	Name           string
	Side           string
	MoneySpent     int
	UnitsCreated   []Unit
	BuildingsBuilt []Building
	Team           int
	Win            bool
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
