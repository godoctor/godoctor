// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file will extract the local variables from
// statements and put them into a variable to replae
// them in the statements.

package refactoring

import (
	"fmt"
	"go/ast"
	"regexp"

	"github.com/godoctor/godoctor/text"

	"golang.org/x/tools/astutil"
)

type ExtractLocal struct {
	refactoringBase
	varName string
}

func (r *ExtractLocal) Description() *Description {
	return &Description{
		Name:      "Extract Local Variable Refactoring",
		Synopsis:  "Extract a selection to a new variable",
		Usage:     "<new_name>",
		Multifile: false,
		Params: []Parameter{
			// args[0] which is the string that will replace the selected text
			Parameter{
				Label:        "newVar name: ",
				Prompt:       "Please select name for the new Variable.",
				DefaultValue: "",
			}},
		Hidden: false,
	}
}

// this run function will run the program
func (r *ExtractLocal) Run(config *Config) *Result {
	r.refactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	// get first variable
	// check if there was a selected node from refactoring
	if r.selectedNode == nil {
		r.Log.Error("Please select an expression to extract.")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
		return &r.Result
	}
	// get second variable to choose whether single extraction
	// or multiple extraction
	r.sChoice(config)
	r.selectionCheck()
	// get third variable that will be the
	// new variable to replace the expression
	r.varName = config.Args[0].(string)
	r.singleExtract()
	r.formatFileInEditor()
	r.updateLog(config, false)
	return &r.Result
}

// sChoice gets input for second variable
func (r *ExtractLocal) sChoice(config *Config) *Result {
	r.varName = config.Args[0].(string)
	if r.varName == "" {
		r.Log.Error("You must enter a name for the new variable.")
		return &r.Result
	}
	return &r.Result
}

// selectionCheck checks if the selection is empty or not
func (r *ExtractLocal) selectionCheck() *Result {
	if r.selectedNode == nil {
		r.Log.Error("Please select an identifier to extract.")
		r.Log.AssociatePos(r.selectionStart, r.selectionEnd)
		return &r.Result
	}
	return &r.Result
}

