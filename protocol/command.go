package protocol

import ()

type Command interface {
	Run(*State, map[string]interface{}) (Reply, error)
	Validate(*State, map[string]interface{}) (bool, error)
}

// a reusable object within other command objects...
type TextSelection struct {
	Filename string
	Offset   int
	Length   int
}
