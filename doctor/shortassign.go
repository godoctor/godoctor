// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"bytes"
	"code.google.com/p/go.tools/go/types"
	"fmt"
	"go/ast"
	"io"
	"reflect"
)

// A ShortAssignmentRefactoring changes short assignment statements (n := 5)
// into explicitly-typed variable declarations (var n int = 5).
type shortAssignRefactoring struct {
	refactoringBase
}

func (r *shortAssignRefactoring) Description() *Description {
	return &Description{
		Name:   "Short Assignment Refactoring",
		Params: nil,
	}
}

func (r *shortAssignRefactoring) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if len(config.Args) != 0 {
		r.Log.Log(FATAL_ERROR, "This refactoring takes no arguments.")
		return &r.Result
	}

	if r.selectedNode == nil {
		//	r.Log.Log(FATAL_ERROR, "selection cannot be null")
		r.Log.Log(ERROR, "The selection cannot be null.Please select a valid node!")
		return &r.Result
	}

	switch selectedNode := r.selectedNode.(type) {
	case *ast.AssignStmt:
		r.createEditSet(selectedNode)
	default:
		r.Log.Log(FATAL_ERROR, fmt.Sprintf("Select a short assignment (:=) statement! Selected node is %s", reflect.TypeOf(r.selectedNode)))
	}
	r.checkForErrors()
	return &r.Result
}

func (r *shortAssignRefactoring) createEditSet(assign *ast.AssignStmt) {
	start, length := r.offsetLength(assign)
	r.Edits[r.filename(r.file)].Add(OffsetLength{start, length + 1}, r.createReplacementString(assign))
}

func (r *shortAssignRefactoring) rhsExprs(assign *ast.AssignStmt) []string {
	rhsValue := make([]string, len(assign.Rhs))
	for j, rhs := range assign.Rhs {
		rhsValue[j] = r.readFromFile(r.offsetLength(rhs))
	}
	return rhsValue
}

func (r *shortAssignRefactoring) createReplacementString(assign *ast.AssignStmt) string {
	var buf bytes.Buffer
	replacement := make([]string, len(assign.Rhs))
	for i, rhs := range assign.Rhs {
		if T, ok := r.pkgInfo(r.file).TypeOf(rhs).(*types.Tuple); ok {
			replacement[i] = fmt.Sprintf("var %s %s = %s\n",
				r.lhsNames(assign)[i].String(),
				typeOfFunctionType(T),
				r.rhsExprs(assign)[i])
			if typeOfFunctionType(T) == "" {
				//	r.Log.Log(ERROR, "This short assignment cannot be converted to an explicitly-typed var declaration.")
				replacement[i] = fmt.Sprintf("var %s = %s\n",
					r.lhsNames(assign)[i].String(),
					r.rhsExprs(assign)[i])
			}
		} else {
			replacement[i] = fmt.Sprintf("var %s %s = %s\n",
				r.lhsNames(assign)[i].String(),
				r.pkgInfo(r.file).TypeOf(rhs),
				r.rhsExprs(assign)[i])
		}
		io.WriteString(&buf, replacement[i])
	}
	return buf.String()
}

// lhsNames returns the names on the LHS of an assignment, comma-separated.
func (r *shortAssignRefactoring) lhsNames(assign *ast.AssignStmt) []bytes.Buffer {
	var lhsbuf bytes.Buffer
	buf := make([]bytes.Buffer, len(assign.Lhs))
	for i, lhs := range assign.Lhs {
		if len(assign.Lhs) == len(assign.Rhs) {
			buf[i].WriteString(r.readFromFile(r.offsetLength(lhs)))
		} else {
			lhsbuf.WriteString(r.readFromFile(r.offsetLength(lhs)))
			if i < len(assign.Lhs)-1 {
				lhsbuf.WriteString(", ")
			}
			buf[0] = lhsbuf
		}
	}
	return buf
}

// typeOfFunctionType receives a type of function's return type, which must be a
// tuple type; if each component has the same type (T, T, T), then it returns
// the type T as a string; otherwise, it returns the empty string.
func typeOfFunctionType(returnType types.Type) string {
	typeArray := make([]string, returnType.(*types.Tuple).Len())
	initialType := returnType.(*types.Tuple).At(0).Type().String()
	finalType := initialType
	for i := 1; i < returnType.(*types.Tuple).Len(); i++ {
		typeArray[i] = returnType.(*types.Tuple).At(i).Type().String()
		if initialType != typeArray[i] {
			finalType = ""
		}
	}
	return finalType
}
