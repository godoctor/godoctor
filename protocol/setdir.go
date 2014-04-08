package protocol

import (
	"errors"
	"reflect"
	"regexp"

	"golang-refactoring.org/go-doctor/doctor"
)

type Setdir struct {
	Mode string `json:"mode" chk:"local|web"`
}

func (s *Setdir) Run(state *State, input map[string]interface{}) (Reply, error) {

	if valid, err := s.Validate(state, input); valid {
		// assuming everything is good?
		mode := input["mode"]
		state.Mode = mode.(string)

		// local mode? get directory and local filesystem
		if mode == "local" {
			state.Dir = input["directory"].(string)
			state.Filesystem = doctor.NewLocalFileSystem()
		}

		// web mode? get that virtual filesystem
		if mode == "web" {
			state.Filesystem = doctor.NewVirtualFileSystem()
		}

		state.State = 2
		return Reply{map[string]interface{}{"reply": "OK"}}, nil
	} else {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}

}

func (s *Setdir) Validate(state *State, input map[string]interface{}) (bool, error) {
	if state.State < 1 {
		return false, errors.New("State must be non-zero for \"setdir\" command")
	}

	// mode key?
	if mode, found := input["mode"]; !found {
		err := errors.New("\"mode\" key is required")
		return false, err
	} else {
		// validate the mode value
		field, _ := reflect.TypeOf(s).Elem().FieldByName("Mode")
		modeValidator := regexp.MustCompile(field.Tag.Get("chk"))
		if valid := modeValidator.MatchString(mode.(string)); !valid {
			return false, errors.New("\"mode\" key must be \"web|local\"")
		}
		// check for directory key if mode == local
		if mode == "local" {
			if _, found := input["directory"]; !found {
				return false, errors.New("\"directory\" key required if \"mode\" is local")
			}
			// validate directory
			fs := doctor.NewLocalFileSystem()
			_, err := fs.ReadDir(input["directory"].(string))
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
