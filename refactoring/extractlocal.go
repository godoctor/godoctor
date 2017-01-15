// Copyright 2014-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"regexp"
	"strconv"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/text"

	"golang.org/x/tools/go/ast/astutil"
)

type ExtractLocal struct {
	RefactoringBase
	varName string
}

func (r *ExtractLocal) Description() *Description {
	return &Description{
		Name:      "Extract Local Variable",
		Synopsis:  "Extracts an expression, assigning it to a new variable",
		Usage:     "<new_name>",
		HTMLDoc:   extractLocalDoc,
		Multifile: false,
		Params: []Parameter{Parameter{
			Label:        "Name: ",
			Prompt:       "Enter a name for the new variable.",
			DefaultValue: "",
		}},
		Hidden: false,
	}
}

func (r *ExtractLocal) Run(config *Config) *Result {
	r.RefactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	if len(config.Args) != 1 {
		r.Log.Error(errInvalidArgs("expected one argument, got: " +
			strconv.Itoa(len(config.Args))))
		return &r.Result
	}

	r.varName = config.Args[0].(string)
	if !isIdentifierValid(r.varName) {
		r.Log.Errorf("The name \"%s\" is not a valid Go identifier",
			r.varName)
		return &r.Result
	}

	if r.SelectedNode == nil {
		r.Log.Error("Please select an expression to extract.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}

	if r.doExtract2() {
		//r.doExtract()
		r.FormatFileInEditor()
		r.UpdateLog(config, false)
	}
	return &r.Result
}

func isTypeNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.ArrayType:
		return true
	case *ast.InterfaceType:
		return true
	case *ast.MapType:
		return true
	case *ast.StructType:
		return true
	case *ast.TypeSpec:
		return true
	default:
		return false
	}
}

