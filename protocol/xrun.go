package protocol

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

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
	}
	// setup TextSelection
	textselection := input["textselection"].(map[string]interface{})
	ts := doctor.TextSelection{
		Filename:  filepath.Join(state.Dir, textselection["filename"].(string)),
		StartLine: int(textselection["startline"].(float64)),
		StartCol:  int(textselection["startcol"].(float64)),
		EndLine:   int(textselection["endline"].(float64)),
		EndCol:    int(textselection["endcol"].(float64)),
	}

	// get refactoring
	refactoring := doctor.GetRefactoring(input["transformation"].(string))

	config := &doctor.Config{
		FileSystem: state.Filesystem,
		Scope:      nil,
		Selection:  ts,
		Args:       input["arguments"].([]interface{}),
	}

	// run
	result := refactoring.Run(config)

	// grab logs
	logs := make([]map[string]interface{}, 0)
	fatalError := false
	for _, entry := range result.Log.Entries {
		var severity string
		switch entry.Severity {
		case doctor.INFO:
			// No prefix
		case doctor.WARNING:
			severity = "warning"
		case doctor.ERROR:
			severity = "error"
		case doctor.FATAL_ERROR:
			severity = "fatal"
			fatalError = true
		}
		log := map[string]interface{}{"severity": severity, "message": entry.Message}
		logs = append(logs, log)
	}
	// any fatal errors? return without giving changes
	if fatalError {
		return Reply{map[string]interface{}{"reply": "OK", "description": refactoring.Description().Name, "log": logs}}, nil
	}

	changes := make([]map[string]string, 0)

	// if mode == patch or no mode was given
	if mode, found := input["mode"]; !found || mode.(string) == "patch" {
		for f, e := range result.Edits {
			var p *doctor.Patch
			var err error
			p, err = doctor.CreatePatchForFile(e, f)
			if err != nil {
				return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
			}
			diffFile, err := os.Create(strings.Join([]string{f, ".diff"}, ""))
			p.Write(f, f, diffFile)
			//fmt.Println(f)
			//fmt.Println(diffFile.Name())
			changes = append(changes, map[string]string{"filename": f, "patchFile": diffFile.Name()})
			diffFile.Close()
		}
	} else {
		for f, e := range result.Edits {
			content, err := doctor.ApplyToFile(e, f)
			if err != nil {
				return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
			}
			changes = append(changes, map[string]string{"filename": f, "content": string(content)})
		}
	}

	// filesystem changes
	var fschanges []map[string]string
	if len(result.FSChanges) > 0 {
		fschanges = make([]map[string]string, len(result.FSChanges))
		for i, change := range result.FSChanges {
			switch change := change.(type) {
			case *doctor.FSCreateFile:
				fschanges[i] = map[string]string{"change": "create", "file": change.Path, "content": change.Contents}
			case *doctor.FSRemove:
				fschanges[i] = map[string]string{"change": "delete", "path": change.Path}
			case *doctor.FSRename:
				fschanges[i] = map[string]string{"change": "rename", "from": change.Path, "to": change.NewName}
			}
		}
		// return with filesystem changes
		return Reply{map[string]interface{}{"reply": "OK", "description": refactoring.Description().Name, "log": logs, "files": changes, "fsChanges": fschanges}}, nil
	}

	// return without filesystem changes
	return Reply{map[string]interface{}{"reply": "OK", "description": refactoring.Description().Name, "log": logs, "files": changes}}, nil
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
	if limit, found := input["limit"]; found {
		if limit.(int) < 0 {
			return false, errors.New("\"limit\" key must be a positive integer")
		}
	}

	// check mode key if exists
	if mode, found := input["mode"]; found {
		field, _ := reflect.TypeOf(x).Elem().FieldByName("Mode")
		qualityValidator := regexp.MustCompile(field.Tag.Get("chk"))

		if valid := qualityValidator.MatchString(mode.(string)); !valid {
			return false, errors.New("\"mode\" key must be \"text|patch\"")
		}
	}

	// all good?
	return true, nil
}
