// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Null refactoring, which makes no changes to a program.
// It is for testing only (and can be used as a template for building new
// refactorings).

package refactoring

// A Null refactoring makes no changes to a program.
type Null struct {
	base RefactoringBase
}

func (r *Null) Description() *Description {
	return &Description{
		Name:      "Null Refactoring",
		Synopsis:  "Refactoring that makes no changes to a program",
		Usage:     "<allow_errors?>",
		HTMLDoc:   "",
		Multifile: false,
		Params: []Parameter{Parameter{
			Label:        "Allow Errors",
			Prompt:       "Allow Errors",
			DefaultValue: true,
		}},
		Hidden: true,
	}
}

func (r *Null) Run(config *Config) *Result {
	r.base.Run(config)

	if !ValidateArgs(config, r.Description(), r.base.Log) {
		return &r.base.Result
	}

	if config.Args[0].(bool) {
		r.base.Log.ChangeInitialErrorsToWarnings()
	}

	if r.base.Log.ContainsErrors() {
		return &r.base.Result
	}

	r.base.UpdateLog(config, false)
	return &r.base.Result
}
