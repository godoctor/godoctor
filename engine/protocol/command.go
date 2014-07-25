// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"golang-refactoring.org/go-doctor/engine"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/refactoring"
	"golang-refactoring.org/go-doctor/text"
)

type Command interface {
	Run(*State, map[string]interface{}) (Reply, error)
	Validate(*State, map[string]interface{}) (bool, error)
}

// -=-= About =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

// TODO fix about text

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

// -=-= List =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// TODO add in implementation of fileselection and textselection keys

type List struct {
	Fileselection []string               `json:"fileselection"`
	Textselection map[string]interface{} `json:"textselection"`
	Quality       string                 `json:"quality" chk:"in_testing|in_development|production"`
}

func (l *List) Run(state *State, input map[string]interface{}) (Reply, error) {
	if valid, err := l.Validate(state, input); valid {
		minQuality := refactoring.Development
		switch input["quality"].(string) {
		case "in_testing":
			minQuality = refactoring.Testing
		case "production":
			minQuality = refactoring.Production
		}

		// get all of the refactoring names
		namesList := make([]map[string]string, 0)
		for _, shortName := range engine.AllRefactoringNames() {
			refactoring := engine.GetRefactoring(shortName)
			if refactoring.Description().Quality >= minQuality {
				namesList = append(namesList, map[string]string{"shortName": shortName, "name": refactoring.Description().Name})
			}
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

	// validate text/file selection
	// TODO validate fileselection
	textselection, tsfound := input["textselection"]
	_, fsfound := input["fileselection"]

	if tsfound && fsfound {
		return false, errors.New("Both textseleciton and fileselection cannot be used together")
	} else if tsfound {
		if state.State < 2 {
			return false, errors.New("File system not yet configured, cannot use textselection")
		}
		_, err := parseSelection(state, textselection.(map[string]interface{}))
		if err != nil {
			return false, err
		}
	} else if fsfound {
		if state.State < 2 {
			return false, errors.New("File system not yet configured, cannot use fileselection")
		}
	}
	return true, nil
}

// -=-= Open =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// TODO open with version

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

// -=-= Params =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

type Params struct {
	Transformation string                 `json:"transformation"`
	Fileselection  []string               `json:"fileselection"`
	Textselection  map[string]interface{} `json:"textselection"`
}

func (p *Params) Run(state *State, input map[string]interface{}) (Reply, error) {
	//refactoring := engine.GetRefactoring("rename")
	if valid, err := p.Validate(state, input); valid {
		refactoring := engine.GetRefactoring(input["transformation"].(string))
		// since GetParams returns just a string, assume it as prompt and label
		params := make([]map[string]interface{}, 0)
		for _, param := range refactoring.Description().Params {
			params = append(params, map[string]interface{}{"label": param.Label, "prompt": param.Prompt, "type": reflect.TypeOf(param.DefaultValue).String(), "default": param.DefaultValue})
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
	// validate text/file selection
	// TODO validate fileselection
	textselection, tsfound := input["textselection"]
	_, fsfound := input["fileselection"]

	if tsfound && fsfound {
		return false, errors.New("Both textseleciton and fileselection cannot be used together")
	} else if tsfound {
		_, err := parseSelection(state, textselection.(map[string]interface{}))
		if err != nil {
			return false, err
		}
	} else if fsfound {

	}
	return true, nil
}

// -=-= Put -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

type Put struct { // FIXME: Robert -- make sure this is right
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (p *Put) Run(state *State, input map[string]interface{}) (Reply, error) {
	if valid, err := p.Validate(state, input); !valid {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}

	var editedFS *filesystem.EditedFileSystem
	var ok bool
	if editedFS, ok = state.Filesystem.(*filesystem.EditedFileSystem); !ok {
		return Reply{map[string]interface{}{"reply": "Error",
				"message": "put can only be executed in Web mode"}},
			nil // FIXME: Robert -- OK to return nil here?
	}

	if input["filename"] != filesystem.FakeStdinFilename {
		return Reply{map[string]interface{}{"reply": "Error", "message": fmt.Sprintf("put filename must be \"%s\"", filesystem.FakeStdinFilename)}},
			nil // FIXME: Robert -- OK to return nil here?
	}

	stdinPath, err := filesystem.FakeStdinPath()
	if err != nil {
		return Reply{map[string]interface{}{"reply": "Error",
			"message": err.Error()}}, err
	}

	es := text.NewEditSet()
	es.Add(text.Extent{0, 0}, input["content"].(string))
	editedFS.Edits[stdinPath] = es
	return Reply{map[string]interface{}{"reply": "OK"}}, nil
}

func (p *Put) Validate(state *State, input map[string]interface{}) (bool, error) {
	//FIXME: Robert: implement Validate please
	return true, nil
}

// -=-= Setdir =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

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
			state.Filesystem = filesystem.NewLocalFileSystem()
		}

		// web mode? use edited filesystem
		if mode == "web" {
			state.Dir = "."
			state.Filesystem = filesystem.NewEditedFileSystem(
				filesystem.NewLocalFileSystem(),
				map[string]*text.EditSet{})
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
			fs := filesystem.NewLocalFileSystem()
			_, err := fs.ReadDir(input["directory"].(string))
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// -=-= XRun =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

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
	// setup text selection
	textselection := input["textselection"].(map[string]interface{})

	ts, _ := parseSelection(state, textselection)

	if ts.GetFilename() != filesystem.FakeStdinFilename {
		return Reply{map[string]interface{}{"reply": "Error", "message": fmt.Sprintf("put filename must be \"%s\"", filesystem.FakeStdinFilename)}},
			nil // FIXME: Robert -- OK to return nil here?
	}
	stdinPath, err := filesystem.FakeStdinPath()
	if err != nil {
		return Reply{map[string]interface{}{"reply": "Error",
			"message": err.Error()}}, err
	}
	switch ts := ts.(type) {
	case *text.OffsetLengthSelection:
		ts.Filename = stdinPath
	case *text.LineColSelection:
		ts.Filename = stdinPath
	}

	// get refactoring
	refac := engine.GetRefactoring(input["transformation"].(string))

	config := &refactoring.Config{
		FileSystem: state.Filesystem,
		Scope:      nil,
		Selection:  ts,
		Args:       input["arguments"].([]interface{}),
	}

	// run
	result := refac.Run(config)

	// grab logs
	logs := make([]map[string]interface{}, 0)
	for _, entry := range result.Log.Entries {
		var severity string
		switch entry.Severity {
		case refactoring.Info:
			// No prefix
		case refactoring.Warning:
			severity = "warning"
		case refactoring.Error:
			severity = "error"
		}
		log := map[string]interface{}{"severity": severity, "message": entry.Message}
		logs = append(logs, log)
	}

	changes := make([]map[string]string, 0)

	// if mode == patch or no mode was given
	if mode, found := input["mode"]; !found || mode.(string) == "patch" {
		for f, e := range result.Edits {
			var p *text.Patch
			var err error
			p, err = filesystem.CreatePatch(e, state.Filesystem, f)
			if err != nil {
				return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
			}
			diffFile, err := os.Create(strings.Join([]string{f, ".diff"}, ""))
			p.Write(f, f, time.Time{}, time.Time{}, diffFile)
			//fmt.Println(f)
			//fmt.Println(diffFile.Name())
			changes = append(changes, map[string]string{"filename": f, "patchFile": diffFile.Name()})
			diffFile.Close()
		}
	} else {
		for f, e := range result.Edits {
			content, err := filesystem.ApplyEdits(e, state.Filesystem, f)
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
			case *filesystem.CreateFile:
				fschanges[i] = map[string]string{"change": "create", "file": change.Path, "content": change.Contents}
			case *filesystem.Remove:
				fschanges[i] = map[string]string{"change": "delete", "path": change.Path}
			case *filesystem.Rename:
				fschanges[i] = map[string]string{"change": "rename", "from": change.Path, "to": change.NewName}
			}
		}
		// return with filesystem changes
		return Reply{map[string]interface{}{"reply": "OK", "description": refac.Description().Name, "log": logs, "files": changes, "fsChanges": fschanges}}, nil
	}

	// return without filesystem changes
	return Reply{map[string]interface{}{"reply": "OK", "description": refac.Description().Name, "log": logs, "files": changes}}, nil
}

// TODO validate TextSelection, FileSelection, arguments
func (x *XRun) Validate(state *State, input map[string]interface{}) (bool, error) {
	if state.State < 2 {
		return false, errors.New("State of 2 (file system configured) is required")
	}

	// check transformation is valid
	if engine.GetRefactoring(input["transformation"].(string)) == nil {
		return false, errors.New("Transformation given is not a valid refactoring name")
	}

	// validate text/file selection
	// TODO validate fileselection
	textselection, tsfound := input["textselection"]
	_, fsfound := input["fileselection"]

	if tsfound && fsfound {
		return false, errors.New("Both textseleciton and fileselection cannot be used together")
	} else if tsfound {
		_, err := parseSelection(state, textselection.(map[string]interface{}))
		if err != nil {
			return false, err
		}
	} else if fsfound {

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

// -=-= Helpers =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// takes a map for a text selection, either in line/col form or offset/length
// and returns the appropriate type (LineColSelection or OffsetLengthSelection)
// also can be used to simply validate the text selection given
func parseSelection(state *State, input map[string]interface{}) (text.Selection, error) {
	// validate filename
	filename, filefound := input["filename"]
	if !filefound {
		return nil, fmt.Errorf("File is not given")
	}
	if reflect.TypeOf(input["filename"]).Kind() != reflect.String {
		return nil, fmt.Errorf("Invalid type of value given for file: given %T", reflect.TypeOf(input["filename"]))
	}
	file := filepath.Join(state.Dir, filename.(string))

	// determine if offset/length or line/col
	offset, offsetFound := input["offset"]
	length, lengthFound := input["length"]

	if !offsetFound || !lengthFound {
		return nil, fmt.Errorf("invalid offset/length combo: value(s) missing")
	} else {
		// validate
		if reflect.TypeOf(offset).Kind() != reflect.Float64 ||
			reflect.TypeOf(length).Kind() != reflect.Float64 {
			return nil, fmt.Errorf("Invalid type(s) given for offset/length combo (%v, %v)", reflect.TypeOf(offset), reflect.TypeOf(length))
		}

		pos := fmt.Sprintf("%d,%d", int(offset.(float64)), int(length.(float64)))
		ts, err := text.NewSelection(file, pos)
		if err != nil {
			return nil, err
		}
		return ts, nil
	}

	sl, slfound := input["startline"]
	sc, scfound := input["startcol"]
	el, elfound := input["endline"]
	ec, ecfound := input["endcol"]

	if !slfound || !scfound || !elfound || !ecfound {
		return nil, fmt.Errorf("invalid line/col combo: value(s) missing")
	} else {
		// validate
		if reflect.TypeOf(sl).Kind() != reflect.Int || reflect.TypeOf(sc).Kind() != reflect.Int || reflect.TypeOf(el).Kind() != reflect.Int || reflect.TypeOf(ec).Kind() != reflect.Int {
			return nil, fmt.Errorf("invalid type(s) given for line/col combo")
		}
		pos := fmt.Sprintf("%d,%d:%d,%d", int(sl.(float64)), int(sc.(float64)), int(el.(float64)), int(ec.(float64)))
		ts, err := text.NewSelection(file, pos)
		if err != nil {
			return nil, err
		}
		return ts, nil
	}

}
