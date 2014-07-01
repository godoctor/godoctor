// TODO add in implementation of fileselection and textselection keys
package protocol

import (
	"errors"
	"reflect"
	"regexp"

	"golang-refactoring.org/go-doctor/refactoring"
	"golang-refactoring.org/go-doctor/text"
)

type List struct {
	Fileselection []string           `json:"fileselection"`
	Textselection text.TextSelection `json:"textselection"`
	Quality       string             `json:"quality" chk:"in_testing|in_development|production"`
}

func (l *List) Run(state *State, input map[string]interface{}) (Reply, error) {

	if valid, err := l.Validate(state, input); valid {
		// get all of the refactoring names
		namesList := make([]map[string]string, 0)
		for shortName, refactoring := range refactoring.AllRefactorings() {
			namesList = append(namesList, map[string]string{"shortName": shortName, "name": refactoring.Description().Name})
		}
		return Reply{map[string]interface{}{"reply": "OK", "transformations": namesList}}, nil
	} else {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
}

func (l *List) Validate(state *State, input map[string]interface{}) (bool, error) {
	if state.State < 1 {
		err := errors.New("The about command requires a state of non-zero")
		return false, err
	}
	// check for required keys
	if _, found := input["quality"]; !found {
		err := errors.New("Quality key not found")
		return false, err
	} else {
		// check quality matches
		field, _ := reflect.TypeOf(l).Elem().FieldByName("Quality")
		qualityValidator := regexp.MustCompile(field.Tag.Get("chk"))

		if valid := qualityValidator.MatchString(input["quality"].(string)); !valid {
			return false, errors.New("Quality key must be \"in_testing|in_development|production\"")
		}
	}
	return true, nil
}
