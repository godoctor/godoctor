// TODO open with version
package protocol

type Open struct {
	Version float64 `json:"version"`
}

func (o *Open) Run(state *State, input map[string]interface{}) (Reply, error) {
	state.State = 1
	//printReply(Reply{"OK", ""})
	return Reply{map[string]interface{}{"reply": "OK"}}, nil
}

// basically useless until we implement versioning...
func (o *Open) Validate(state *State, input map[string]interface{}) (bool, error) {
	return true, nil
}
