// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"code.google.com/p/go.tools/go/types"
	"fmt"
	"go/ast"
	"os"
	"reflect"
)

// A ShortAssignmentRefactoring changes short assignment statements (n := 5)
// into explicitly-typed variable declarations (var n int = 5).
type ShortAssignRefactoring struct {
	RefactoringBase
}

func (r *ShortAssignRefactoring) Name() string {
	return "Short Assignment Refactoring"
}

func (r *ShortAssignRefactoring) GetParams() []string {
	return []string{}
}

func (r *ShortAssignRefactoring) Configure(args []string) bool {
	return true
}

func (r *ShortAssignRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}

	switch selectedNode := r.selectedNode.(type) {
	case *ast.AssignStmt:
		r.createEditSet(selectedNode)
		return
	default:
		r.log.Log(FATAL_ERROR, fmt.Sprintf("Select a short assignment (:=) statement! Selected node is %s", reflect.TypeOf(r.selectedNode)))
		return
	}

	r.checkForErrors()
	return
}

func (r *ShortAssignRefactoring) lhsNames(selectedNode *ast.AssignStmt) []string {
	lhsName := make([]string, len(selectedNode.Lhs))
	for i, lhs := range selectedNode.Lhs {
		lhsName[i] = lhs.(*ast.Ident).Name
	}
	return lhsName
}

func (r *ShortAssignRefactoring) rhsExprs(selectedNode *ast.AssignStmt) []string {
	rhsValue := make([]string, len(selectedNode.Rhs))
	for j, rhs := range selectedNode.Rhs {
		startOffset, length := r.offsetLength(rhs)
		rhsValue[j] = r.readFromFile(length, startOffset)
	}
	return rhsValue
}

func (r *ShortAssignRefactoring) createEditSet(selectedNode *ast.AssignStmt) {
	startOffset, length := r.offsetLength(selectedNode)
	replacementString := r.createReplacementString(selectedNode)
	r.editSet[r.filename(r.file)].Add(OffsetLength{startOffset, length + 1}, replacementString)
}

func (r *ShortAssignRefactoring) offsetLength(node ast.Node) (int, int) {
	startOffset := r.program.Fset.Position(node.Pos()).Offset
	endOffset := r.program.Fset.Position(node.End()).Offset
	return startOffset, (endOffset - startOffset)
}

func (r *ShortAssignRefactoring) createReplacementString(selectedNode *ast.AssignStmt) string {
	var replacementString string

	replacement := make([]string, len(selectedNode.Rhs))
	for index, rhs := range selectedNode.Rhs {
		if reflect.TypeOf(r.pkgInfo(r.file).TypeOf(rhs)).String() == "*types.Tuple" {
			replacement[index] = fmt.Sprintf("var %s %s = %s\n",
				r.lhsNamesCommaSeparated(selectedNode),
				r.returnTypeOfFunction(r.pkgInfo(r.file).TypeOf(rhs)),
				r.rhsExprs(selectedNode)[index])
			if r.returnTypeOfFunction(r.pkgInfo(r.file).TypeOf(rhs)) == "" {
				r.log.Log(ERROR, "This short assignment cannot be converted to an explicitly-typed var declaration.")
			}
		} else {
			replacement[index] = fmt.Sprintf("var %s %s = %s\n",
				r.lhsNames(selectedNode)[index],
				r.pkgInfo(r.file).TypeOf(rhs).String(),
				r.rhsExprs(selectedNode)[index])
			// include the space in front of the var for the second element you add
		}
		replacementString += replacement[index]
	}
	return replacementString
}

// lhsNamesCommaSeparated receives an assignment statement and returns a string
// consisting of the variable(s) on the left-hand side separated by commas.
// For example, given the assignment statement "i, j, k := f()", it returns the
// string "i,j,k".
func (r *ShortAssignRefactoring) lhsNamesCommaSeparated(selectedNode *ast.AssignStmt) string {
	var lhsValue string
	for j, lhs := range selectedNode.Lhs {
		startOffset, length := r.offsetLength(lhs)
		lhsValue += r.readFromFile(length, startOffset)
		if j < len(selectedNode.Lhs)-1 {
			lhsValue = lhsValue + ","
		}
	}
	return lhsValue
}

func (r *ShortAssignRefactoring) readFromFile(len, offset int) string {
	buf := make([]byte, len)
	file, err := os.Open(r.filename(r.file))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	_, err = file.ReadAt(buf, int64(offset))
	if err != nil {
		panic(err)
	}
	return string(buf)
}

// returnTypeOfFunction receives a function's return type, which must be a
// tuple type; if each component has the same type (T, T, T), then it returns
// the type T as a string; otherwise, it returns the empty string.
func (r *ShortAssignRefactoring) returnTypeOfFunction(returnType types.Type) string {
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