// singleExtract locates the position of the selection and will
// replace it with a new variable of the user's chosing
func (r *ExtractLocal) singleExtract() {
	var switchList, typeSwitchFound []ast.Stmt
	var plists, rlists, recLists []*ast.Field
	var lhs, rhs []ast.Expr
	var parentNode, switchNode, fini, fcond, fpost, ifInitAssign, ifElse, condition, assign ast.Node
	found, isBranchStmt, isLabelStmt := false, false, false
	errorType := ""
	ast.Inspect(r.file, func(n ast.Node) bool {
		switch selectedNode := n.(type) {
		case *ast.FuncDecl:
			if selectedNode.Type.Params != nil {
				plists = selectedNode.Type.Params.List
			}
			if selectedNode.Type.Results != nil {
				rlists = selectedNode.Type.Results.List
			}
			if selectedNode.Recv != nil {
				recLists = selectedNode.Recv.List
			}
		case ast.Stmt:
			parentNode = selectedNode
			if sNode, ok := selectedNode.(*ast.SwitchStmt); ok {
				switchNode = sNode
				switchList = sNode.Body.List
			} else if fNode, ok := selectedNode.(*ast.ForStmt); ok {
				fini = fNode.Init
				fcond = fNode.Cond
				fpost = fNode.Post
			} else if ifNode, ok := selectedNode.(*ast.IfStmt); ok {
				if assignNode, ok := ifNode.Init.(*ast.AssignStmt); ok {
					ifInitAssign = assignNode
				} else if ifNode.Else != nil {
					ifElse = ifNode.Else
				}
			} else if typeSwitch, ok := selectedNode.(*ast.TypeSwitchStmt); ok {
				body := typeSwitch.Body.List
				if len(body) != 0 {
					typeSwitchFound = body
				}
				if typeSwitch.Init != nil {
					condition = typeSwitch.Init
				} else if typeSwitch.Assign != nil {
					assign = typeSwitch.Assign
				}
			} else if assignment, ok := selectedNode.(*ast.AssignStmt); ok {
				lhs, rhs = assignment.Lhs, assignment.Rhs
			} else if block, ok := selectedNode.(*ast.BlockStmt); ok {
				if block == r.selectedNode {
					errorType = "blockSelected"
					found = true
				}
			} else if branch, ok := selectedNode.(*ast.BranchStmt); ok {
				if branch == r.selectedNode || branch.Label == r.selectedNode || r.posLine(branch) == r.posLine(r.selectedNode) {
					isBranchStmt = true
				}
			} else if label, ok := selectedNode.(*ast.LabeledStmt); ok {
				if label == r.selectedNode {
					isLabelStmt = true
				}
			}
		case ast.Expr:
			multiAssignVar := r.multiAssignVarCheck(lhs, rhs)
			isVarStmt := r.varStmtCheck()
			isLhsAssignVar := r.lhsAssignVarCheck()
			isCallExpr := r.callCheck(selectedNode)
			isSelectorType := r.selectorCheck(selectedNode)
			isNil := r.checkNil(selectedNode)
			isInsideCompositLit := r.checkInsideCompsite(selectedNode)
			isKeyValueExpr := r.keyValueCheck()
			//isCompositeLit := r.checkComposite()
			isObjInBinary := r.checkBinaryObj(selectedNode) // need to figure out why it is needed for random case 352
			//isObjInBinary = false
			isIdentInAssign := r.checkAssignIdents()
			funcInput := r.funcInputCheck(plists, rlists, recLists)
			// might not need this one, will have to wait till after
			isSwitchCaseSelector := r.caseSelectorCheck(switchList)
			if multiAssignVar { // check if selected node is from part a _, _ := _, _ stmt
				errorType = "mulitAssign"
				found = true
			} else if r.isPreDeclaredIdent() {
				errorType = "preIdent"
				found = true
			} else if isVarStmt { // checks if its a var _ _ stmt
				errorType = "assignStmt"
				found = true
			} else if isLhsAssignVar { // checks if its from the lhs of assign stmt
				errorType = "lhsAssign"
				found = true
			} else if isCallExpr { // checks if it's in call expr
				errorType = "callExpr"
				found = true
			} else if funcInput { // check if selected node is from the function definition
				errorType = "funcInput"
				found = true
			} else if isSelectorType { // check for when a selector type
				errorType = "selectorType"
				found = true
			} else if isSwitchCaseSelector { // checks if its from the switch type (ie switch node, checks if its node)
				errorType = "switchKey"
				found = true
			} else if isNil { // checks if it's nil
				errorType = "nil"
				found = true
			} else if isKeyValueExpr { // check to see if from a key:value
				errorType = "keyValue"
				found = true
			} else if isObjInBinary {
				errorType = "objInBinary"
				found = true
			} else if isInsideCompositLit {
				newVar := r.createVar(r.selectedNode)
				found = r.createParent(newVar)
			} else if isBranchStmt {
				errorType = "branchStmt"
				found = true
			} else if isLabelStmt {
				errorType = "labelStmt"
				found = true
			} else if isIdentInAssign {
				newVar := r.createVar(r.selectedNode)
				found = r.createParent(newVar)
			} else if selectedNode == r.selectedNode {
				typeCase := r.typeSwitchCheck(typeSwitchFound, condition, assign)
				isIfMult := r.ifMultLeftCheck(ifInitAssign, selectedNode)
				isSwitchCase := r.switchCheck(switchList, selectedNode)
				isForCond := r.forCheck(fini, fcond, fpost, selectedNode)
				line1 := r.endLine(selectedNode)
				line2 := r.endLine(r.selectedNode)
				//isLeftBinary := r.checkBinary(selectedNode)
				isIndexExpr := r.checkIndexExpr(selectedNode)
				//isStarExpr := r.starCheck(selectedNode)
				if isSwitchCase { // check for switch stmts
					newVar := r.createVar(selectedNode)
					found = r.addForSwitch(switchNode, selectedNode, newVar)
				} else if typeCase { // check if selected node is from part of the type switch
					errorType = "typeSwitchCase"
					found = true
				} else if isForCond { // check if selected node is from the for loop definition
					errorType = "for"
					found = true
				} else if isIfMult { // check for for loops or if mutli assign
					found = true
					errorType = "ifMulti"
				} else if isIndexExpr {
					errorType = "indexExpr"
					found = true
				} else if line1 == line2 || ifElse != nil {
					newVar := r.createVar(selectedNode)
					found = r.createParent(newVar)
				} /* else if isCompositeLit {
					errorType = "blockSelected"
					found = true
				}  else if isStarExpr { // checks if it's a star expr
					errorType = "starExpr"
					found = true
				} else if isLeftBinary { // check to see if on the lhs of a =, ==, or :=
					errorType = "lhsAssign"
					found = true
				}*/
			}
		default:
		}
		// used to end the inspect func once an extraction is done
		if found == false {
			return true
		}
		return false

	})
	r.errorCheck(errorType)
}
func (r *ExtractLocal) posOff(node ast.Node) int {
	return r.program.Fset.Position(node.Pos()).Offset
}
func (r *ExtractLocal) endOff(node ast.Node) int {
	return r.program.Fset.Position(node.End()).Offset
}
func (r *ExtractLocal) endLine(node ast.Node) int {
	return r.program.Fset.Position(node.End()).Line
}
func (r *ExtractLocal) posLine(node ast.Node) int {
	return r.program.Fset.Position(node.Pos()).Line
}

