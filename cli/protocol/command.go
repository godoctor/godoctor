package protocol

type Command interface {
	Run(*State, map[string]interface{}) (Reply, error)
	Validate(*State, map[string]interface{}) (bool, error)
}
