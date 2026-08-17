package iniparse

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchKey(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		// BuildCost tests
		{"BuildCost_asExpected", "  BuildCost", "BuildCost"},
		{"BuildCost_tooManySpaces", "       BuildCost", "BuildCost"},
		{"BuildCost_noSpaces", "BuildCost", "BuildCost"},
		{"BuildCost_endlineJunk", "  BuildCost OMG", "BuildCost"},
		{"BuildCost_withValue", "  BuildCost=123", "BuildCost"},
		{"BuildCost_withComment", "  BuildCost=123;comment", "BuildCost"},

		// Object tests
		{"Object_asExpected", "Object del cool", "Object"},
		{"Object_noExtra", "Object", ""},
		{"Object_leadingSpaces", "  Object", ""},
		{"Object_withSpaces", "Object SomeUnit", "Object"},

		// End tests
		{"End_asExpected", "End", "End"},
		{"End_withSpaces", "  End", "End"},
		{"End_withContent", "End some content", "End"},

		// Upgrade tests
		{"Upgrade_asExpected", "Upgrade SomeUpgrade", "Upgrade"},
		{"Upgrade_noSpaces", "Upgrade", ""},
		{"Upgrade_leadingSpaces", "  Upgrade", ""},

		// SpecialPower tests
		{"SpecialPower_asExpected", "SpecialPower SomePower", "SpecialPower"},
		{"SpecialPower_noSpaces", "SpecialPower", ""},
		{"SpecialPower_leadingSpaces", "  SpecialPower", ""},

		// KindOf tests
		{"KindOf_asExpected", "  KindOf = INFANTRY SELECTABLE", "KindOf"},
		{"KindOf_noSpaces", "KindOf", "KindOf"},
		{"KindOf_leadingSpaces", "    KindOf = VEHICLE", "KindOf"},
		{"KindOf_withValue", "  KindOf=AIRCRAFT VEHICLE", "KindOf"},

		// Edge cases
		{"EmptyString", "", ""},
		{"OnlySpaces", "   ", ""},
		{"UnknownKey", "UnknownKey", ""},
		{"PartialMatch", "Obj", ""},
		{"ObjectAsField", "Object = AmericaWarFactory", ""},
		{"ObjectAsFieldNoSpaces", "Object=AmericaWarFactory", ""},
		{"ObjectReskin", "ObjectReskin NewUnit OldUnit", "Object"},
		{"CaseSensitive", "object", ""},
		{"MixedCase", "Object", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := matchKey(tc.input)
			if tc.expected != output {
				t.Errorf("unexpected output. got: %s, expected: %s", output, tc.expected)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	reader := bytes.NewReader([]byte("Object LazrGun\n  junk=nothing\n  BuildCost=123\nEnd"))
	objectStore := &ObjectStore{}
	objectStore.parseFile(reader)
	obj, err := objectStore.GetObject(2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if obj.Name != "LazrGun" && obj.Cost != 123 {
		t.Errorf("parsed object returned bad result: %+v", obj)
	}
}

func TestObjectStoreGetObject(t *testing.T) {
	objectStore := &ObjectStore{
		Object: []Object{
			{Name: "Unit1", Cost: 100},
			{Name: "Unit2", Cost: 200},
			{Name: "Unit3", Cost: 300},
		},
	}

	cases := []struct {
		name        string
		id          int
		expected    *Object
		expectError bool
	}{
		{"IDTooLow", 1, nil, true},
		{"IDTooLowZero", 0, nil, true},
		{"IDTooLowNegative", -1, nil, true},
		{"FirstObject", 2, &Object{Name: "Unit1", Cost: 100}, false},
		{"SecondObject", 3, &Object{Name: "Unit2", Cost: 200}, false},
		{"ThirdObject", 4, &Object{Name: "Unit3", Cost: 300}, false},
		{"IDTooHigh", 5, nil, true},
		{"IDWayTooHigh", 100, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := objectStore.GetObject(tc.id)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expected == nil {
				if obj != nil {
					t.Errorf("expected nil, got %+v", obj)
				}
			} else {
				if obj == nil {
					t.Errorf("expected %+v, got nil", tc.expected)
				} else if *obj != *tc.expected {
					t.Errorf("expected %+v, got %+v", tc.expected, obj)
				}
			}
		})
	}
}

func TestObjectStoreGetObjectEmpty(t *testing.T) {
	objectStore := &ObjectStore{}
	obj, err := objectStore.GetObject(2)
	if err == nil {
		t.Errorf("expected error when object store is empty")
	}
	if obj != nil {
		t.Errorf("expected nil when object store is empty")
	}
}

func TestPowerStoreGetObject(t *testing.T) {
	powerStore := &PowerStore{
		Power: []Power{
			{Name: "Power1"},
			{Name: "Power2"},
			{Name: "Power3"},
		},
	}

	cases := []struct {
		name        string
		id          int
		expected    *Power
		expectError bool
	}{
		{"IDTooLow", 1, nil, true},
		{"IDTooLowZero", 0, nil, true},
		{"IDTooLowNegative", -1, nil, true},
		{"FirstPower", 2, &Power{Name: "Power1"}, false},
		{"SecondPower", 3, &Power{Name: "Power2"}, false},
		{"ThirdPower", 4, &Power{Name: "Power3"}, false},
		{"IDTooHigh", 5, nil, true},
		{"IDWayTooHigh", 100, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			power, err := powerStore.GetPower(tc.id)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expected == nil {
				if power != nil {
					t.Errorf("expected nil, got %+v", power)
				}
			} else {
				if power == nil {
					t.Errorf("expected %+v, got nil", tc.expected)
				} else if *power != *tc.expected {
					t.Errorf("expected %+v, got %+v", tc.expected, power)
				}
			}
		})
	}
}

func TestUpgradeStoreGetObject(t *testing.T) {
	upgradeStore := &UpgradeStore{
		Upgrade: []Upgrade{
			{Name: "Upgrade1", Cost: 100},
			{Name: "Upgrade2", Cost: 200},
			{Name: "Upgrade3", Cost: 300},
		},
	}

	cases := []struct {
		name        string
		id          int
		expected    *Upgrade
		expectError bool
	}{
		{"IDBelowOffset", 2269, nil, true},
		{"IDAtOffset", 2270, &Upgrade{Name: "Upgrade1", Cost: 100}, false},
		{"IDAtOffsetPlus1", 2271, &Upgrade{Name: "Upgrade2", Cost: 200}, false},
		{"IDAtOffsetPlus2", 2272, &Upgrade{Name: "Upgrade3", Cost: 300}, false},
		{"IDTooHigh", 2273, nil, true},
		{"IDWayTooHigh", 3000, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upgrade, err := upgradeStore.GetUpgrade(tc.id)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expected == nil {
				if upgrade != nil {
					t.Errorf("expected nil, got %+v", upgrade)
				}
			} else {
				if upgrade == nil {
					t.Errorf("expected %+v, got nil", tc.expected)
				} else if *upgrade != *tc.expected {
					t.Errorf("expected %+v, got %+v", tc.expected, upgrade)
				}
			}
		})
	}
}

