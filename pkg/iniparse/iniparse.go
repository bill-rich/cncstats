package iniparse

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ObjectType classifies game objects by their category.
type ObjectType string

const (
	ObjectTypeInfantry  ObjectType = "infantry"
	ObjectTypeVehicle   ObjectType = "vehicle"
	ObjectTypeAircraft  ObjectType = "aircraft"
	ObjectTypeStructure ObjectType = "structure"
	ObjectTypeUnknown   ObjectType = "unknown"
)

type ObjectStore struct {
	Object []Object
	byName map[string]*Object
}

type Object struct {
	Name string
	Cost int
	Type ObjectType
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

type ColorStore struct {
	Color []MultiplayerColor
}

type MultiplayerColor struct {
	Name          string
	RGBColor      RGBColor
	RGBNightColor RGBColor
	TooltipName   string
}

type RGBColor struct {
	R int
	G int
	B int
}

const (
	ObjectStoreOffset = 2
	// UpgradeStoreOffset is 2270 because upgrades are usually part of the object listing. This is where the upgrades start.
	UpgradeStoreOffset = 2270
	PowerStoreOffset   = 2
)

var IniKey = []string{
	"Object",
	"End",
	"  BuildCost",
	"  KindOf",
	"Upgrade",
	"SpecialPower",
	"MultiplayerColor",
	"  RGBColor",
	"  RGBNightColor",
	"  TooltipName",
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

	// Build name lookup map
	o.byName = make(map[string]*Object, len(o.Object))
	for i := range o.Object {
		o.byName[o.Object[i].Name] = &o.Object[i]
	}
	return nil
}

// GetObjectByName returns a pointer to the Object with the given name, or nil if not found.
func (o *ObjectStore) GetObjectByName(name string) *Object {
	if o == nil || o.byName == nil {
		return nil
	}
	return o.byName[name]
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

func (p *PowerStore) GetPower(i int) (*Power, error) {
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
	defer file.Close()
	return p.parseFile(file)
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

func (u *UpgradeStore) GetUpgrade(i int) (*Upgrade, error) {
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
	defer file.Close()
	return u.parseFile(file)
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

// parseKindOfFromLine extracts KindOf flags from a line like "  KindOf = INFANTRY SELECTABLE"
func parseKindOfFromLine(line string) []string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return nil
	}
	// Strip inline comments and carriage returns
	value := strings.SplitN(parts[1], ";", 2)[0]
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return strings.Fields(value)
}

// classifyObject determines the ObjectType from KindOf flags.
// Priority: AIRCRAFT > INFANTRY > STRUCTURE > VEHICLE > unknown.
// Aircraft is checked first because aircraft objects carry both VEHICLE and AIRCRAFT flags.
func classifyObject(kindOf []string) ObjectType {
	has := make(map[string]bool, len(kindOf))
	for _, flag := range kindOf {
		has[flag] = true
	}
	switch {
	case has["AIRCRAFT"]:
		return ObjectTypeAircraft
	case has["INFANTRY"]:
		return ObjectTypeInfantry
	case has["STRUCTURE"]:
		return ObjectTypeStructure
	case has["VEHICLE"]:
		return ObjectTypeVehicle
	default:
		return ObjectTypeUnknown
	}
}

func matchKey(line string) string {
	for _, key := range IniKey {
		// Handle keys with leading spaces (like "  BuildCost")
		if strings.HasPrefix(line, key) {
			// Return the key without leading spaces for consistency
			return strings.TrimLeft(key, " ")
		}
	}
	return ""
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
		case "KindOf":
			if object == nil {
				break
			}
			flags := parseKindOfFromLine(line)
			object.Type = classifyObject(flags)
		case "End":
		default:
		}
	}
	if object != nil {
		o.Object = append(o.Object, *object)
	}
	return nil
}

func NewColorStore(dir string) (*ColorStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}
	colorStore := &ColorStore{
		Color: []MultiplayerColor{},
	}
	err := colorStore.loadColors(dir)
	return colorStore, err
}

func (c *ColorStore) GetColor(i int) (*MultiplayerColor, error) {
	if i < 0 {
		return nil, fmt.Errorf("color ID %d is below minimum 0", i)
	}
	if i >= len(c.Color) {
		return nil, fmt.Errorf("color ID %d is out of range (max: %d)", i, len(c.Color)-1)
	}
	return &c.Color[i], nil
}

