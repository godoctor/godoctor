// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Null refactoring, which makes no changes to a program.
// It is for testing only (and can be used as a template for building new
// refactorings).

package doctor

// A nullRefactoring makes no changes to a program.
type nullRefactoring struct {
	refactoringBase
}

func (r *nullRefactoring) Description() *Description {
	return &Description{
		Name: "Null Refactoring",
		Params: []Parameter{Parameter{
			Label:        "Allow Errors",
			Prompt:       "Allow Errors",
			DefaultValue: true,
		}},
		Quality: Development,
	}
}

func (r *nullRefactoring) Run(config *Config) *Result {
	r.refactoringBase.Run(config)

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	if config.Args[0].(bool) {
		r.Log.ChangeInitialErrorsToWarnings()
	}

	if r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	return &r.Result
}