func TestUpgradeStoreWithBase(t *testing.T) {
	upgradeStore := &UpgradeStore{
		Upgrade: []Upgrade{
			{Name: "Upgrade1", Cost: 100},
			{Name: "Upgrade2", Cost: 200},
		},
	}

	shifted := upgradeStore.WithBase(2271)
	if shifted == upgradeStore {
		t.Fatalf("expected a new view for a non-default base")
	}
	if _, err := shifted.GetUpgrade(2270); err == nil {
		t.Errorf("expected error below shifted base, got nil")
	}
	upgrade, err := shifted.GetUpgrade(2271)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgrade.Name != "Upgrade1" {
		t.Errorf("expected Upgrade1 at shifted base, got %s", upgrade.Name)
	}

	if same := upgradeStore.WithBase(UpgradeStoreOffset); same != upgradeStore {
		t.Errorf("expected the same store for the default base")
	}
	var nilStore *UpgradeStore
	if nilStore.WithBase(2271) != nil {
		t.Errorf("expected nil view from nil store")
	}
}

func TestUpgradeBaseForVersion(t *testing.T) {
	cases := []struct {
		version string
		base    int
	}{
		{"Version 1.04", 2270}, // retail
		{"Version 1.05", 2270}, // 1.05 prerelease
		{"", 2270},
		{"1.2.4", 2270},
		{"1.3.1", 2270},
		{"1.5.1", 2270},
		{"1.5.2", 2271}, // FunctionLexicon gained an entry; name keys shifted
		{"1.5.10", 2271},
		{"1.6.0", 2271},
		{"2.0.0", 2271},
	}

	for _, tc := range cases {
		if got := UpgradeBaseForVersion(tc.version); got != tc.base {
			t.Errorf("UpgradeBaseForVersion(%q) = %d, expected %d", tc.version, got, tc.base)
		}
	}
}

