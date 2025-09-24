package iniparse

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
	nilString         = ""
	ObjectStart       = "Object"
	ObjectEnd         = "End"
	Cost              = "BuildCost"
	ObjectStoreOffset = 2
	// UpgradeStoreOffset is 2270 because upgrades are usually part of the object listing. This is where the upgrades start.
	UpgradeStoreOffset = 2270
	PowerStoreOffset   = 2
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
	if dir == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}
	objectStore := &ObjectStore{
		Object: []Object{},
	}
	err := objectStore.loadObjects(dir)
	return objectStore, err
}

func (o *ObjectStore) GetObject(i int) (*Object, error) {
	if i < ObjectStoreOffset {
		return nil, fmt.Errorf("object ID %d is below minimum %d", i, ObjectStoreOffset)
	}
	index := i - ObjectStoreOffset
	if index >= len(o.Object) {
		return nil, fmt.Errorf("object ID %d is out of range (max: %d)", i, len(o.Object)+ObjectStoreOffset-1)
	}
	return &o.Object[index], nil
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
		file.Close() // Ensure file is closed
		if err != nil {
			return err
		}
	}
	return nil
}

func NewPowerStore(dir string) (*PowerStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}
	powerStore := &PowerStore{
		Power: []Power{},
	}
	err := powerStore.loadPowers(dir)
	return powerStore, err
}

func (p *PowerStore) GetObject(i int) (*Power, error) {
	if i < PowerStoreOffset {
		return nil, fmt.Errorf("power ID %d is below minimum %d", i, PowerStoreOffset)
	}
	index := i - PowerStoreOffset
	if index >= len(p.Power) {
		return nil, fmt.Errorf("power ID %d is out of range (max: %d)", i, len(p.Power)+PowerStoreOffset-1)
	}
	return &p.Power[index], nil
}

func (p *PowerStore) loadPowers(dir string) error {
	file, err := os.Open(dir + "/SpecialPower.ini")
	if err != nil {
		return err
	}
	defer file.Close() // Ensure file is closed
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
			name, err := parseNameFromLine(line)
			if err != nil {
				return err
			}
			power = &Power{
				Name: name,
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
	if dir == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}
	upgradeStore := &UpgradeStore{
		Upgrade: []Upgrade{},
	}
	err := upgradeStore.loadUpgrades(dir)
	return upgradeStore, err
}

func (u *UpgradeStore) GetObject(i int) (*Upgrade, error) {
	max := len(u.Upgrade) + UpgradeStoreOffset
	if i < UpgradeStoreOffset {
		return nil, fmt.Errorf("upgrade ID %d is below minimum %d", i, UpgradeStoreOffset)
	}
	if i >= max {
		return nil, fmt.Errorf("upgrade ID %d is out of range (max: %d)", i, max-1)
	}
	return &u.Upgrade[i-UpgradeStoreOffset], nil
}

func (u *UpgradeStore) loadUpgrades(dir string) error {
	file, err := os.Open(dir + "/Upgrade.ini")
	if err != nil {
		return err
	}
	defer file.Close() // Ensure file is closed
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
			name, err := parseNameFromLine(line)
			if err != nil {
				return err
			}
			upgrade = &Upgrade{
				Name: name,
			}
		case "BuildCost":
			if upgrade == nil {
				return fmt.Errorf("need an upgrade to store cost")
			}
			cost, err := parseCostFromLine(line)
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

// parseCostFromLine extracts the cost value from a BuildCost line
func parseCostFromLine(line string) (int, error) {
	fields := strings.Split(line, "=")
	if len(fields) < 2 {
		return 0, fmt.Errorf("cannot find cost value")
	}
	fieldsComment := strings.Split(fields[1], ";")
	costString := strings.ReplaceAll(fieldsComment[0], " ", "")
	cost, err := strconv.Atoi(costString)
	if err != nil {
		return 0, fmt.Errorf("invalid cost value: %w", err)
	}
	return cost, nil
}

// parseNameFromLine extracts the name from an Object/Upgrade/SpecialPower line
func parseNameFromLine(line string) (string, error) {
	fields := strings.Split(line, " ")
	if len(fields) < 2 {
		return "", fmt.Errorf("could not get name from line: %s", line)
	}
	return fields[1], nil
}

func matchKey(line string) string {
	for _, key := range IniKey {
		// Handle keys with leading spaces (like "  BuildCost")
		if strings.HasPrefix(line, key) {
			// Return the key without leading spaces for consistency
			return strings.TrimLeft(key, " ")
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
			name, err := parseNameFromLine(line)
			if err != nil {
				return err
			}
			object = &Object{
				Name: name,
			}
		case "BuildCost":
			if object == nil {
				return fmt.Errorf("need an object to store cost")
			}
			cost, err := parseCostFromLine(line)
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
