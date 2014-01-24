// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Null refactoring, which makes no changes to a program.
// It is for testing only (and can be used as a template for building new
// refactorings).

package doctor

// A NullRefactoring makes no changes to a program.
type NullRefactoring struct {
	RefactoringBase
	optShowAST        bool
	optShowPackages   bool
	optShowReferences bool
}

func (r *NullRefactoring) Name() string {
	return "Null Refactoring"
}

func (r *NullRefactoring) GetParams() []string {
	return []string{}
}

func (r *NullRefactoring) Configure(args []string) bool {
	return len(args) == 0
}

func (r *NullRefactoring) Run() {
	r.log.ChangeInitialErrorsToWarnings()
}