func (r *ExtractLocal) posColumn(node ast.Node) int {
	return r.program.Fset.Position(node.Pos()).Column
}

/*// getIndexOfParentIf gets the index of the first if statement
// parent node to put the assignment before when r.selectedNode
// is inside an if statement
func (r *ExtractLocal) getIndexOfParentIf(enclosing []ast.Node) int {
	firstIfPar := 0
	for _, index := range enclosing {
		if firstIfPar == 0 {
			if _, ok := index.(*ast.IfStmt); ok {
				firstIfPar++
			}
		} else {
			if _, ok := index.(*ast.IfStmt); ok {
				//	if ifPar.Else != nil {
				firstIfPar++
				//}
			} else {
				break
			}
		}
	}
	return firstIfPar
} */

// switchCheck switch case check for switch version of extraction
func (r *ExtractLocal) switchCheck(switchList []ast.Stmt, selectedNode ast.Node) bool {
	for index, _ := range switchList {
		line1 := r.posLine(switchList[index])
		line2 := r.posLine(selectedNode)
		if line1 == line2 {
			return true
		}
	}
	return false
}

// forCheck for loop init/cond/post test
func (r *ExtractLocal) forCheck(fini ast.Node, fcond ast.Node, fpost ast.Node, selectedNode ast.Node) bool {
	off1 := r.posLine(r.selectedNode)
	//fmt.Println("test goes here")
	//r.Log.Infof("\ntest goes here\n")
	//fmt.Println("\ntest goes here\n")
	enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
	for _, index2 := range enclosing {
		if forparent, ok := index2.(*ast.ForStmt); ok {
			if r.posLine(forparent) == off1 {
				//fmt.Println("test goes here 2")

				return true
			}
		}
	}

	if fini != nil {
		off2 := r.posLine(fini)
		isInitExpr := off2 == off1
		if isInitExpr {
			fmt.Println("test goes here3")

			return true
		}
	}
	if fcond != nil {
		off3 := r.posLine(fcond)
		isCondExpr := off3 == off1
		if isCondExpr {
			fmt.Println("test goes here4")

			return true
		}
	}
	if fpost != nil {
		off4 := r.posLine(fpost)
		isPostExpr := off4 == off1
		if isPostExpr {
			fmt.Println("test goes here5")

			return true
		}
	}
	if fini == r.selectedNode || fcond == r.selectedNode || fpost == r.selectedNode {
		fmt.Println("test goes here6")

		return true
	}
	return false
}

// ifMultLeftCheck if stmt check for _, _ conditions on the left
func (r *ExtractLocal) ifMultLeftCheck(ifInitAssign ast.Node, selectedNode ast.Node) bool {
	if ifInitAssign != nil {
		line1 := r.posLine(ifInitAssign)
		line2 := r.posLine(selectedNode)
		if line1 == line2 {
			return true
		}
	}
	return false
}

