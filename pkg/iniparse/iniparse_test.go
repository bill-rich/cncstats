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
		{"BuildCost_tooManySpaces", "       BuildCost", ""},
		{"BuildCost_noSpaces", "BuildCost", ""},
		{"BuildCost_endlineJunk", "  BuildCost OMG", "BuildCost"},
		{"BuildCost_withValue", "  BuildCost=123", "BuildCost"},
		{"BuildCost_withComment", "  BuildCost=123;comment", "BuildCost"},

		// Object tests
		{"Object_asExpected", "Object del cool", "Object"},
		{"Object_noExtra", "Object", "Object"},
		{"Object_leadingSpaces", "  Object", ""},
		{"Object_withSpaces", "Object SomeUnit", "Object"},

		// End tests
		{"End_asExpected", "End", "End"},
		{"End_withSpaces", "  End", ""},
		{"End_withContent", "End some content", "End"},

		// Upgrade tests
		{"Upgrade_asExpected", "Upgrade SomeUpgrade", "Upgrade"},
		{"Upgrade_noSpaces", "Upgrade", "Upgrade"},
		{"Upgrade_leadingSpaces", "  Upgrade", ""},

		// SpecialPower tests
		{"SpecialPower_asExpected", "SpecialPower SomePower", "SpecialPower"},
		{"SpecialPower_noSpaces", "SpecialPower", "SpecialPower"},
		{"SpecialPower_leadingSpaces", "  SpecialPower", ""},

		// Edge cases
		{"EmptyString", "", ""},
		{"OnlySpaces", "   ", ""},
		{"UnknownKey", "UnknownKey", ""},
		{"PartialMatch", "Obj", ""},
		{"CaseSensitive", "object", ""},
		{"MixedCase", "Object", "Object"},
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
	obj := objectStore.GetObject(2)
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
		name     string
		id       int
		expected *Object
	}{
		{"IDTooLow", 1, nil},
		{"IDTooLowZero", 0, nil},
		{"IDTooLowNegative", -1, nil},
		{"FirstObject", 2, &Object{Name: "Unit1", Cost: 100}},
		{"SecondObject", 3, &Object{Name: "Unit2", Cost: 200}},
		{"ThirdObject", 4, &Object{Name: "Unit3", Cost: 300}},
		{"IDTooHigh", 5, nil},
		{"IDWayTooHigh", 100, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := objectStore.GetObject(tc.id)
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
	obj := objectStore.GetObject(2)
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
		name     string
		id       int
		expected *Power
	}{
		{"IDTooLow", 1, nil},
		{"IDTooLowZero", 0, nil},
		{"IDTooLowNegative", -1, nil},
		{"FirstPower", 2, &Power{Name: "Power1"}},
		{"SecondPower", 3, &Power{Name: "Power2"}},
		{"ThirdPower", 4, &Power{Name: "Power3"}},
		{"IDTooHigh", 5, nil},
		{"IDWayTooHigh", 100, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			power := powerStore.GetObject(tc.id)
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
		name     string
		id       int
		expected *Upgrade
	}{
		{"IDBelowOffset", 2269, nil},
		{"IDAtOffset", 2270, &Upgrade{Name: "Upgrade1", Cost: 100}},
		{"IDAtOffsetPlus1", 2271, &Upgrade{Name: "Upgrade2", Cost: 200}},
		{"IDAtOffsetPlus2", 2272, &Upgrade{Name: "Upgrade3", Cost: 300}},
		{"IDTooHigh", 2273, nil},
		{"IDWayTooHigh", 3000, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upgrade := upgradeStore.GetObject(tc.id)
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
			nil,
			true,
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
		{Name: "InfantryUnit", Cost: 200}, // The parser picks up BuildCost from Upgrade section
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
