package iniparse

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

type ObjectStore struct {
	Object []Object
}

type Object struct {
	Name string
	Cost int
}

type UpgradeStore struct {
	Upgrade []Upgrade
}

type Upgrade struct {
	Name string
	Cost int
}

type PowerStore struct {
	Power []Power
}

type Power struct {
	Name string
}

const (
	nilString   = ""
	ObjectStart = "Object"
	ObjectEnd   = "End"
	Cost        = "BuildCost"
)

var IniKey = []string{
	"Object",
	"End",
	"  BuildCost",
	"Upgrade",
	"SpecialPower",
	/*
		"OkToChangeModelColor",
		"ConditionState",
		"ArmorSet",
		"Body",
		"Behavior",
		"Draw",
		"ClientUpdate",
		"DefaultConditionState",
		"TransitionState",
		"WeaponSet",
		"UnitSpecificSounds",
		"Prerequisites",
		"Turret",
		"AttackAreaDecal",
		"TargetingReticleDecal",
	*/
}

func NewObjectStore(dir string) (*ObjectStore, error) {
	objectStore := &ObjectStore{
		Object: []Object{},
	}
	err := objectStore.loadObjects(dir)
	return objectStore, err
}

func (o *ObjectStore) GetObject(i int) *Object {
	if i < 2 {
		return nil
	}
	index := i - 2
	if index >= len(o.Object) {
		return nil
	}
	return &o.Object[index]
}

func (o *ObjectStore) loadObjects(dir string) error {
	dirItems, err := os.ReadDir(dir + "/Object/")
	if err != nil {
		return err
	}

	for _, dirItem := range dirItems {
		file, err := os.Open(dir + "/Object/" + dirItem.Name())
		if err != nil {
			return err
		}
		err = o.parseFile(file)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewPowerStore(dir string) (*PowerStore, error) {
	powerStore := &PowerStore{
		Power: []Power{},
	}
	err := powerStore.loadObjects(dir)
	return powerStore, err
}

func (p *PowerStore) GetObject(i int) *Power {
	if i < 2 {
		return nil
	}
	index := i - 2
	if index >= len(p.Power) {
		return nil
	}
	return &p.Power[index]
}

func (p *PowerStore) loadObjects(dir string) error {
	file, err := os.Open(dir + "/SpecialPower.ini")
	if err != nil {
		return err
	}
	err = p.parseFile(file)
	if err != nil {
		return err
	}
	return nil
}

func (p *PowerStore) parseFile(file io.Reader) error {
	scanner := bufio.NewScanner(file)
	var power *Power
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "SpecialPower":
			if power != nil {
				p.Power = append(p.Power, *power)
			}
			fields := strings.Split(line, " ")
			if len(fields) < 2 {
				return fmt.Errorf("could not get power name from line: %s", line)
			}
			power = &Power{
				Name: fields[1],
			}
		case "End":
		default:
		}
	}
	if power != nil {
		p.Power = append(p.Power, *power)
	}
	return nil
}

func NewUpgradeStore(dir string) (*UpgradeStore, error) {
	upgradeStore := &UpgradeStore{
		Upgrade: []Upgrade{},
	}
	err := upgradeStore.loadObjects(dir)
	return upgradeStore, err
}

func (u *UpgradeStore) GetObject(i int) *Upgrade {
	offset := 2270 // It seems that usually the upgrades are part of the object listing. This is where the upgrades start.
	max := len(u.Upgrade) + offset
	if i < offset || i >= max {
		log.WithField("upgradeId", i).WithField("offset", offset).WithField("max", max).Errorf("upgradeId is out of range")
		return nil
	}
	return &u.Upgrade[i-offset]
}

func (u *UpgradeStore) loadObjects(dir string) error {
	file, err := os.Open(dir + "/Upgrade.ini")
	if err != nil {
		return err
	}
	err = u.parseFile(file)
	if err != nil {
		return err
	}
	return nil
}

func (u *UpgradeStore) parseFile(file io.Reader) error {
	scanner := bufio.NewScanner(file)
	var upgrade *Upgrade
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "Upgrade":
			if upgrade != nil {
				u.Upgrade = append(u.Upgrade, *upgrade)
			}
			fields := strings.Split(line, " ")
			if len(fields) < 2 {
				return fmt.Errorf("could not get upgrade name from line: %s", line)
			}
			upgrade = &Upgrade{
				Name: fields[1],
			}
		case "BuildCost":
			if upgrade == nil {
				return fmt.Errorf("need an object to store cost")
			}
			fields := strings.Split(line, "=")
			if len(fields) < 2 {
				return fmt.Errorf("cannot find object cost")
			}
			fieldsComment := strings.Split(fields[1], ";")
			costString := strings.ReplaceAll(fieldsComment[0], " ", "")
			cost, err := strconv.Atoi(costString)
			if err != nil {
				return err
			}
			upgrade.Cost = cost
		case "End":
		default:
		}
	}
	if upgrade != nil {
		u.Upgrade = append(u.Upgrade, *upgrade)
	}
	return nil
}

func matchKey(line string) string {
	for _, key := range IniKey {
		if strings.HasPrefix(line, key) {
			return strings.ReplaceAll(key, " ", "")
		}
	}
	return nilString
}

func (o *ObjectStore) parseFile(file io.Reader) error {
	scanner := bufio.NewScanner(file)
	var object *Object
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "Object":
			if object != nil {
				o.Object = append(o.Object, *object)
			}
			fields := strings.Split(line, " ")
			if len(fields) < 2 {
				return fmt.Errorf("could not get object name from line: %s", line)
			}
			object = &Object{
				Name: fields[1],
			}
		case "BuildCost":
			if object == nil {
				return fmt.Errorf("need an object to store cost")
			}
			fields := strings.Split(line, "=")
			if len(fields) < 2 {
				return fmt.Errorf("cannot find object cost")
			}
			fieldsComment := strings.Split(fields[1], ";")
			costString := strings.ReplaceAll(fieldsComment[0], " ", "")
			cost, err := strconv.Atoi(costString)
			if err != nil {
				return err
			}
			object.Cost = cost
		case "End":
		default:
		}
	}
	if object != nil {
		o.Object = append(o.Object, *object)
	}
	return nil
}