// typeSwitchCheck sees if the switch is a type switch and if so returns true to produce an error
func (r *ExtractLocal) typeSwitchCheck(typeSwitchFound []ast.Stmt, condition ast.Node, assign ast.Node) bool {
	if len(typeSwitchFound) != 0 {
		for _, index := range typeSwitchFound {
			if caseClauses, ok := index.(*ast.CaseClause); ok {
				for _, index2 := range caseClauses.List {
					if index2 == r.selectedNode {
						return true
					}
					enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
					for _, index3 := range enclosing {
						if index3 == index2 {
							return true
						}
					}
				}
			}
		}
	}
	if condition == r.selectedNode {
		return true
	} else if assign != nil {
		if aStmt, ok := assign.(*ast.AssignStmt); ok {
			if aStmt.Lhs != nil && aStmt.Lhs[0] == r.selectedNode {
				return true
			} else if aStmt.Rhs != nil && aStmt.Rhs[0] == r.selectedNode {
				return true
			}
		}
	}
	return false
}

// funcInputCheck checks to see if selected node is a function parameter or result
// in the definition and returns true if it is so as to produce an error
func (r *ExtractLocal) funcInputCheck(plists []*ast.Field, rlists []*ast.Field, recLists []*ast.Field) bool {
	if len(plists) != 0 {
		for _, index := range plists {
			for _, name := range index.Names {
				if name == r.selectedNode {
					return true
				}
			}
		}
	}
	if len(rlists) != 0 {
		for _, index := range rlists {
			if r.posLine(index) == r.posLine(r.selectedNode) {
				return true
			}
		}
	}
	if len(recLists) != 0 {
		for _, index := range recLists {
			if r.posLine(index) == r.posLine(r.selectedNode) {
				return true
			}
		}
	}

	return false
}

// multiAssignVarCheck checks if it's a multi assign stmt ie  _, _ := _, _
// and returns true if the selected node is on either side
func (r *ExtractLocal) multiAssignVarCheck(lhs []ast.Expr, rhs []ast.Expr) bool {
	if len(lhs) > 1 && r.posLine(lhs[0]) == r.posLine(r.selectedNode) {
		return true
	}
	return false
}

/*// ifFmtWBinary checks if the if stmt has mulitple conditions/inital conditions
// ie if x := a.Type(); x != y {
// TODO: might not need, check all 400 tests to see
func (r *ExtractLocal) ifFmtWBinary(parentNode ast.Node, childNode ast.Node, selectedNode ast.Node) bool {
	if parentNode != nil && childNode != nil {
		line1 := r.posLine(childNode)
		line2 := r.posLine(selectedNode)
		if line1 == line2 {
			return true
		}
	}
	return false
} */

// keyValueCheck checks if is a key value expr or a child of
// a key value expr, so anything to do maps, or keys : values
func (r *ExtractLocal) keyValueCheck() bool {
	if _, ok := r.selectedNode.(*ast.KeyValueExpr); ok {
		return true
	}
	return false
}

// varStmtCheck check if it's a declartion stmt, and return true
// if it is ie var apple int or var handler http.Handler
func (r *ExtractLocal) varStmtCheck() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
	for _, index := range enclosing {
		if _, ok := index.(*ast.DeclStmt); ok {
			return true
		}
	}
	return false
}

// lhsAssignVarCheck checks if r.selectedNode is on the lhs of an
// assign statement
func (r *ExtractLocal) lhsAssignVarCheck() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
	for _, index := range enclosing {
		if assign, ok := index.(*ast.AssignStmt); ok {
			for _, index2 := range assign.Lhs {
				if index2 == r.selectedNode {
					return true
				}
				if _, ok := index2.(*ast.SelectorExpr); ok {
					return true
				}
			}
		}
	}
	return false
}