func TestObjectStoreParseFile(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		expected    []Object
		expectError bool
	}{
		{
			"SingleObject",
			"Object TestUnit\n  BuildCost=100\nEnd",
			[]Object{{Name: "TestUnit", Cost: 100}},
			false,
		},
		{
			"MultipleObjects",
			"Object Unit1\n  BuildCost=100\nEnd\nObject Unit2\n  BuildCost=200\nEnd",
			[]Object{{Name: "Unit1", Cost: 100}, {Name: "Unit2", Cost: 200}},
			false,
		},
		{
			"ObjectWithComment",
			"Object TestUnit\n  BuildCost=100;comment\nEnd",
			[]Object{{Name: "TestUnit", Cost: 100}},
			false,
		},
		{
			"ObjectWithSpaces",
			"Object TestUnit\n  BuildCost = 100 \nEnd",
			[]Object{{Name: "TestUnit", Cost: 100}},
			false,
		},
		{
			"ObjectWithoutCost",
			"Object TestUnit\nEnd",
			[]Object{{Name: "TestUnit", Cost: 0}},
			false,
		},
		{
			"ObjectWithoutEnd",
			"Object TestUnit\n  BuildCost=100",
			[]Object{{Name: "TestUnit", Cost: 100}},
			false,
		},
		{
			"EmptyFile",
			"",
			[]Object{},
			false,
		},
		{
			"ObjectWithoutName",
			"Object\n  BuildCost=100\nEnd",
			nil,
			true,
		},
		{
			"BuildCostWithoutObject",
			"  BuildCost=100",
			nil,
			true,
		},
		{
			"InvalidCost",
			"Object TestUnit\n  BuildCost=invalid\nEnd",
			nil,
			true,
		},
		{
			"BuildCostWithoutValue",
			"Object TestUnit\n  BuildCost=\nEnd",
			nil,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			objectStore := &ObjectStore{}
			err := objectStore.parseFile(reader)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(objectStore.Object) != len(tc.expected) {
				t.Errorf("expected %d objects, got %d", len(tc.expected), len(objectStore.Object))
				return
			}

			for i, expected := range tc.expected {
				if objectStore.Object[i] != expected {
					t.Errorf("object %d: expected %+v, got %+v", i, expected, objectStore.Object[i])
				}
			}
		})
	}
}

func TestPowerStoreParseFile(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		expected    []Power
		expectError bool
	}{
		{
			"SinglePower",
			"SpecialPower TestPower\nEnd",
			[]Power{{Name: "TestPower"}},
			false,
		},
		{
			"MultiplePowers",
			"SpecialPower Power1\nEnd\nSpecialPower Power2\nEnd",
			[]Power{{Name: "Power1"}, {Name: "Power2"}},
			false,
		},
		{
			"PowerWithoutEnd",
			"SpecialPower TestPower",
			[]Power{{Name: "TestPower"}},
			false,
		},
		{
			"EmptyFile",
			"",
			[]Power{},
			false,
		},
		{
			"PowerWithoutName",
			"SpecialPower\nEnd",
			[]Power{},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			powerStore := &PowerStore{}
			err := powerStore.parseFile(reader)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(powerStore.Power) != len(tc.expected) {
				t.Errorf("expected %d powers, got %d", len(tc.expected), len(powerStore.Power))
				return
			}

			for i, expected := range tc.expected {
				if powerStore.Power[i] != expected {
					t.Errorf("power %d: expected %+v, got %+v", i, expected, powerStore.Power[i])
				}
			}
		})
	}
}

