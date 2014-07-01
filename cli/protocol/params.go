// TODO validation of file and textselection fields
package protocol

import (
	"errors"
	"reflect"

	"golang-refactoring.org/go-doctor/refactoring"
	"golang-refactoring.org/go-doctor/text"
)

type Params struct {
	Transformation string         `json:"transformation"`
	Fileselection  []string       `json:"fileselection"`
	Textselection  text.Selection `json:"textselection"`
}

func (p *Params) Run(state *State, input map[string]interface{}) (Reply, error) {
	//refactoring := refactoring.GetRefactoring("rename")
	if valid, err := p.Validate(state, input); valid {
		refactoring := refactoring.GetRefactoring(input["transformation"].(string))
		// since GetParams returns just a string, assume it as prompt and label
		params := make([]map[string]interface{}, 0)
		for _, param := range refactoring.Description().Params {
			params = append(params, map[string]interface{}{"label": param.Label, "prompt": param.Prompt, "type": reflect.TypeOf(param.DefaultValue), "default": param.DefaultValue})
		}
		return Reply{map[string]interface{}{"reply": "OK", "params": params}}, nil
	} else {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
}

func (p *Params) Validate(state *State, input map[string]interface{}) (bool, error) {
	if state.State < 2 {
		return false, errors.New("State of 2 (file system configured) is required")
	}
	if _, found := input["transformation"]; !found {
		return false, errors.New("Transformation key not found")
	}
	return true, nil
}