// callCheck check for call expr and don't allow if its just
// a call expr itself, or if part of the callexpr (although
// inside the () of it should work)
func (r *ExtractLocal) callCheck(selectedNode ast.Node) bool {
	if call, ok := selectedNode.(*ast.CallExpr); ok {
		if call == r.selectedNode {
			return true
		} else if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
			if fun == r.selectedNode || fun.X == r.selectedNode || fun.Sel == r.selectedNode {
				return true
			}
		} else if len(call.Args) != 0 {
			for _, index := range call.Args {
				if selector, ok := index.(*ast.SelectorExpr); ok {
					if selector == r.selectedNode || selector.X == r.selectedNode || selector.Sel == r.selectedNode {
						return true
					}
				} else if ident, ok := index.(*ast.Ident); ok {
					if ident == r.selectedNode {
						return false
					}
				}
			}
		} else {
			enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
			if enclosing[1] != nil {
				if _, ok := enclosing[1].(*ast.ExprStmt); ok {
					return true
				}
			}
		}
	}
	enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
	for _, index := range enclosing {
		if call, ok := index.(*ast.CallExpr); ok {
			for _, index2 := range call.Args {
				if index2 == r.selectedNode {
					return false
				} else if _, ok := index2.(*ast.BinaryExpr); ok {
					return false
				}
			}
		}
		if _, ok := index.(*ast.CallExpr); ok {
			return true
		}
	}
	return false
}

/*// starCheck checks if the r.selectedNode is a starExpr or
// is a starExpr inside of parenthesis
// TODO: might not need, check all 400 tests to see
func (r *ExtractLocal) starCheck(selectedNode ast.Node) bool {
	if paren, ok := r.selectedNode.(*ast.ParenExpr); ok {
		if _, ok := paren.X.(*ast.StarExpr); ok {
			return true
		}
	}
	if _, ok := r.selectedNode.(*ast.StarExpr); ok {
		return true
	}
	return false
} */

// selectorCheck this will check the selector expr and see if the
// sel part (which is the type) is the r.selectedNode
func (r *ExtractLocal) selectorCheck(selectedNode ast.Node) bool {
	if selector, ok := selectedNode.(*ast.SelectorExpr); ok {
		if selector.Sel == r.selectedNode {
			return true
		}
	}
	return false
}

// checkNil checks if nil is trying to be extracted
func (r *ExtractLocal) checkNil(selectedNode ast.Node) bool {

	if name, ok := selectedNode.(*ast.Ident); ok {
		if name == r.selectedNode {
			if name.Name == "nil" {
				return true
			}
		}
	}
	return false
}

/*// checkBinary checks the lhs of a binary expression to see if that
// was extracted, and also to make sure it's only checking for
// exprs with =, ==, or :=
// TODO: might not need since have lhs of assign one, check all
// 400 tests to see
func (r *ExtractLocal) checkBinary(selectedNode ast.Node) bool {
	if bin, ok := selectedNode.(*ast.BinaryExpr); ok {
		if bin == r.selectedNode {
			return false
		}
	}
	if binary, ok := selectedNode.(*ast.BinaryExpr); ok {
		if binary.X == r.selectedNode && binary.Op.String() == "=" || binary.Op.String() == ":=" {
			return true
		}
	}
	return false
} */

// checkBinaryObj checks if the binary expr is a var type expr
// and stops it since extracting from a var obj would change
// the def of the object
func (r *ExtractLocal) checkBinaryObj(selectedNode ast.Node) bool {
	if binary, ok := selectedNode.(*ast.BinaryExpr); ok {
		if r.posLine(selectedNode) == r.posLine(r.selectedNode) {
			if binary.X == r.selectedNode && binary.Op.String() == "=" || binary.Op.String() == ":=" {
				if ident, ok := binary.X.(*ast.Ident); ok {
					if ident.Obj != nil {
						if ident.Obj.Kind.String() == "var" {
							return true
						}
					}
				}
			}
		}
	}
	return false

}

/*// checkComposite checks if r.selectedNode is a composite lit
// which should be handled by extract, not extract local
// TODO: might not need, check all 400 tests to see
func (r *ExtractLocal) checkComposite() bool {
	if _, ok := r.selectedNode.(*ast.CompositeLit); ok {
		return true
	}
	return false
} */

// checks if it's inside composite lit and if so uses the line position of the composite lit
// as the "parent node" for placement
func (r *ExtractLocal) checkInsideCompsite(selectedNode ast.Node) bool {
	if comp, ok := selectedNode.(*ast.CompositeLit); ok {
		for _, index := range comp.Elts {
			if index == r.selectedNode {
				return true
			}
		}
	}
	return false
}