// GetColorName returns the color name by ID, or an error if the ID is invalid
func (c *ColorStore) GetColorName(i int) (string, error) {
	color, err := c.GetColor(i)
	if err != nil {
		return "", err
	}
	return color.Name, nil
}

func (c *ColorStore) loadColors(dir string) error {
	file, err := os.Open(dir + "/multiplayer.ini")
	if err != nil {
		return err
	}
	defer file.Close()
	return c.parseFile(file)
}

func (c *ColorStore) parseFile(file io.Reader) error {
	scanner := bufio.NewScanner(file)
	var color *MultiplayerColor
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "MultiplayerColor":
			if color != nil {
				c.Color = append(c.Color, *color)
			}
			name, err := parseNameFromLine(line)
			if err != nil {
				return err
			}
			color = &MultiplayerColor{
				Name: name,
			}
		case "RGBColor":
			if color == nil {
				return fmt.Errorf("need a color to store RGBColor")
			}
			rgbColor, err := parseRGBFromLine(line)
			if err != nil {
				return err
			}
			color.RGBColor = rgbColor
		case "RGBNightColor":
			if color == nil {
				return fmt.Errorf("need a color to store RGBNightColor")
			}
			rgbColor, err := parseRGBFromLine(line)
			if err != nil {
				return err
			}
			color.RGBNightColor = rgbColor
		case "TooltipName":
			if color == nil {
				return fmt.Errorf("need a color to store TooltipName")
			}
			tooltipName, err := parseTooltipNameFromLine(line)
			if err != nil {
				return err
			}
			color.TooltipName = tooltipName
		case "End":
		default:
		}
	}
	if color != nil {
		c.Color = append(c.Color, *color)
	}
	return nil
}

// parseRGBFromLine extracts RGB values from a line like "RGBColor = R:221 G:226 B:13"
func parseRGBFromLine(line string) (RGBColor, error) {
	fields := strings.Split(line, "=")
	if len(fields) < 2 {
		return RGBColor{}, fmt.Errorf("cannot find RGB value")
	}

	// Remove spaces and split by R:, G:, B:
	rgbString := strings.TrimSpace(fields[1])

	// Parse R value
	rStart := strings.Index(rgbString, "R:")
	if rStart == -1 {
		return RGBColor{}, fmt.Errorf("cannot find R value")
	}
	rEnd := strings.Index(rgbString[rStart+2:], " ")
	if rEnd == -1 {
		return RGBColor{}, fmt.Errorf("cannot find end of R value")
	}
	rStr := rgbString[rStart+2 : rStart+2+rEnd]
	r, err := strconv.Atoi(rStr)
	if err != nil {
		return RGBColor{}, fmt.Errorf("invalid R value: %w", err)
	}

	// Parse G value
	gStart := strings.Index(rgbString, "G:")
	if gStart == -1 {
		return RGBColor{}, fmt.Errorf("cannot find G value")
	}
	gEnd := strings.Index(rgbString[gStart+2:], " ")
	if gEnd == -1 {
		return RGBColor{}, fmt.Errorf("cannot find end of G value")
	}
	gStr := rgbString[gStart+2 : gStart+2+gEnd]
	g, err := strconv.Atoi(gStr)
	if err != nil {
		return RGBColor{}, fmt.Errorf("invalid G value: %w", err)
	}

	// Parse B value
	bStart := strings.Index(rgbString, "B:")
	if bStart == -1 {
		return RGBColor{}, fmt.Errorf("cannot find B value")
	}
	bStr := strings.TrimSpace(rgbString[bStart+2:])
	b, err := strconv.Atoi(bStr)
	if err != nil {
		return RGBColor{}, fmt.Errorf("invalid B value: %w", err)
	}

	return RGBColor{R: r, G: g, B: b}, nil
}

// parseTooltipNameFromLine extracts the tooltip name from a line like "TooltipName = Color:Gold"
func parseTooltipNameFromLine(line string) (string, error) {
	fields := strings.Split(line, "=")
	if len(fields) < 2 {
		return "", fmt.Errorf("cannot find tooltip name value")
	}
	return strings.TrimSpace(fields[1]), nil
}
