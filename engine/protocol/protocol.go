// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package protocol provides an implementation of the OpenRefactory protocol
// (server-side), which provides a standard mechanism for text editors to
// communicate with refactoring engines.
package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang-refactoring.org/go-doctor/filesystem"
)

type Reply struct {
	Params map[string]interface{}
}

func (r Reply) String() string {
	replyJson, _ := json.Marshal(r.Params)
	return string(replyJson)
}

type State struct {
	State      int
	Mode       string
	Dir        string
	Filesystem filesystem.FileSystem
}

func Run(args []string) {

	// single command console
	if len(args) == 0 {
		runSingle()
		return
	}
	cmdList := setup()
	// list of commands
	var argJson []map[string]interface{}
	err := json.Unmarshal([]byte(args[0]), &argJson)
	if err != nil {
		printReply(Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
		return
	}
	var state = State{1, "", "", nil}
	for i, cmdObj := range argJson {
		// has command?
		cmd, found := cmdObj["command"]
		if !found { // no command
			printReply(Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			return
		}
		// valid command?
		if _, found := cmdList[cmd.(string)]; found {
			resultReply, err := cmdList[cmd.(string)].Run(&state, cmdObj)
			if err != nil {
				printReply(resultReply)
				return
			}
			// last command?
			if i == len(argJson)-1 {
				printReply(resultReply)
			}
		} else {
			printReply(Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			return
		}
	}

}

func runSingle() {
	cmdList := setup()
	var state = State{0, "", "", nil}
	var inputJson map[string]interface{}
	ioreader := bufio.NewReader(os.Stdin)
	for {
		input, err := ioreader.ReadBytes('\n')
		if err == io.EOF {
			// exit
			break
		} else if err != nil {
			printReply(Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			continue
		}
		err = json.Unmarshal(input, &inputJson)
		if err != nil {
			printReply(Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			continue
		}
		// check command key exists
		cmd, found := inputJson["command"]
		if !found {
			printReply(Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			continue
		}
		// if close command, just exit
		if cmd == "close" {
			break
		}
		// check command is one we support
		if _, found := cmdList[cmd.(string)]; !found {
			printReply(Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			continue
		}
		// everything good to run command
		result, _ := cmdList[cmd.(string)].Run(&state, inputJson) // run the command
		printReply(result)
	}
}

// little helpers
func setup() map[string]Command {
	cmds := make(map[string]Command)
	cmds["about"] = &About{}
	cmds["open"] = &Open{}
	cmds["list"] = &List{}
	cmds["setdir"] = &Setdir{}
	cmds["params"] = &Params{}
	cmds["put"] = &Put{}
	cmds["xrun"] = &XRun{}
	return cmds
}

func printReply(reply Reply) {
	fmt.Printf("%s\n", reply)
}