// checkIndexExpr checks to see if it's an index expr
func (r *ExtractLocal) checkIndexExpr(selectedNode ast.Node) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.file, selectedNode.Pos(), selectedNode.End())
	for _, index := range enclosing {
		if indexExpr, ok := index.(*ast.IndexExpr); ok {
			if indexExpr.X == r.selectedNode || indexExpr.Index == r.selectedNode {
				return true
			}
		}
	}
	return false
}

// checkAssignIdents checks to see if an ident obj is inside an assign, which would be
// allowed to be extracted
func (r *ExtractLocal) checkAssignIdents() bool {
	if ident, ok := r.selectedNode.(*ast.BasicLit); ok {
		enclosing, _ := astutil.PathEnclosingInterval(r.file, ident.Pos(), ident.End())
		for _, index := range enclosing {
			if assign, ok := index.(*ast.AssignStmt); ok {
				for _, index := range assign.Lhs {
					if index == ident {
						enclosing2, _ := astutil.PathEnclosingInterval(r.file, index.Pos(), index.End())
						for _, index2 := range enclosing2 {
							if _, ok := index2.(*ast.SelectorExpr); ok {
								return true
							} else if index2 == assign {
								break
							}
						}
						return false
					}
				}
				return true
			}
		}
	} else if ident, ok := r.selectedNode.(*ast.Ident); ok {
		enclosing, _ := astutil.PathEnclosingInterval(r.file, ident.Pos(), ident.End())
		for _, index := range enclosing {
			if assign, ok := index.(*ast.AssignStmt); ok {
				for _, index := range assign.Rhs {
					if index == ident {
						enclosing2, _ := astutil.PathEnclosingInterval(r.file, index.Pos(), index.End())
						for _, index2 := range enclosing2 {
							if _, ok := index2.(*ast.SelectorExpr); ok {
								return true
							} else if index2 == assign {
								break
							}
						}
						return false
					} else if selector, ok := index.(*ast.SelectorExpr); ok {
						if selector.Sel == ident || selector.X == ident {
							return false
						}
					}
				}
				return true
			}
		}
	}

	return false
}

// caseSelectorCheck check case clauses to see if the case part is trying to be extracted
func (r *ExtractLocal) caseSelectorCheck(switchList []ast.Stmt) bool {
	for _, index := range switchList {
		if cases, ok := index.(*ast.CaseClause); ok {
			if cases == r.selectedNode {
				return true
			}
		}
	}
	return false
}

// function needed to detect if a reserved word is trying to be extracted (a predeclared Ident)
func (r *ExtractLocal) isPreDeclaredIdent() bool {
	pos1 := r.posOff(r.selectedNode)
	pos2 := r.endOff(r.selectedNode)
	selectedNodeName := string(r.fileContents[int(pos1):int(pos2)])
	result, _ := regexp.MatchString("^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|global)$", selectedNodeName)
	return result
}

// addForSwitch switch extract function
func (r *ExtractLocal) addForSwitch(switchNode ast.Node, selectedNode ast.Node, newVar string) bool {
	pos3 := r.posOff(switchNode)
	r.Edits[r.filename].Add(&text.Extent{pos3, 0}, newVar)
	pos1 := r.posOff(selectedNode)
	pos2 := r.endOff(selectedNode) - pos1
	r.Edits[r.filename].Add(&text.Extent{pos1, pos2}, r.varName)
	return true
}

// createVar create the new var to go into the coding
func (r *ExtractLocal) createVar(selectedNode ast.Node) string {
	pos1 := r.posOff(selectedNode)
	pos2 := r.endOff(selectedNode)
	newVar := r.varName + " := " + string(r.fileContents[int(pos1):int(pos2)]) + "\n"
	return newVar
}

/*// createVar2 create new var as var __ ___
func (r *ExtractLocal) createVar2(selectedNode ast.Node) string {
	pos1 := r.posOff(selectedNode)
	pos2 := r.endOff(selectedNode)
	newVar := "var " + r.varName + " " + string(r.fileContents[int(pos1):int(pos2)]) + "\n"
	return newVar
} */