func TestUpgradeStoreParseFile(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		expected    []Upgrade
		expectError bool
	}{
		{
			"SingleUpgrade",
			"Upgrade TestUpgrade\n  BuildCost=100\nEnd",
			[]Upgrade{{Name: "TestUpgrade", Cost: 100}},
			false,
		},
		{
			"MultipleUpgrades",
			"Upgrade Upgrade1\n  BuildCost=100\nEnd\nUpgrade Upgrade2\n  BuildCost=200\nEnd",
			[]Upgrade{{Name: "Upgrade1", Cost: 100}, {Name: "Upgrade2", Cost: 200}},
			false,
		},
		{
			"UpgradeWithComment",
			"Upgrade TestUpgrade\n  BuildCost=100;comment\nEnd",
			[]Upgrade{{Name: "TestUpgrade", Cost: 100}},
			false,
		},
		{
			"UpgradeWithoutCost",
			"Upgrade TestUpgrade\nEnd",
			[]Upgrade{{Name: "TestUpgrade", Cost: 0}},
			false,
		},
		{
			"UpgradeWithoutEnd",
			"Upgrade TestUpgrade\n  BuildCost=100",
			[]Upgrade{{Name: "TestUpgrade", Cost: 100}},
			false,
		},
		{
			"EmptyFile",
			"",
			[]Upgrade{},
			false,
		},
		{
			"UpgradeWithoutName",
			"Upgrade\n  BuildCost=100\nEnd",
			nil,
			true,
		},
		{
			"BuildCostWithoutUpgrade",
			"  BuildCost=100",
			nil,
			true,
		},
		{
			"InvalidCost",
			"Upgrade TestUpgrade\n  BuildCost=invalid\nEnd",
			nil,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			upgradeStore := &UpgradeStore{}
			err := upgradeStore.parseFile(reader)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(upgradeStore.Upgrade) != len(tc.expected) {
				t.Errorf("expected %d upgrades, got %d", len(tc.expected), len(upgradeStore.Upgrade))
				return
			}

			for i, expected := range tc.expected {
				if upgradeStore.Upgrade[i] != expected {
					t.Errorf("upgrade %d: expected %+v, got %+v", i, expected, upgradeStore.Upgrade[i])
				}
			}
		})
	}
}

