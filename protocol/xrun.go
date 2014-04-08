package protocol

import (
	"errors"
	"reflect"
	"regexp"

	"golang-refactoring.org/go-doctor/doctor"
)

type XRun struct {
	Transformation string                 `json:"transformation"`
	Fileselection  []string               `json:"fileselection"`
	Textselection  map[string]interface{} `json:"textselection"`
	Arguments      []interface{}          `json:"arguments"`
	Limit          int                    `json:"limit"`
	Mode           string                 `json:"mode" chk:"text|patch"`
}

// TODO implement
func (x *XRun) Run(state *State, input map[string]interface{}) (Reply, error) {
	if valid, err := x.Validate(state, input); !valid {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	} else {
		// do work
	}
	return Reply{map[string]interface{}{"reply": "OK", "message": "we ran"}}, nil
}

// TODO validate TextSelection, FileSelection, arguments
func (x *XRun) Validate(state *State, input map[string]interface{}) (bool, error) {
	if state.State < 2 {
		return false, errors.New("State of 2 (file system configured) is required")
	}

	// check transformation is valid
	var valid bool
	for shortName, _ := range doctor.AllRefactorings() {
		if shortName == input["transformation"].(string) {
			valid = true
		}
	}
	if !valid {
		return false, errors.New("Transformation given is not a valid refactoring name")
	}

	// check limit is > 0 if exists
	if limit, found := input["limit"].(int); found {
		if limit < 0 {
			return false, errors.New("\"limit\" key must be a positive integer")
		}
	}

	// check mode key if exists
	if mode, found := input["mode"].(string); found {
		field, _ := reflect.TypeOf(x).Elem().FieldByName("Mode")
		qualityValidator := regexp.MustCompile(field.Tag.Get("chk"))

		if valid := qualityValidator.MatchString(mode); !valid {
			return false, errors.New("\"mode\" key must be \"text|patch\"")
		}
	}

	// all good?
	return true, nil
}