func (r *ExtractLocal) doExtract2() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for i, node := range enclosing {
		fmt.Printf("%d: %s\n", i, reflect.TypeOf(node))
	}

	if _, ok := r.SelectedNode.(ast.Expr); !ok {
		r.Log.Error("Please select an expression to extract.")
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	selectedExpr := r.SelectedNode.(ast.Expr)

	exprType := r.SelectedNodePkg.TypeOf(selectedExpr)
	fmt.Printf("Type: %s\n", exprType) // FIXME
	if _, isFunctionType := exprType.(*types.Tuple); isFunctionType {
		r.Log.Error("The selected expression cannot be assigned to a variable.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	if basic, isBasic := exprType.(*types.Basic); isBasic && basic.Info() == types.IsUntyped {
		r.Log.Error("The selected expression cannot be assigned to a variable.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	if selectorExpr, found := enclosing[1].(*ast.SelectorExpr); found {
		if selectorExpr.Sel == r.SelectedNode {
			r.Log.Error("A field selector cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	for _, node := range enclosing {
		if isTypeNode(node) {
			r.Log.Error("An expression used to specify a type cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	parent := enclosing[1]
	if call, ok := parent.(*ast.CallExpr); ok {
		fmt.Printf("FOUND\n")                                       // FIXME remove
		fmt.Printf("Selected %s\n", reflect.TypeOf(r.SelectedNode)) // FIXME remove
		fmt.Printf("Fun is %s\n", reflect.TypeOf(call.Fun))         // FIXME remove
		if r.SelectedNode == call.Fun {
			r.Log.Error("The function name in a function call expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	enclosingStmt := -1
	for i, node := range enclosing {
		if _, ok := node.(ast.Stmt); ok {
			enclosingStmt = i
			break
		}
	}
	if enclosingStmt < 0 {
		r.Log.Error("The selected expression cannot be extracted " +
			"since it is not in an executable statement.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	var enclosingFunc *ast.FuncDecl
	for _, node := range enclosing {
		if fn, ok := node.(*ast.FuncDecl); ok {
			enclosingFunc = fn
			break
		}
	}
	if enclosingFunc == nil {
		r.Log.Error("The selected expression cannot be extracted " +
			"since it is not in a function declaration.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	if _, found := enclosing[enclosingStmt+1].(*ast.TypeSwitchStmt); found {
		r.Log.Error("The selected expression cannot be extracted " +
			"since it is in the header of a type switch statement.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	/*
		// FIXME (jeff) we can handle this
		// Insert before the IfStmt, not before the init stmt
		// Make sure we don't end up putting a variable before its declaration/definition
		if ifStmt, found := enclosing[enclosingStmtIdx+1].(*ast.IfStmt); found {
			if enclosingStmt == ifStmt.Init {
				r.Log.Error("The selected expression cannot be extracted " +
					"since it is in the initialization portion of an if statement.")
				r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
				return false
			}
		}
	*/

	var insertBefore ast.Node = enclosing[enclosingStmt]

	switch stmt := enclosing[enclosingStmt].(type) {
	case *ast.AssignStmt:
		for _, lhsExpr := range stmt.Lhs {
			if r.SelectedNode == lhsExpr {
				r.Log.Error("The selected expression cannot be extracted " +
					"since it is in the left-hand side of an assignment.")
				r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
				return false
			}
		}
		break
	case *ast.CaseClause:
		if _, found := enclosing[enclosingStmt+2].(*ast.TypeSwitchStmt); found {
			r.Log.Error("The selected expression cannot be extracted " +
				"since it is in a case clause for a type switch statement.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
		insertBefore = enclosing[enclosingStmt+2] // grandparent
		break
	//case *ast.DeclStmt: // const, type, or var... is this ok?
	//	break
	//case *ast.DeferStmt:
	//	break
	// EmptyStmt impossible
	case *ast.ExprStmt:
		break
	// ForStmt not allowed (control flow) - must be in init expr
	//case *ast.GoStmt:
	//	break
	case *ast.IfStmt: // really?
		c := cfg.FromFunc(enclosingFunc)
		in, _ := dataflow.ReachingDefs(c, r.SelectedNodePkg)
		if _, found := in[stmt][stmt.Init]; found {
			r.Log.Error("The selected expression cannot be extracted " +
				"since the \"if\" condition depends on variables " +
				"defined in the \"if\" statement initialization.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}

		// If inside an else if statement, find outermost if
		for j := enclosingStmt; j < len(enclosing); j++ {
			if ifStmt, ok := enclosing[j].(*ast.IfStmt); ok {
				insertBefore = ifStmt
			} else {
				break
			}
		}
		break
	//case *ast.IncDecStmt: // really?
	//	break
	// LabeledStmt not allowed - label cannot be extracted
	case *ast.RangeStmt: // ??
		break
	case *ast.ReturnStmt:
		break
	//case *ast.SelectStmt: // check type
	//	break
	//case *ast.SendStmt: // ?
	//	break
	//case *ast.SwitchStmt:
	//	break
	//case *ast.TypeSwitchStmt:
	//	break
	default:
		r.Log.Error("The selected expression cannot be extracted.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	off1 := r.posOff(insertBefore)
	assignment := r.createVar(selectedExpr)
	r.Edits[r.Filename].Add(&text.Extent{off1, 0}, assignment)

	off2 := r.posOff(r.SelectedNode)
	off3 := r.endOff(r.SelectedNode) - off2
	r.Edits[r.Filename].Add(&text.Extent{off2, off3}, r.varName)
	return true
}

// doExtract locates the position of the selection and will
// replace it with a new variable of the user's chosing
func (r *ExtractLocal) doExtract() {
	var enclosingSwitchStmtBodyList, enclosingTypeSwitchBodyList []ast.Stmt
	var enclosingFuncParams, enclosingFuncResults, enclosingFuncRecv []*ast.Field
	var lhs, rhs []ast.Expr
	var enclosingForStmtInit, enclosingForStmtCond, enclosingForStmtPos, parentNode, enclosingSwitchStmt, enclosingIfStmtInitAssign, enclosingIfStmtElse, enclosingTypeSwitchInit, enclosingTypeSwitchAssign ast.Node
	found, isBranchStmt, isLabelStmt := false, false, false
	errorType := ""
	ast.Inspect(r.File, func(n ast.Node) bool {
		switch curNode := n.(type) {
		case *ast.FuncDecl:
			if curNode.Type.Params != nil {
				enclosingFuncParams = curNode.Type.Params.List
			}
			if curNode.Type.Results != nil {
				enclosingFuncResults = curNode.Type.Results.List
			}
			if curNode.Recv != nil {
				enclosingFuncRecv = curNode.Recv.List
			}
		case ast.Stmt:
			parentNode = curNode
			if sNode, ok := curNode.(*ast.SwitchStmt); ok {
				enclosingSwitchStmt = sNode
				enclosingSwitchStmtBodyList = sNode.Body.List
			} else if ifNode, ok := curNode.(*ast.IfStmt); ok {
				if assignNode, ok := ifNode.Init.(*ast.AssignStmt); ok {
					enclosingIfStmtInitAssign = assignNode
				} else if ifNode.Else != nil {
					enclosingIfStmtElse = ifNode.Else
				}
			} else if fNode, ok := curNode.(*ast.ForStmt); ok {
				enclosingForStmtInit = fNode.Init
				enclosingForStmtCond = fNode.Cond
				enclosingForStmtPos = fNode.Post
				isForCond := r.selectedNodeIsInForLoopHeader()
				if isForCond {
					found = true
					errorType = "for"
				}
			} else if typeSwitch, ok := curNode.(*ast.TypeSwitchStmt); ok {
				body := typeSwitch.Body.List
				if len(body) != 0 {
					enclosingTypeSwitchBodyList = body
				}
				if typeSwitch.Init != nil {
					enclosingTypeSwitchInit = typeSwitch.Init
				} else if typeSwitch.Assign != nil {
					enclosingTypeSwitchAssign = typeSwitch.Assign
				}
			} else if assignment, ok := curNode.(*ast.AssignStmt); ok {
				lhs, rhs = assignment.Lhs, assignment.Rhs
			} else if block, ok := curNode.(*ast.BlockStmt); ok {
				if block == r.SelectedNode {
					errorType = "blockSelected"
					found = true
				}
			} else if branch, ok := curNode.(*ast.BranchStmt); ok {
				if /*branch == r.SelectedNode ||*/ branch.Label == r.SelectedNode || r.posLine(branch) == r.posLine(r.SelectedNode) {
					isBranchStmt = true
				}
			} else if label, ok := curNode.(*ast.LabeledStmt); ok {
				if label == r.SelectedNode {
					isLabelStmt = true
				}
			}
		case ast.Expr:
			isInMultiLHSAssignment := r.isInMultiLHSAssignment(lhs, rhs)
			isInDeclStmt := r.isInDeclStmt()
			isInLHSofAssignStmt := r.isInLHSofAssignStmt()
			isInCallExprNotInArgs := r.isInCallExprNotInArgs(curNode)
			isSelectorExprAndIsEqualToSelectedNode := r.isSelectorExprAndIsEqualToSelectedNode(curNode)
			isIdentNamedNilAndIsEqualToSelectedNode := r.isIdentNamedNilAndIsEqualToSelectedNode(curNode)
			selectedNodeIsKeyValueExpr := r.selectedNodeIsKeyValueExpr()
			selectedNodeIsCompositeLit := r.selectedNodeIsCompositeLit()
			selectedNodeIsOKOnRHSofAssign := r.selectedNodeIsIdentOnRHSofAssignButNotInSelectorExpr()
			isInFuncParamResultOrRecv := r.isInFuncParamResultOrRecv(enclosingFuncParams, enclosingFuncResults, enclosingFuncRecv)
			// might not need this one, will have to wait till after
			selectedNodeIsCaseClause := r.selectedNodeIsCaseClause(enclosingSwitchStmtBodyList)
			if isInMultiLHSAssignment { // check if selected node is from part a _, _ := _, _ stmt
				errorType = "mulitAssign"
				found = true
			} else if r.isPreDeclaredIdent() {
				errorType = "preIdent"
				found = true
			} else if isInDeclStmt { // checks if its a var _ _ stmt
				errorType = "assignStmt"
				found = true
			} else if isInLHSofAssignStmt { // checks if its from the lhs of enclosingTypeSwitchAssign stmt
				errorType = "lhsAssign"
				found = true
			} else if isInCallExprNotInArgs { // checks if it's in call expr
				errorType = "callExpr"
				found = true
			} else if isInFuncParamResultOrRecv { // check if selected node is from the function definition
				errorType = "funcInput"
				found = true
			} else if isSelectorExprAndIsEqualToSelectedNode { // check for when a selector type
				errorType = "selectorType"
				found = true
			} else if selectedNodeIsCaseClause { // checks if its from the switch type (ie switch node, checks if its node)
				errorType = "switchKey"
				found = true
			} else if isIdentNamedNilAndIsEqualToSelectedNode { // checks if it's nil
				errorType = "nil"
				found = true
			} else if selectedNodeIsKeyValueExpr { // check to see if from a key:value
				errorType = "keyValue"
				found = true
			} else if selectedNodeIsCompositeLit {
				errorType = "blockSelected"
				found = true
			} else if isBranchStmt {
				errorType = "branchStmt"
				found = true
			} else if isLabelStmt {
				errorType = "labelStmt"
				found = true
			} else if selectedNodeIsOKOnRHSofAssign {
				newVar := r.createVar(r.SelectedNode)
				found = r.createParent(newVar)
			} else if curNode == r.SelectedNode {
				typeCase := r.selectedNodeIsInCaseClause(enclosingTypeSwitchBodyList, enclosingTypeSwitchInit, enclosingTypeSwitchAssign)
				isIfMult := r.isInIfStmtInitAssign(enclosingIfStmtInitAssign, curNode)
				isSwitchCase := r.isInSwitchStmtBodyList(enclosingSwitchStmtBodyList, curNode)
				line1 := r.endLine(curNode)
				line2 := r.endLine(r.SelectedNode)
				//isIndexExpr := r.checkIndexExpr(curNode)
				if isSwitchCase { // check for switch stmts
					newVar := r.createVar(curNode)
					found = r.addForSwitch(enclosingSwitchStmt, curNode, newVar)
				} else if typeCase { // check if selected node is from part of the type switch
					errorType = "typeSwitchCase"
					found = true
				} else if isIfMult { // check for for loops or if mutli enclosingTypeSwitchAssign
					found = true
					errorType = "ifMulti"
				} else if line1 == line2 || enclosingIfStmtElse != nil {
					newVar := r.createVar(curNode)
					found = r.createParent(newVar)
				} /* else if isStarExpr { // checks if it's a star expr
						errorType = "starExpr"
						found = true
					} else if isIndexExpr {
					errorType = "indexExpr"
					found = true
				} */
			}
		default:
		}
		// used to end the inspect func once an extraction is done
		if found == false {
			return true
		}
		return false

	})
	// FIXME (jeff) r.errorCheck(errorType)
}
func (r *ExtractLocal) posOff(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Offset
}
func (r *ExtractLocal) endOff(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Offset
}
func (r *ExtractLocal) endLine(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Line
}
func (r *ExtractLocal) posLine(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Line
}

// isInSwitchStmtBodyList switch case check for switch version of extraction
func (r *ExtractLocal) isInSwitchStmtBodyList(switchList []ast.Stmt, selectedNode ast.Node) bool {
	for index, _ := range switchList {
		line1 := r.posLine(switchList[index])
		line2 := r.posLine(selectedNode)
		if line1 == line2 {
			return true
		}
	}
	return false
}

// selectedNodeIsInForLoopHeader for loop init/cond/post test
func (r *ExtractLocal) selectedNodeIsInForLoopHeader() bool {
	off1 := r.posLine(r.SelectedNode)
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index2 := range enclosing {
		if forparent, ok := index2.(*ast.ForStmt); ok {
			if r.posLine(forparent) == off1 {
				return true
			}
		}
	}
	return false
}

// isInIfStmtInitAssign if stmt check for _, _ conditions on the left
func (r *ExtractLocal) isInIfStmtInitAssign(ifInitAssign ast.Node, selectedNode ast.Node) bool {
	if ifInitAssign != nil {
		line1 := r.posLine(ifInitAssign)
		line2 := r.posLine(selectedNode)
		if line1 == line2 {
			return true
		}
	}
	return false
}

// selectedNodeIsInCaseClause sees if the switch is a type switch and if so returns true to produce an error
func (r *ExtractLocal) selectedNodeIsInCaseClause(typeSwitchFound []ast.Stmt, condition ast.Node, assign ast.Node) bool {
	if len(typeSwitchFound) != 0 {
		for _, index := range typeSwitchFound {
			if caseClauses, ok := index.(*ast.CaseClause); ok {
				for _, index2 := range caseClauses.List {
					if index2 == r.SelectedNode {
						return true
					}
					enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
					for _, index3 := range enclosing {
						if index3 == index2 {
							return true
						}
					}
				}
			}
		}
	}
	if condition == r.SelectedNode {
		return true
	} else if assign != nil {
		if aStmt, ok := assign.(*ast.AssignStmt); ok {
			if aStmt.Lhs != nil && aStmt.Lhs[0] == r.SelectedNode {
				return true
			} else if aStmt.Rhs != nil && aStmt.Rhs[0] == r.SelectedNode {
				return true
			}
		}
	}
	return false
}

// isInFuncParamResultOrRecv checks to see if selected node is a function parameter or result
// in the definition and returns true if it is so as to produce an error
func (r *ExtractLocal) isInFuncParamResultOrRecv(plists []*ast.Field, rlists []*ast.Field, recLists []*ast.Field) bool {
	if len(plists) != 0 {
		for _, index := range plists {
			for _, name := range index.Names {
				if name == r.SelectedNode {
					return true
				}
			}
		}
	}
	if len(rlists) != 0 {
		for _, index := range rlists {
			if r.posLine(index) == r.posLine(r.SelectedNode) {
				return true
			}
		}
	}
	if len(recLists) != 0 {
		for _, index := range recLists {
			if r.posLine(index) == r.posLine(r.SelectedNode) {
				return true
			} /*else if index == r.SelectedNode {
				fmt.Printf("goes here")
				return true
			}
			for _, names := range index.Names {
				if names == r.SelectedNode {
					fmt.Printf("goes here")
					return true
				}
			}*/
		}
	}
	return false
}

// isInMultiLHSAssignment checks if it's a multi assign stmt ie  _, _ := _, _
// and returns true if the selected node is on either side
func (r *ExtractLocal) isInMultiLHSAssignment(lhs []ast.Expr, rhs []ast.Expr) bool {
	if len(lhs) > 1 && r.posLine(lhs[0]) == r.posLine(r.SelectedNode) {
		return true
	}
	return false
}

/*// ifFmtWBinary checks if the if stmt has mulitple conditions/inital conditions
// ie if x := a.Type(); x != y {
// TODO: might not need, check all 400 tests to see
func (r *ExtractLocal) ifFmtWBinary(parentNode ast.Node, childNode ast.Node, curNode ast.Node) bool {
	if parentNode != nil && childNode != nil {
		line1 := r.posLine(childNode)
		line2 := r.posLine(curNode)
		if line1 == line2 {
			return true
		}
	}
	return false
} */

// selectedNodeIsKeyValueExpr checks if is a key value expr or a child of
// a key value expr, so anything to do maps, or keys : values
func (r *ExtractLocal) selectedNodeIsKeyValueExpr() bool {
	if _, ok := r.SelectedNode.(*ast.KeyValueExpr); ok {
		return true
	}
	return false
}

// isInDeclStmt check if it's a declartion stmt, and return true
// if it is ie var apple int or var handler http.Handler
func (r *ExtractLocal) isInDeclStmt() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if _, ok := index.(*ast.DeclStmt); ok {
			return true
		}
	}
	return false
}

// isInLHSofAssignStmt checks if r.SelectedNode is on the lhs of an
// enclosingTypeSwitchAssign statement
func (r *ExtractLocal) isInLHSofAssignStmt() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if assign, ok := index.(*ast.AssignStmt); ok {
			for _, index2 := range assign.Lhs {
				if _, ok := index2.(*ast.SelectorExpr); ok {
					return true
				}
				if index2 == r.SelectedNode {
					return true
				}
			}
		}
	}
	return false
}

// isInCallExprNotInArgs check for call expr and don't allow if its just
// a call expr itself, or if part of the callexpr (although
// inside the () of it should work)
func (r *ExtractLocal) isInCallExprNotInArgs(selectedNode ast.Node) bool {
	/*if call, ok := curNode.(*ast.CallExpr); ok {
		if call == r.SelectedNode {
			return true
		} else if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
			if fun == r.SelectedNode || fun.X == r.SelectedNode || fun.Sel == r.SelectedNode {
				return true
			}
		} else if len(call.Args) != 0 {
			for _, index := range call.Args {
				if selector, ok := index.(*ast.SelectorExpr); ok {
					if selector == r.SelectedNode || selector.X == r.SelectedNode || selector.Sel == r.SelectedNode {
						return true
					}
				} else if ident, ok := index.(*ast.Ident); ok {
					if ident == r.SelectedNode {
						return false
					}
				}
			}
		} else {
			enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
			if enclosing[1] != nil {
				if _, ok := enclosing[1].(*ast.ExprStmt); ok {
					return true
				}
			}
		}
	} */
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if call, ok := index.(*ast.CallExpr); ok {
			for _, index2 := range call.Args {
				if index2 == r.SelectedNode {
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

// isSelectorExprAndIsEqualToSelectedNode this will check the selector expr and see if the
// sel part (which is the type) is the r.SelectedNode
func (r *ExtractLocal) isSelectorExprAndIsEqualToSelectedNode(selectedNode ast.Node) bool {
	if selector, ok := selectedNode.(*ast.SelectorExpr); ok {
		if selector.Sel == r.SelectedNode {
			return true
		}
	}
	return false
}

// isIdentNamedNilAndIsEqualToSelectedNode checks if nil is trying to be extracted
func (r *ExtractLocal) isIdentNamedNilAndIsEqualToSelectedNode(selectedNode ast.Node) bool {

	if name, ok := selectedNode.(*ast.Ident); ok {
		if name == r.SelectedNode {
			if name.Name == "nil" {
				return true
			}
		}
	}
	return false
}

// selectedNodeIsCompositeLit checks if r.SelectedNode is a composite lit
// which should be handled by extract, not extract local
func (r *ExtractLocal) selectedNodeIsCompositeLit() bool {
	if _, ok := r.SelectedNode.(*ast.CompositeLit); ok {
		return true
	}
	return false
}

/*// checkIndexExpr checks to see if it's an index expr
func (r *ExtractLocal) checkIndexExpr(curNode ast.Node) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, curNode.Pos(), curNode.End())
	for _, index := range enclosing {
		if indexExpr, ok := index.(*ast.IndexExpr); ok {
			if indexExpr.X == r.SelectedNode || indexExpr.Index == r.SelectedNode {
				return true
			}
		}
	}
	return false
}*/

// selectedNodeIsIdentOnRHSofAssignButNotInSelectorExpr checks to see if an ident obj is inside an enclosingTypeSwitchAssign, which would be
// allowed to be extracted
func (r *ExtractLocal) selectedNodeIsIdentOnRHSofAssignButNotInSelectorExpr() bool {
	/*if ident, ok := r.SelectedNode.(*ast.BasicLit); ok {
		enclosing, _ := astutil.PathEnclosingInterval(r.File, ident.Pos(), ident.End())
		for _, index := range enclosing {
			if enclosingTypeSwitchAssign, ok := index.(*ast.AssignStmt); ok {
				for _, index := range enclosingTypeSwitchAssign.Lhs {
					if index == ident {
						enclosing2, _ := astutil.PathEnclosingInterval(r.File, index.Pos(), index.End())
						for _, node := range enclosing2 {
							if _, ok := node.(*ast.SelectorExpr); ok {
								return true
							} else if node == enclosingTypeSwitchAssign {
								break
							}
						}
						return false
					}
				}
				return true
			}
		}
	} else*/if ident, ok := r.SelectedNode.(*ast.Ident); ok {
		enclosing, _ := astutil.PathEnclosingInterval(r.File, ident.Pos(), ident.End())
		for _, index := range enclosing {
			if assign, ok := index.(*ast.AssignStmt); ok {
				for _, index := range assign.Rhs {
					/*if index == ident {
						enclosing2, _ := astutil.PathEnclosingInterval(r.File, index.Pos(), index.End())
						for _, node := range enclosing2 {
							if _, ok := node.(*ast.SelectorExpr); ok {
								return true
							} else if node == enclosingTypeSwitchAssign {
								break
							}
						}
						return false
					} else*/if selector, ok := index.(*ast.SelectorExpr); ok {
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

// selectedNodeIsCaseClause check case clauses to see if the case part is trying to be extracted
func (r *ExtractLocal) selectedNodeIsCaseClause(switchList []ast.Stmt) bool {
	for _, index := range switchList {
		if cases, ok := index.(*ast.CaseClause); ok {
			if cases == r.SelectedNode {
				return true
			}
		}
	}
	return false
}

// function needed to detect if a reserved word is trying to be extracted (a predeclared Ident)
func (r *ExtractLocal) isPreDeclaredIdent() bool {
	pos1 := r.posOff(r.SelectedNode)
	pos2 := r.endOff(r.SelectedNode)
	selectedNodeName := string(r.FileContents[int(pos1):int(pos2)])
	result, _ := regexp.MatchString("^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|global|reflect)$", selectedNodeName)
	return result
}

// addForSwitch switch extract function
func (r *ExtractLocal) addForSwitch(switchNode ast.Node, selectedNode ast.Node, newVar string) bool {
	pos3 := r.posOff(switchNode)
	r.Edits[r.Filename].Add(&text.Extent{pos3, 0}, newVar)
	pos1 := r.posOff(selectedNode)
	pos2 := r.endOff(selectedNode) - pos1
	r.Edits[r.Filename].Add(&text.Extent{pos1, pos2}, r.varName)
	return true
}

// createVar create the new var to go into the coding
func (r *ExtractLocal) createVar(selectedNode ast.Node) string {
	pos1 := r.posOff(selectedNode)
	pos2 := r.endOff(selectedNode)
	newVar := r.varName + " := " + string(r.FileContents[int(pos1):int(pos2)]) + "\n"
	return newVar
}

/*// createVar2 create new var as var _
  // just in case there is a extract that you have to do var a ___
  // rather than a := ______ ___
func (r *ExtractLocal) createVar2(curNode ast.Node) string {
	pos1 := r.posOff(curNode)
	pos2 := r.endOff(curNode)
	newVar := "var " + r.varName + " " + string(r.FileContents[int(pos1):int(pos2)]) + "\n"
	return newVar
} */

// createParent finds the parent of the selected node, and gives it to the
// function that inputs the newVar into the file at the parent spot and
// at the selected node spot
func (r *ExtractLocal) createParent(newVar string) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
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
	off2 := r.posOff(r.SelectedNode)
	off3 := r.endOff(r.SelectedNode) - off2
	r.Edits[r.Filename].Add(&text.Extent{off1, 0}, newVar)
	r.Edits[r.Filename].Add(&text.Extent{off2, off3}, r.varName)
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
			r.Log.Error("You can't extract from the function parameters/results/method input at the function definition or the whole function itself (for full function extraction use the extract refactoring).")
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
		case "labelStmt":
			r.Log.Error("You can't create a variable for a lable.")
		/*case "indexExpr":
		r.Log.Error("You can't extract an index expr or the variable that has an index expr with it (ie mapping[beta.result] can't extract mapping or beta.result although you can extract the whole thing.)")*/
		case "preIdent":
			r.Log.Error("Sorry, you can't pull out a predetermined identifier like string or reflect and make a variable of that type (reflect.String can't be made newVar.String since it isn't type reflect)")
		default:
			r.Log.Error("found an unknown error.")
		}
	}
}

const extractLocalDoc = `
  <h4>Purpose</h4>
  <p>The Extract Local Variable refactoring creates a new variable FIXME,
  then replaces the original expression with that variable.</p>

  <h4>Usage</h4>
  <ol class="enum">
    <li>Select an expression in an existing statement.</li>
    <li>Activate the Extract Local Variable refactoring.</li>
    <li>Enter a name for the new variable that will be created.</li>
  </ol>

  <p>An error or warning will be reported if the selected expression cannot be
  extracted into a new variable.  Usually, this occurs because FIXME.</p>
`