func TestNewObjectStore(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	objectDir := filepath.Join(tempDir, "Object")
	err := os.MkdirAll(objectDir, 0755)
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Create test INI files
	testFiles := []struct {
		filename string
		content  string
	}{
		{
			"test1.ini",
			"Object TestUnit1\n  BuildCost=100\nEnd\nObject TestUnit2\n  BuildCost=200\nEnd",
		},
		{
			"test2.ini",
			"Object TestUnit3\n  BuildCost=300\nEnd",
		},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(objectDir, tf.filename)
		err := os.WriteFile(filePath, []byte(tf.content), 0644)
		if err != nil {
			t.Fatalf("failed to write test file %s: %v", tf.filename, err)
		}
	}

	// Test successful creation
	objectStore, err := NewObjectStore(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if objectStore == nil {
		t.Errorf("expected non-nil object store")
		return
	}

	// Verify objects were loaded
	expectedCount := 3 // TestUnit1, TestUnit2, TestUnit3
	if len(objectStore.Object) != expectedCount {
		t.Errorf("expected %d objects, got %d", expectedCount, len(objectStore.Object))
	}

	// Test with non-existent directory
	_, err = NewObjectStore("/non/existent/directory")
	if err == nil {
		t.Errorf("expected error for non-existent directory")
	}
}

func TestNewPowerStore(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create test SpecialPower.ini file
	powerContent := "SpecialPower TestPower1\nEnd\nSpecialPower TestPower2\nEnd"
	powerFile := filepath.Join(tempDir, "SpecialPower.ini")
	err := os.WriteFile(powerFile, []byte(powerContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test successful creation
	powerStore, err := NewPowerStore(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if powerStore == nil {
		t.Errorf("expected non-nil power store")
		return
	}

	// Verify powers were loaded
	expectedCount := 2
	if len(powerStore.Power) != expectedCount {
		t.Errorf("expected %d powers, got %d", expectedCount, len(powerStore.Power))
	}

	// Test with non-existent directory
	_, err = NewPowerStore("/non/existent/directory")
	if err == nil {
		t.Errorf("expected error for non-existent directory")
	}
}

func TestNewUpgradeStore(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create test Upgrade.ini file
	upgradeContent := "Upgrade TestUpgrade1\n  BuildCost=100\nEnd\nUpgrade TestUpgrade2\n  BuildCost=200\nEnd"
	upgradeFile := filepath.Join(tempDir, "Upgrade.ini")
	err := os.WriteFile(upgradeFile, []byte(upgradeContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test successful creation
	upgradeStore, err := NewUpgradeStore(tempDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if upgradeStore == nil {
		t.Errorf("expected non-nil upgrade store")
		return
	}

	// Verify upgrades were loaded
	expectedCount := 2
	if len(upgradeStore.Upgrade) != expectedCount {
		t.Errorf("expected %d upgrades, got %d", expectedCount, len(upgradeStore.Upgrade))
	}

	// Test with non-existent directory
	_, err = NewUpgradeStore("/non/existent/directory")
	if err == nil {
		t.Errorf("expected error for non-existent directory")
	}
}

func TestIntegrationComplexINI(t *testing.T) {
	// Test complex INI content with mixed sections and edge cases
	complexContent := `Object ComplexUnit
  BuildCost=500
  ; This is a comment
  SomeOtherProperty=value
End

Object AnotherUnit
  BuildCost=750;inline comment
  ; Another comment line
End

Object UnitWithoutCost
  ; No cost specified
End

; Comment at start of line
Object FinalUnit
  BuildCost=1000
End`

	reader := strings.NewReader(complexContent)
	objectStore := &ObjectStore{}
	err := objectStore.parseFile(reader)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedObjects := []Object{
		{Name: "ComplexUnit", Cost: 500},
		{Name: "AnotherUnit", Cost: 750},
		{Name: "UnitWithoutCost", Cost: 0},
		{Name: "FinalUnit", Cost: 1000},
	}

	if len(objectStore.Object) != len(expectedObjects) {
		t.Errorf("expected %d objects, got %d", len(expectedObjects), len(objectStore.Object))
		return
	}

	for i, expected := range expectedObjects {
		if objectStore.Object[i] != expected {
			t.Errorf("object %d: expected %+v, got %+v", i, expected, objectStore.Object[i])
		}
	}
}

func TestIntegrationMixedContent(t *testing.T) {
	// Test INI content with mixed object types and various formatting
	mixedContent := `Object InfantryUnit
  BuildCost=50
End

SpecialPower Airstrike
End

Upgrade WeaponUpgrade
  BuildCost=200
End

Object VehicleUnit
  BuildCost=300
End

SpecialPower Artillery
End`

	// Test ObjectStore parsing (should ignore SpecialPower and Upgrade)
	reader := strings.NewReader(mixedContent)
	objectStore := &ObjectStore{}
	err := objectStore.parseFile(reader)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedObjects := []Object{
		{Name: "InfantryUnit", Cost: 50},
		{Name: "VehicleUnit", Cost: 300},
	}

	if len(objectStore.Object) != len(expectedObjects) {
		t.Errorf("expected %d objects, got %d", len(expectedObjects), len(objectStore.Object))
		return
	}

	for i, expected := range expectedObjects {
		if objectStore.Object[i] != expected {
			t.Errorf("object %d: expected %+v, got %+v", i, expected, objectStore.Object[i])
		}
	}
}

func TestParseKindOfFromLine(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Normal", "  KindOf = INFANTRY SELECTABLE SCORE", []string{"INFANTRY", "SELECTABLE", "SCORE"}},
		{"WithComment", "  KindOf = VEHICLE SELECTABLE ; some comment", []string{"VEHICLE", "SELECTABLE"}},
		{"Empty", "  KindOf = ", nil},
		{"WithCarriageReturn", "  KindOf = AIRCRAFT VEHICLE\r", []string{"AIRCRAFT", "VEHICLE"}},
		{"NoEquals", "  KindOf INFANTRY", nil},
		{"SingleFlag", "  KindOf = STRUCTURE", []string{"STRUCTURE"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseKindOfFromLine(tc.input)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("expected %d flags, got %d: %v", len(tc.expected), len(result), result)
				return
			}
			for i, expected := range tc.expected {
				if result[i] != expected {
					t.Errorf("flag %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestClassifyObject(t *testing.T) {
	cases := []struct {
		name     string
		kindOf   []string
		expected ObjectType
	}{
		{"Aircraft", []string{"VEHICLE", "SELECTABLE", "AIRCRAFT"}, ObjectTypeAircraft},
		{"Infantry", []string{"INFANTRY", "SELECTABLE", "SCORE"}, ObjectTypeInfantry},
		{"Structure", []string{"STRUCTURE", "SELECTABLE"}, ObjectTypeStructure},
		{"Vehicle", []string{"VEHICLE", "SELECTABLE"}, ObjectTypeVehicle},
		{"Unknown", []string{"SELECTABLE", "SCORE"}, ObjectTypeUnknown},
		{"Empty", []string{}, ObjectTypeUnknown},
		{"Nil", nil, ObjectTypeUnknown},
		{"AircraftOverVehicle", []string{"AIRCRAFT", "VEHICLE"}, ObjectTypeAircraft},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifyObject(tc.kindOf)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestGetObjectByName(t *testing.T) {
	objectStore := &ObjectStore{
		Object: []Object{
			{Name: "Humvee", Cost: 700, Type: ObjectTypeVehicle},
			{Name: "Ranger", Cost: 225, Type: ObjectTypeInfantry},
		},
		byName: map[string]*Object{},
	}
	// Build the map
	for i := range objectStore.Object {
		objectStore.byName[objectStore.Object[i].Name] = &objectStore.Object[i]
	}

	// Found
	obj := objectStore.GetObjectByName("Humvee")
	if obj == nil {
		t.Fatal("expected non-nil object for Humvee")
	}
	if obj.Name != "Humvee" || obj.Type != ObjectTypeVehicle {
		t.Errorf("unexpected object: %+v", obj)
	}

	// Not found
	obj = objectStore.GetObjectByName("NonExistent")
	if obj != nil {
		t.Errorf("expected nil for non-existent object, got %+v", obj)
	}

	// Nil store
	var nilStore *ObjectStore
	obj = nilStore.GetObjectByName("Humvee")
	if obj != nil {
		t.Errorf("expected nil for nil store, got %+v", obj)
	}
}

func TestParseFileWithKindOf(t *testing.T) {
	input := "Object AmericaVehicleHumvee\n  BuildCost=700\n  KindOf = VEHICLE SELECTABLE SCORE\nEnd\n" +
		"Object AmericaInfantryRanger\n  BuildCost=225\n  KindOf = INFANTRY SELECTABLE SCORE\nEnd\n" +
		"Object AmericaJetRaptor\n  BuildCost=1400\n  KindOf = AIRCRAFT VEHICLE SELECTABLE\nEnd\n" +
		"Object AmericaWarFactory\n  BuildCost=2000\n  KindOf = STRUCTURE SELECTABLE\nEnd"

	reader := strings.NewReader(input)
	objectStore := &ObjectStore{}
	err := objectStore.parseFile(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []Object{
		{Name: "AmericaVehicleHumvee", Cost: 700, Type: ObjectTypeVehicle},
		{Name: "AmericaInfantryRanger", Cost: 225, Type: ObjectTypeInfantry},
		{Name: "AmericaJetRaptor", Cost: 1400, Type: ObjectTypeAircraft},
		{Name: "AmericaWarFactory", Cost: 2000, Type: ObjectTypeStructure},
	}

	if len(objectStore.Object) != len(expected) {
		t.Fatalf("expected %d objects, got %d", len(expected), len(objectStore.Object))
	}

	for i, exp := range expected {
		if objectStore.Object[i] != exp {
			t.Errorf("object %d: expected %+v, got %+v", i, exp, objectStore.Object[i])
		}
	}
}

func TestUsesCommunityData(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"Version 1.04", false},
		{"Version 1.06", false},
		{"1.5.2", false},
		{"1.5.4", false},
		{"1.5.5", true},
		{"1.5.6", true},
		{"1.6.0", true},
		{"2.0.0", true},
		{"", false},
	}
	for _, tc := range cases {
		if got := UsesCommunityData(tc.version); got != tc.want {
			t.Errorf("UsesCommunityData(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestIniPathLess(t *testing.T) {
	// Engine ordering: stricmp on '\'-separated paths. '\' (0x5C) sorts
	// above digits but below lowercase letters, so "Faction/..." must sort
	// before "FactionBuilding.ini" even though '/' would not.
	cases := []struct {
		a, b string
		want bool
	}{
		{"Faction/America/x.ini", "FactionBuilding.ini", true},
		{"Campaign/x.ini", "Faction/x.ini", true},
		{"gc_faction/x.ini", "Nature/x.ini", true}, // lowercased 'g' < 'n'
		{"ABC.ini", "abd.ini", true},                // case-insensitive
		{"a.ini", "a.ini", false},
	}
	for _, tc := range cases {
		if got := iniPathLess(tc.a, tc.b); got != tc.want {
			t.Errorf("iniPathLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestLoadObjectsEngineOrderAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	obj := filepath.Join(dir, "Object")
	if err := os.MkdirAll(filepath.Join(obj, "Faction", "America"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(obj, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Top-level files load first (sorted), then subdirectory files.
	// ZTop.ini sorts after the subdir path but still loads first because it
	// is a top-level file. The nested redefinition of UnitA overwrites its
	// cost but must not take a new slot.
	write("ZTop.ini", "Object UnitA\n  BuildCost = 100\n  KindOf = VEHICLE\nEnd\n")
	write("Blank.ini", "")
	write(filepath.Join("Faction", "America", "UnitA.ini"), "Object UnitA\nBuildCost = 250\nKindOf = VEHICLE\nEnd\n")
	write(filepath.Join("Faction", "America", "UnitB.ini"), "Object UnitB\nBuildCost = 50\nKindOf = INFANTRY\nEnd\n")

	store, err := NewObjectStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Object{
		{Name: "UnitA", Cost: 250, Type: ObjectTypeVehicle},
		{Name: "UnitB", Cost: 50, Type: ObjectTypeInfantry},
	}
	if len(store.Object) != len(want) {
		t.Fatalf("expected %d objects, got %d: %+v", len(want), len(store.Object), store.Object)
	}
	for i, exp := range want {
		if store.Object[i] != exp {
			t.Errorf("object %d: expected %+v, got %+v", i, exp, store.Object[i])
		}
	}
}

func TestParseFilePrerequisiteObjectLines(t *testing.T) {
	// Community-patch data writes nested fields at column zero, so a
	// Prerequisites entry must not start a new object, and a module KindOf
	// after the object's own must not reclassify it.
	input := "Object AmericaWarFactory\n" +
		"BuildCost = 2000\n" +
		"KindOf = STRUCTURE SELECTABLE\n" +
		"Prerequisites\n" +
		"Object = AmericaSupplyCenter\n" +
		"End\n" +
		"Behavior = HordeUpdate ModuleTag_04\n" +
		"KindOf = INFANTRY\n" +
		"End\n" +
		"End\n"
	store := &ObjectStore{}
	if err := store.parseFile(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if len(store.Object) != 1 {
		t.Fatalf("expected 1 object, got %d: %+v", len(store.Object), store.Object)
	}
	got := store.Object[0]
	want := Object{Name: "AmericaWarFactory", Cost: 2000, Type: ObjectTypeStructure}
	if got != want {
		t.Errorf("expected %+v, got %+v", got, want)
	}
}

func TestUpgradeStoreOverwritesDuplicate(t *testing.T) {
	// Retail Upgrade.ini defines SupW_Upgrade_AmericaPointDefenseDrone
	// twice; the engine overwrites in place without a new slot, so the
	// entry after the duplicate keeps the duplicate's position.
	input := "Upgrade First\n  BuildCost = 100\nEnd\n" +
		"Upgrade First\n  BuildCost = 150\nEnd\n" +
		"Upgrade Second\n  BuildCost = 200\nEnd\n"
	store := &UpgradeStore{}
	if err := store.parseFile(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	want := []Upgrade{{Name: "First", Cost: 150}, {Name: "Second", Cost: 200}}
	if len(store.Upgrade) != len(want) {
		t.Fatalf("expected %d upgrades, got %d: %+v", len(want), len(store.Upgrade), store.Upgrade)
	}
	for i, exp := range want {
		if store.Upgrade[i] != exp {
			t.Errorf("upgrade %d: expected %+v, got %+v", i, exp, store.Upgrade[i])
		}
	}
}
