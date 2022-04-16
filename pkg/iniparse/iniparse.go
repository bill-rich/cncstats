package iniparse

import (
	"bufio"
	"fmt"
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

func (o *ObjectStore) LoadObjects(dir string) error {
	dirItems, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, dirItem := range dirItems {
		file, err := os.Open(dir + "/" + dirItem.Name())
		if err != nil {
			return err
		}
		o.ParseFile(file)
	}
	return nil
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

func NewObjectStore() *ObjectStore {
	return &ObjectStore{
		Object: []Object{},
	}
}

func (o *ObjectStore) GetObject(i int) *Object {
	return &o.Object[i-2]
}

func matchKey(line string) string {
	for _, key := range IniKey {
		if strings.HasPrefix(line, key) {
			return strings.ReplaceAll(key, " ", "")
		}
	}
	return nilString
}

func (o *ObjectStore) ParseFile(file *os.File) error {
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
