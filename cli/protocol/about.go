// TODO fix about text

package protocol

import (
	"errors"
)

type About struct {
	aboutText string
}

func (a *About) Run(state *State, input map[string]interface{}) (Reply, error) {
	if valid, err := a.Validate(state, input); valid {
		a.aboutText = "Go Doctor about text"
		return Reply{map[string]interface{}{"reply": "OK", "text": a.aboutText}}, nil
	} else {
		//err := errors.New("The about command requires a state of non-zero")
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
}

func (a *About) Validate(state *State, input map[string]interface{}) (bool, error) {
	if state.State > 0 {
		return true, nil
	} else {
		return false, errors.New("The about command requires a state of non-zero")
	}
}
