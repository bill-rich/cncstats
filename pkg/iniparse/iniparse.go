package iniparse

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	// index maps object name to its slot in Object. The engine deals
	// ThingTemplate IDs in first-definition order and a redefinition of an
	// existing name overwrites the template in place without taking a new
	// ID, so the parser must do the same.
	index  map[string]int
	byName map[string]*Object
}

type Object struct {
	Name string
	Cost int
	Type ObjectType
}

type UpgradeStore struct {
	Upgrade []Upgrade
	// base is the name-key value of the first Upgrade.ini entry for the
	// client that recorded the replay being parsed. Zero means the retail
	// default (UpgradeStoreOffset). Use WithBase to get a view for a
	// different client version; see UpgradeBaseForVersion.
	base int
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
	// UpgradeStoreOffset is the engine name key of the first Upgrade.ini
	// entry. The BuildUpgrade replay argument is a NameKey, handed out in
	// interning order during engine init, so this base moves whenever a
	// client interns one more name before Upgrade.ini loads. 2270 is
	// correct for retail 1.04 and zulu clients through 1.5.1; see
	// UpgradeBaseForVersion for later clients.
	UpgradeStoreOffset = 2270
	PowerStoreOffset   = 2
	// StableUpgradeIDBase is the upgrade ID base for clients that record
	// stable upgrade ids instead of name keys (zulu 1.5.5+). The stable id
	// is the upgrade's mask bit, dealt in template-creation order: the
	// UpgradeCenter creates three veterancy upgrades (Veteran/Elite/Heroic)
	// in init, then Data\INI\Default\Upgrade.ini's DefaultUpgrade parses,
	// so bits 0-3 are taken and the first Upgrade.ini entry gets 4.
	StableUpgradeIDBase = 4
)

// upgradeBaseChanges lists, oldest first, the zulu client versions at which
// the upgrade name-key base moved, and the base they moved it to. A version
// belongs to the last entry it is >= to; earlier versions (and retail, whose
// header version is "Version 1.04" rather than a bare semver) use
// UpgradeStoreOffset.
//
// 1.5.2: commit 92674b2 in GeneralsGameCode added MultiplayerLoadScreenSystem
// to the FunctionLexicon's gameWinSystemTable. TheFunctionLexicon interns a
// name key per table entry and initializes before TheUpgradeCenter, so every
// upgrade key shifted up by one. Any future change that interns a name before
// Upgrade.ini loads (FunctionLexicon entries, sciences, player templates,
// objects, locomotors, ...) needs a new entry here.
var upgradeBaseChanges = []struct {
	major, minor, patch int
	base                int
}{
	{1, 5, 2, 2271},
}

// UpgradeBaseForVersion returns the upgrade name-key base used by the client
// that wrote a replay, given the replay header's version string. Zulu
// releases write a bare semver ("1.5.2"); anything else (retail's
// "Version 1.04", mods, dev builds) gets the retail base.
func UpgradeBaseForVersion(version string) int {
	nums, ok := parseSemVer(version)
	if !ok {
		return UpgradeStoreOffset
	}
	base := UpgradeStoreOffset
	for _, change := range upgradeBaseChanges {
		if atLeast(nums, change.major, change.minor, change.patch) {
			base = change.base
		}
	}
	return base
}

// parseSemVer parses a bare "major.minor.patch" version as written by zulu
// clients into a replay header. Anything else (retail's "Version 1.04",
// mods, dev builds) fails the parse.
func parseSemVer(version string) ([3]int, bool) {
	var nums [3]int
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) != 3 {
		return nums, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nums, false
		}
		nums[i] = n
	}
	return nums, true
}

func atLeast(nums [3]int, major, minor, patch int) bool {
	if nums[0] != major {
		return nums[0] > major
	}
	if nums[1] != minor {
		return nums[1] > minor
	}
	return nums[2] >= patch
}

// UsesCommunityData reports whether the client that recorded a replay ships
// the community balance patch data (zulu 1.5.5+). Those releases rebuilt the
// whole Data\INI\Object tree into per-unit files, reordering ThingTemplate
// registration wholesale, and started recording upgrade purchases as stable
// upgrade ids (mask bits) instead of raw name keys. Replays from them must be
// decoded against the INI_v155 stores with StableUpgradeIDBase.
func UsesCommunityData(version string) bool {
	nums, ok := parseSemVer(version)
	return ok && atLeast(nums, 1, 5, 5)
}

