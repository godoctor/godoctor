// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring that converts between explicitly-typed var
// declarations (var n int = 5) and short assignment statements (n := 5).

package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"reflect"
	"strings"

	"golang.org/x/tools/astutil"
	"golang.org/x/tools/go/types"
	"github.com/godoctor/godoctor/text"
)

// A ToggleVar refactoring converts between explicitly-typed variable
// declarations (var n int = 5) and short assignment statements (n := 5).
type ToggleVar struct {
	refactoringBase
}

func (r *ToggleVar) Description() *Description {
	return &Description{
		Name:      "Toggle var <-> :=",
		Synopsis:  "Toggles between a var declaration and := statement",
		Usage:     "",
		Multifile: false,
		Params:    nil,
		Hidden:    false,
	}
}

func (r *ToggleVar) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Error("selection cannot be null")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
		return &r.Result
	}
	_, nodes, _ := r.program.PathEnclosingInterval(r.selectionStart, r.selectionEnd)
	for _, node := range nodes {
		switch selectedNode := node.(type) {
		case *ast.AssignStmt:
			if selectedNode.Tok == token.DEFINE {
				r.short2var(selectedNode)
				r.updateLog(config, true)
			}
			return &r.Result
		case *ast.GenDecl:
			r.var2short(selectedNode)
			r.updateLog(config, true)
			return &r.Result
		}
	}

	r.Log.Errorf("Please select a short assignment (:=) statement or var declaration.\n\nSelected node: %s", reflect.TypeOf(r.selectedNode))
	r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
	return &r.Result
}

func (r *ToggleVar) short2var(assign *ast.AssignStmt) {
	replacement := r.varDeclString(assign)
	r.Edits[r.filename].Add(r.extent(assign), replacement)
	if strings.Contains(replacement, "\n") {
		r.formatFileInEditor()
	}
}

func (r *ToggleVar) rhsExprs(assign *ast.AssignStmt) []string {
	rhsValue := make([]string, len(assign.Rhs))
	for j, rhs := range assign.Rhs {
		offset, length := r.offsetLength(rhs)
		rhsValue[j] = string(r.fileContents[offset : offset+length])
	}
	return rhsValue
}

func (r *ToggleVar) varDeclString(assign *ast.AssignStmt) string {
	var buf bytes.Buffer
	replacement := make([]string, len(assign.Rhs))
	path, _ := astutil.PathEnclosingInterval(r.file, assign.Pos(), assign.End())
	for i, rhs := range assign.Rhs {
		switch T := r.selectedNodePkg.TypeOf(rhs).(type) {
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

func (r *refactoringBase) lhsNames(assign *ast.AssignStmt) []bytes.Buffer {
	var lhsbuf bytes.Buffer
	buf := make([]bytes.Buffer, len(assign.Lhs))
	for i, lhs := range assign.Lhs {
		offset, length := r.offsetLength(lhs)
		lhsText := r.fileContents[offset : offset+length]
		if len(assign.Lhs) == len(assign.Rhs) {
			buf[i].Write(lhsText)
		} else {
			lhsbuf.Write(lhsText)
			if i < len(assign.Lhs)-1 {
				lhsbuf.WriteString(", ")
			}
			buf[0] = lhsbuf
		}
	}
	return buf
}

//calls the edit set
func (r *ToggleVar) var2short(decl *ast.GenDecl) {
	start, _ := r.offsetLength(decl)
	repstrlen := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Values[0].Pos()).Offset - r.program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.filename].Add(&text.Extent{start, repstrlen}, r.shortAssignString(decl))
}

func (r *ToggleVar) varDeclLHS(decl *ast.GenDecl) string {
	offset, _ := r.offsetLength(decl.Specs[0].(*ast.ValueSpec))
	endOffset := r.program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Names[len(decl.Specs[0].(*ast.ValueSpec).Names)-1].End()).Offset
	return string(r.fileContents[offset:endOffset])
}

// returns the shortAssignString string
func (r *ToggleVar) shortAssignString(decl *ast.GenDecl) string {
	return (fmt.Sprintf("%s := ", r.varDeclLHS(decl)))
}
