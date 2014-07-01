// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"bytes"
	"code.google.com/p/go.tools/astutil"
	"code.google.com/p/go.tools/go/types"
	"fmt"
	"go/ast"
	"io"
	"reflect"
	"strings"
)

// A ShortAssignmentRefactoring changes short assignment statements (n := 5)
// into explicitly-typed variable declarations (var n int = 5).
type shortAssignRefactoring struct {
	refactoringBase
}

func (r *shortAssignRefactoring) Description() *Description {
	return &Description{
		Name:    "Short Assignment Refactoring",
		Params:  nil,
		Quality: Development,
	}
}

func (r *shortAssignRefactoring) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	if r.selectedNode == nil {
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
	r.Edits[r.filename(r.file)].Add(OffsetLength{start, length}, r.createReplacementString(assign))
}

func (r *shortAssignRefactoring) rhsExprs(assign *ast.AssignStmt) []string {
	rhsValue := make([]string, len(assign.Rhs))
	for j, rhs := range assign.Rhs {
		offset, length := r.offsetLength(rhs)
		rhsValue[j] = string(r.fileContents[offset : offset+length])
	}
	return rhsValue
}

func (r *shortAssignRefactoring) createReplacementString(assign *ast.AssignStmt) string {
	var buf bytes.Buffer
	replacement := make([]string, len(assign.Rhs))
	path, _ := astutil.PathEnclosingInterval(r.file, assign.Pos(), assign.End())
	for i, rhs := range assign.Rhs {
		switch T := r.pkgInfo(r.file).TypeOf(rhs).(type) {
		case *types.Tuple: // function type
			if typeOfFunctionType(T) == "" {
				replacement[i] = fmt.Sprintf("var %s = %s\n",
					r.lhsNames(assign)[i].String(),
					r.rhsExprs(assign)[i])
			} else {
				replacement[i] = fmt.Sprintf("var %s %s = %s\n",
					r.lhsNames(assign)[i].String(),
					typeOfFunctionType(T),
					r.rhsExprs(assign)[i])
			}
		case *types.Named: // package and struct types
			if path[len(path)-1].(*ast.File).Name.Name == T.Obj().Pkg().Name() {
				replacement[i] = fmt.Sprintf("var %s %s = %s\n",
					r.lhsNames(assign)[i].String(),
					T.Obj().Name(),
					r.rhsExprs(assign)[i])
			} else {
				replacement[i] = fmt.Sprintf("var %s %s = %s\n",
					r.lhsNames(assign)[i].String(),
					T,
					r.rhsExprs(assign)[i])
			}
		default:
			replacement[i] = fmt.Sprintf("var %s %s = %s\n",
				r.lhsNames(assign)[i].String(),
				T,
				r.rhsExprs(assign)[i])

		}
		io.WriteString(&buf, replacement[i])
	}
	return strings.TrimSuffix(buf.String(), "\n")
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