// The community-data stores (built from inizh/Data/INI_v155) are registered
// once at startup and consulted per replay by recording version. When nothing
// is registered every replay falls back to the legacy stores, which is the
// pre-1.5.5 behavior.
var (
	communityObjectStore  *ObjectStore
	communityUpgradeStore *UpgradeStore
)

func RegisterCommunityStores(o *ObjectStore, u *UpgradeStore) {
	communityObjectStore = o
	communityUpgradeStore = u
}

func CommunityObjectStore() *ObjectStore   { return communityObjectStore }
func CommunityUpgradeStore() *UpgradeStore { return communityUpgradeStore }

// Block-opening keys are only recognized when followed by a name rather than
// '=': the community-patch data normalizes every field to column zero, so a
// Prerequisites field like "Object = AmericaWarFactory" must not read as a
// new definition. Field keys are matched at any indentation for the same
// reason. ObjectReskin defines a new template (with its own ID) named by its
// first argument, so it counts as an Object definition.
var iniBlockKeys = map[string]string{
	"Object":           "Object",
	"ObjectReskin":     "Object",
	"Upgrade":          "Upgrade",
	"SpecialPower":     "SpecialPower",
	"MultiplayerColor": "MultiplayerColor",
}

var iniFieldKeys = map[string]string{
	"BuildCost":     "BuildCost",
	"KindOf":        "KindOf",
	"RGBColor":      "RGBColor",
	"RGBNightColor": "RGBNightColor",
	"TooltipName":   "TooltipName",
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
	root := filepath.Join(dir, "Object")
	var rels []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".ini") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return err
	}

	// The engine (INI::loadDirectory) loads the files of the Object tree in
	// two passes over one list sorted case-insensitively by full path with
	// '\' separators: first every file directly in Object\, then every file
	// in a subdirectory. Object IDs are dealt in that order, so replicate it
	// exactly.
	sort.Slice(rels, func(i, j int) bool { return iniPathLess(rels[i], rels[j]) })
	ordered := make([]string, 0, len(rels))
	for _, rel := range rels {
		if !strings.ContainsRune(rel, '/') {
			ordered = append(ordered, rel)
		}
	}
	for _, rel := range rels {
		if strings.ContainsRune(rel, '/') {
			ordered = append(ordered, rel)
		}
	}

	o.index = make(map[string]int)
	for _, rel := range ordered {
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
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

// iniPathLess orders INI paths the way the engine's FilenameList does:
// stricmp on the full path with '\' separators. The separator mapping
// matters because '\' (0x5C) sorts above digits but below lowercase letters,
// while '/' (0x2F) sorts below both.
func iniPathLess(a, b string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		ca, cb := iniPathByte(a[i]), iniPathByte(b[i])
		if ca != cb {
			return ca < cb
		}
	}
	return len(a) < len(b)
}

func iniPathByte(c byte) byte {
	if c == '/' {
		return '\\'
	}
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
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

// Base returns the name-key value this store maps to its first upgrade.
func (u *UpgradeStore) Base() int {
	if u.base != 0 {
		return u.base
	}
	return UpgradeStoreOffset
}

// WithBase returns a view of the store whose upgrade IDs start at base. The
// view shares the backing upgrade list, so it is cheap to create per replay
// and safe to use alongside the original from concurrent readers. A nil
// receiver returns nil.
func (u *UpgradeStore) WithBase(base int) *UpgradeStore {
	if u == nil || base == u.Base() {
		return u
	}
	return &UpgradeStore{Upgrade: u.Upgrade, base: base}
}

func (u *UpgradeStore) GetUpgrade(i int) (*Upgrade, error) {
	base := u.Base()
	max := len(u.Upgrade) + base
	if i < base {
		return nil, fmt.Errorf("upgrade ID %d is below minimum %d", i, base)
	}
	if i >= max {
		return nil, fmt.Errorf("upgrade ID %d is out of range (max: %d)", i, max-1)
	}
	return &u.Upgrade[i-base], nil
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
	// Upgrade IDs (name keys and stable mask bits alike) are dealt in
	// first-definition order; a redefinition of an existing name (retail
	// Upgrade.ini repeats SupW_Upgrade_AmericaPointDefenseDrone) overwrites
	// in place without taking a new slot, matching the engine.
	index := make(map[string]int, len(u.Upgrade))
	for i := range u.Upgrade {
		index[u.Upgrade[i].Name] = i
	}
	scanner := bufio.NewScanner(file)
	cur := -1
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "Upgrade":
			name, err := parseNameFromLine(line)
			if err != nil {
				return err
			}
			if idx, ok := index[name]; ok {
				cur = idx
			} else {
				index[name] = len(u.Upgrade)
				cur = len(u.Upgrade)
				u.Upgrade = append(u.Upgrade, Upgrade{Name: name})
			}
		case "BuildCost":
			if cur < 0 {
				return fmt.Errorf("need an upgrade to store cost")
			}
			cost, err := parseCostFromLine(line)
			if err != nil {
				return err
			}
			u.Upgrade[cur].Cost = cost
		case "End":
		default:
		}
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
	fields := strings.Fields(line)
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
	// The engine tokenizer treats '=' as whitespace, so "BuildCost=123"
	// and "BuildCost = 123" are the same line.
	fields := strings.Fields(strings.ReplaceAll(line, "=", " = "))
	if len(fields) == 0 {
		return ""
	}
	if key, ok := iniBlockKeys[fields[0]]; ok {
		if len(fields) >= 2 && fields[1] != "=" {
			return key
		}
		return ""
	}
	if key, ok := iniFieldKeys[fields[0]]; ok {
		return key
	}
	if fields[0] == "End" {
		return "End"
	}
	return ""
}