// createParent finds the parent of the selected node, and gives it to the
// function that inputs the newVar into the file at the parent spot and
// at the selected node spot
func (r *ExtractLocal) createParent(newVar string) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.file, r.selectedNode.Pos(), r.selectedNode.End())
	found := false
	var indexOfParent ast.Node
	if len(enclosing) != 0 {
		for index, _ := range enclosing {
			if enclosing[index] != nil {
				if _, ok := enclosing[index].(*ast.LabeledStmt); ok {
					indexOfParent = enclosing[index-1]
					break
				} else if _, ok := enclosing[index].(*ast.CaseClause); ok {
					indexOfParent = enclosing[index-1]
					break
				} else if _, ok := enclosing[index].(*ast.BlockStmt); ok {
					if enclosing[index-1] != nil {
						indexOfParent = enclosing[index-1]
						break
					}
				}
			}
		}
	}
	if indexOfParent != nil {
		found = r.addBeforeParent(indexOfParent, newVar)
	}
	return found
}

// addBeforeParent extract that puts the new var above the parent
func (r *ExtractLocal) addBeforeParent(parentNode ast.Node, newVar string) bool {
	off1 := r.posOff(parentNode)
	off2 := r.posOff(r.selectedNode)
	off3 := r.endOff(r.selectedNode) - off2
	r.Edits[r.filename].Add(&text.Extent{off1, 0}, newVar)
	r.Edits[r.filename].Add(&text.Extent{off2, off3}, r.varName)
	return true
}

// errorCheck checks for any of the errors that were suppose to be thrown
func (r *ExtractLocal) errorCheck(errorType string) {
	if errorType != "" {
		switch errorType {
		case "typeSwitchCase":
			r.Log.Error("You can't extract a type variable from a type switch statement or it's case statements.")
		case "for":
			r.Log.Error("You can't extract from a for loop's conditions (any part in the for statement).")
		case "mulitAssign":
			r.Log.Error("You can't extract from a multi-assign statement (ie: a, b := 0, 0).")
		case "funcInput":
			r.Log.Error("You can't extract from the function parameters/results/method input at the function definition or the function definition itself.")
		case "keyValue":
			r.Log.Error("You can't extract the whole key/value from a key value expression (ie: key: value can't be newVar := key: value).")
		case "assignStmt":
			r.Log.Error("Extracting from a var stmt will alter the definition and should be avoided.")
		case "lhsAssign":
			r.Log.Error("You can't extract a variable from the lhs of an assignment statement.")
		case "callExpr":
			r.Log.Error("You can't extract this part of a 'call expr' (ie:  fmt.Println('____') can't extract the fmt or Println, or fmt.Println).")
		case "ifMulti":
			r.Log.Error("You can't extract from an if statement with an assign stmt in it")
		/*case "starExpr":
		r.Log.Error("You can't put a star expr as a value (ie: *Buffer can't be buffer1 := *Buffer).")*/
		case "blockSelected":
			r.Log.Error("You can't select a block for extract local. Please use the extract refactoring when selecting blocks or functions.")
		case "selectorType":
			r.Log.Error("You can't extract the type from a selector expr (ie: case reflect.Float32:  can't extract Float32).")
		case "nil":
			r.Log.Error("You can't extract nil since nil isn't a type.")
		case "switchKey":
			r.Log.Error("Sorry, you can't extract the switch key or the case selector.")
		case "branchStmt":
			r.Log.Error("Sorry, you can't extract a goto, break, continue, or fallthrough statement")
		case "objInBinary":
			r.Log.Error("You can't extract an object from lhs of == (ie f *ast.FieldList, if f == nil can't extract f ).")
		case "labelStmt":
			r.Log.Error("You can't create a variable for a lable.")
		case "indexExpr":
			r.Log.Error("You can't extract an index expr or the variable that has an index expr with it (ie mapping[beta.result] can't extract mapping or beta.result although you can extract the whole thing.)")
		case "preIdent":
			r.Log.Error("Sorry, you can't pull out a predetermined identifier like string or reflect and make a variable of that type (reflect.String can't be made newVar.String since it isn't type reflect)")
		default:
			r.Log.Error("found an unknown error.")
		}
	}
}