func (o *ObjectStore) parseFile(file io.Reader) error {
	if o.index == nil {
		o.index = make(map[string]int)
	}
	scanner := bufio.NewScanner(file)
	cur := -1
	// Nested module fields can shadow an object's own fields: HordeUpdate
	// and GrantStealthBehavior have their own KindOf. In retail data the
	// object's own fields sit at the shallowest indentation, so shallower
	// KindOf lines win over deeper ones. The community-patch data writes
	// everything at column zero, where indentation cannot tell them apart;
	// there the object's own KindOf comes before its modules, so at equal
	// indentation a line that classifies to a known type wins over one
	// that does not, and the first known type sticks. BuildCost appears
	// once per definition, so its first value wins.
	costSet := false
	kindIndent := -1
	typeKnown := false
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "Object":
			name, err := parseNameFromLine(line)
			if err != nil {
				return err
			}
			costSet, kindIndent, typeKnown = false, -1, false
			if idx, ok := o.index[name]; ok {
				// Redefinition: the engine overwrites the existing
				// template in place, keeping its ID, so later fields
				// update the original slot.
				cur = idx
			} else {
				o.index[name] = len(o.Object)
				cur = len(o.Object)
				o.Object = append(o.Object, Object{Name: name})
			}
		case "BuildCost":
			if cur < 0 {
				return fmt.Errorf("need an object to store cost")
			}
			if costSet {
				break
			}
			cost, err := parseCostFromLine(line)
			if err != nil {
				return err
			}
			o.Object[cur].Cost = cost
			costSet = true
		case "KindOf":
			if cur < 0 {
				break
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if kindIndent >= 0 && indent > kindIndent {
				break
			}
			t := classifyObject(parseKindOfFromLine(line))
			shallower := kindIndent < 0 || indent < kindIndent
			if shallower || t != ObjectTypeUnknown && !typeKnown {
				o.Object[cur].Type = t
				typeKnown = t != ObjectTypeUnknown
			}
			kindIndent = indent
		case "End":
		default:
		}
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
	if err := c.loadColorFile(dir+"/multiplayer.ini", true); err != nil {
		return err
	}
	return c.loadColorFile(dir+"/ZuluColors.ini", false)
}

func (c *ColorStore) loadColorFile(path string, required bool) error {
	file, err := os.Open(path)
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	return c.parseFile(file)
}

// commitColor adds a parsed color to the store, replicating the engine's
// override behavior (INI::parseMultiplayerColorDefinition): if the block's
// name matches an existing color's TooltipName, overwrite that entry in place
// rather than appending a duplicate. The original Name is preserved so the
// color's reported identity and index are unchanged; only the swatch values
// are updated. This is how the Zulu "Color:Purple" block re-tints the built-in
// Purple without shifting later indices.
func (c *ColorStore) commitColor(color *MultiplayerColor) {
	for i := range c.Color {
		if c.Color[i].TooltipName == color.Name {
			c.Color[i].RGBColor = color.RGBColor
			c.Color[i].RGBNightColor = color.RGBNightColor
			return
		}
	}
	c.Color = append(c.Color, *color)
}

func (c *ColorStore) parseFile(file io.Reader) error {
	scanner := bufio.NewScanner(file)
	var color *MultiplayerColor
	for scanner.Scan() {
		line := scanner.Text()
		switch matchKey(line) {
		case "MultiplayerColor":
			if color != nil {
				c.commitColor(color)
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
		c.commitColor(color)
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
