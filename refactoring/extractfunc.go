// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/internal/golang.org/x/tools/astutil"
	"github.com/godoctor/godoctor/internal/golang.org/x/tools/go/loader"
	"github.com/godoctor/godoctor/internal/golang.org/x/tools/go/types"
	"github.com/godoctor/godoctor/text"
)

// The ExtractFunc refactoring is used to break down larger functions into
// smaller functions such that the logic of the code remains unchanged.
// The user is expected to extract a part of code from the function and enter a valid name

type ExtractFunc struct {
	RefactoringBase
	funcName string
}

func (r *ExtractFunc) Description() *Description {
	return &Description{
		Name:      "Extract Function",
		Synopsis:  "Extracts statements to a new function/method",
		Usage:     "<new_name>",
		Multifile: false,
		Params:    nil,
		Hidden:    false,
	}
}

func (r *ExtractFunc) Run(config *Config) *Result {
	if r.RefactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}
	if len(config.Args) != 1 {
		r.Log.Error(errInvalidArgs("expected one argument, got: " + strconv.Itoa(len(config.Args))))
		return &r.Result
	}

	r.funcName = (config.Args[0]).(string)
	if !r.isIdentifierValid(r.funcName) {
		r.Log.Error("Please select a valid Go identifier")
		return &r.Result
	}

	if r.SelectedNode == nil {
		r.Log.Error(errInvalidSelection("no position specified"))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}
	path, _ := astutil.PathEnclosingInterval(r.File, r.SelectionStart, r.SelectedNode.End())
	switch node := r.SelectedNode.(type) {
	case *ast.BlockStmt:
		flag := r.checkForAnonymousFns(path)
		if flag {
			r.callEditset(node, path)
		} else {
			r.Log.Error("Code cannot be extracted from anonymous functions.")
		}
	default:
		flag := r.checkForAnonymousFns(path)
		if flag {
			r.checkForBlockStmt(node, path)
		} else {
			r.Log.Error("Code cannot contain Anonymous Function (OR) Code Cannot be extracted from Anonymous Function")
		}
	}
	r.FormatFileInEditor()
	r.UpdateLog(config, true)
	r.Log.ChangeInitialErrorsToWarnings()
	return &r.Result
}

// checks if the extracted code is part of anonymous functions, if so, then returns false
func (r *ExtractFunc) checkForAnonymousFns(path []ast.Node) bool {
	flag := true
	for _, parentNode := range path {
		if _, ok := parentNode.(*ast.FuncLit); ok {
			flag = false
			return flag
		}
	}
	anonymousFlag := r.checkForAnonymousFnsInCode()
	return flag && anonymousFlag
}

// check for anonymous function by performing ast.inspect
func (r *ExtractFunc) checkForAnonymousFnsInCode() bool {
	flag := true
	ast.Inspect(r.SelectedNode, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			r.Log.Error("There is an anonymous function in the code")
			flag = false
		}
		return true
	})
	return flag
}

// parses through the parent nodes of extracted node and returns the immediate *ast.BlockStmt
func (r *ExtractFunc) checkForBlockStmt(node ast.Node, path []ast.Node) {
	for _, parentNode := range path {
		if node, ok := parentNode.(*ast.BlockStmt); ok {
			r.callEditset(node, path)
			return
		}
	}
}

// If the users start selection is in the midway of a node, then the entire node is selected as the starting node
// TODO reduce the calls of this, store a more useful version of it inside of our *ExtractFunc
func (r *ExtractFunc) findStartPosition(node *ast.BlockStmt) ast.Stmt {
	var stmt ast.Stmt
	line := r.Program.Fset.Position(r.SelectionStart).Line
	for _, s := range node.List {
		ast.Inspect(s, func(n ast.Node) bool {
			if a, ok := n.(ast.Stmt); ok {
				if r.SelectionStart == a.Pos() {
					stmt = a
					return false
				} else if r.Program.Fset.Position(a.Pos()).Line == line && // if it's on same line
					r.SelectionStart < a.End() && // and less than end
					(r.SelectionStart > a.Pos() || // and we're either in the middle of a node
						stmt == nil || // or we haven't found anything
						a.Pos()-r.SelectionStart < stmt.Pos()-r.SelectionStart) { // or closer than what we have found
					// NOTE: [I] do not trust this approximation very much. it attempts to fudge the beginning of the
					// selection to the right, b/c in the case that they selected the beginning of a line, we want to find
					// the first statement, and skews left in the case that they made a selection in the middle of the node.
					// the only case of invalid input I can think of is selecting _only_ whitespace, which should be
					// guarded against by checking the end boundary coupled with there not being an ast.Stmt there. bueller?
					stmt = a
				}
			}
			return true
		})
	}
	return stmt
}

// If the users end selection is in the midway of a node, then the entire node is selected as the last node
// TODO reduce the calls of this, store a more useful version of it inside of our *ExtractFunc
func (r *ExtractFunc) findEndPosition(node *ast.BlockStmt) ast.Stmt {
	var stmt ast.Stmt = nil
	for _, s := range node.List {
		ast.Inspect(s, func(n ast.Node) bool {
			if s, ok := n.(ast.Stmt); ok {
				if s.Pos() >= r.SelectionStart && s.Pos() <= r.SelectionEnd { // since there can be one to many statements
					stmt = s
					return false
				}
			}
			return true
		})
	}
	return stmt
}

// to call the editset function to update the function call and the function definition
// throws an error if the start node or the end node is nil
func (r *ExtractFunc) callEditset(node *ast.BlockStmt, path []ast.Node) {
	start := r.findStartPosition(node)
	end := r.findEndPosition(node)
	if start == nil || end == nil {
		r.Log.Error(errInvalidSelection("could not find specified bounds"))
		return
	}

	replace, funcCall := r.createNewFunction(node, path)
	length := r.Program.Fset.Position(end.End()).Offset - r.Program.Fset.Position(start.Pos()).Offset
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(start.Pos()).Offset, length}, funcCall)
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(r.File.End()).Offset, 0}, replace)
}

// this function returns the func Call and func Definition strings
func (r *ExtractFunc) createNewFunction(node *ast.BlockStmt, path []ast.Node) (string, string) {
	receieverName, receiverType, rType := r.checkForReceiver(path)
	paramVarList, returnVarList, varDecl, flagShortAssign := r.parseCode(node, path)
	varName, varType := r.returnNameType(paramVarList, path, rType, true)
	retVarName, retVarType := r.returnNameType(returnVarList, path, nil, false) // the final boolean is for pointers that must not be returned
	varDeclName, varDeclType := r.returnNameType(varDecl, path, rType, true)
	r.checkParameters(receieverName, varName, varDeclName)
	if len(retVarType) == 1 { // if the variable returned is of the receiver type, the shortAssign flag is set to false
		if returnVarList[0].Type() == rType {
			flagShortAssign = false
		}
	}
	return r.createEditString(r.createString(varDeclName, varDeclType, true), r.createString(varName, nil, false), r.createString(varName, varType, false),
		r.createString(retVarName, nil, false), r.createString(nil, retVarType, false), len(retVarName), r.extractCode(node), receieverName,
		receiverType, flagShortAssign)
}

// checks if code is extracted from a method or a normal function, if it is extracted from a method, it returns receiver's type/name.
func (r *ExtractFunc) checkForReceiver(path []ast.Node) (string, string, types.Type) {
	var receieverName string
	var receiverType string
	var rType types.Type
	for i := 0; i < len(path); i++ {
		switch s := path[i].(type) {
		case *ast.FuncDecl:
			if s.Recv != nil {
				receieverName = s.Recv.List[0].Names[0].Name
				switch a := s.Recv.List[0].Type.(type) {
				case *ast.StarExpr:
					receiverType = "*" + a.X.(*ast.Ident).Name
					rType = r.SelectedNodePkg.TypeOf(a)
				case ast.Expr:
					receiverType = a.(*ast.Ident).Name
					rType = r.SelectedNodePkg.TypeOf(a)
				}
			}
		}
	}
	return receieverName, receiverType, rType
}

// for every variable that is passed/returned/declared the function returns the name and type
func (r *ExtractFunc) returnNameType(varList []*types.Var, path1 []ast.Node, rType types.Type, boolVar bool) ([]string, []string) {
	var varType []string
	var varName []string
	for _, a := range varList {
		if rType != nil && a.Type() == rType { // if the receiver type is same as the current package, do nothing
			continue
		} else {
			switch b := (a.Type()).(type) {
			case *types.Named:
				if b.Obj().Type().String() != "error" {
					if b.Obj().Pkg().Name() == path1[len(path1)-1].(*ast.File).Name.Name { // prefix is of the current package, remove prefix
						if !a.IsField() && a.Name() != "_" {
							varName = append(varName, a.Name())
							varType = append(varType, strings.TrimPrefix(a.Type().String(), b.Obj().Pkg().Path()+"."))
						}
					} else {
						if !a.IsField() && a.Name() != "_" {
							varName = append(varName, a.Name())
							varType = append(varType, b.Obj().Pkg().Name()+"."+b.Obj().Id())
						}
					}
				} else {
					if !a.IsField() && a.Name() != "_" {
						varName = append(varName, a.Name())
						varType = append(varType, b.Obj().Type().String())
					}
				}
			case *types.Pointer:
				switch temp := b.Elem().(type) {
				case *types.Basic:
					if temp.Name() == "float64" || temp.Name() == "float32" || temp.Name() == "int" || temp.Name() == "string" {
						if !a.IsField() && a.Name() != "_" && boolVar == true {
							varName = append(varName, a.Name())
							varType = append(varType, "*"+b.Elem().(*types.Basic).Name())
						}
					}
				case *types.Named:
					if temp.Obj().Pkg().Name() == path1[len(path1)-1].(*ast.File).Name.Name { // prefix is of the current package, remove prefix
						if !a.IsField() && a.Name() != "_" && boolVar == true {
							varName = append(varName, a.Name())
							varType = append(varType, "*"+strings.TrimPrefix(a.Type().String(), "*"+b.Elem().(*types.Named).Obj().Pkg().Path()+"."))
						}
					} else if temp.Obj().Pkg().Name() != path1[len(path1)-1].(*ast.File).Name.Name {
						if !a.IsField() && a.Name() != "_" && boolVar == true {
							varName = append(varName, a.Name())
							varType = append(varType, "*"+b.Elem().(*types.Named).Obj().Pkg().Name()+"."+b.Elem().(*types.Named).Obj().Name())
						}
					}
				}
			default:
				if !a.IsField() && a.Name() != "_" {
					varName = append(varName, a.Name())
					varType = append(varType, r.TypeString(a.Pkg(), a.Type()))
				}
			}
		}
	}
	return varName, varType
}

// if the name of the receiver and that of any of the arguments passed are the same then do not allow transformation
// function of check if the receiver name and that of the parametr names are the same
func (r *ExtractFunc) checkParameters(receieverName string, varName []string, varDeclName []string) {
	for _, a := range varName {
		if a == receieverName {
			r.Log.Error("The method receiever name and the parameters passed cannot have the same name")
		}
	}
	for _, a := range varDeclName {
		if a == receieverName {
			r.Log.Error("The method receiever name and the parameters declared cannot have the same name")
		}
	}
}

// returns the string representation of the code
func (r *ExtractFunc) extractCode(node *ast.BlockStmt) []byte {
	start := r.findStartPosition(node)
	end := r.findEndPosition(node)
	// TODO(reed): these find[Start|End]Position functions seem used too frequently,
	// and all calls appear to be on the same 'node' which we got from PathEnclosingInterval,
	// which is supposed to already put us around the correct spot. we should, at least,
	// call these early in Run and store as fields in our *ExtractFunc. In the best case,
	// we should stop passing the opaque 'node' around and use a more useful section of the ast.
	if start == nil || end == nil {
		r.Log.Error("The start positon of the extracted code is not right")
		return nil // TODO this should be verified before this func is ever run
	}

	// TODO(reed): can we just use a more meaningful subset of []ast.Stmt instead of the entire block?
	for i := 0; i < len(node.List); i++ {
		if node.List[i].Pos() >= start.Pos() {
			a := r.Program.Fset.Position(start.Pos()).Offset
			b := r.Program.Fset.Position(end.End()).Offset
			return r.FileContents[a:b]
		}
	}
	return nil // logged that we got issues
}

// This function schecks if the selection is valid by using the following technique:
// 1. Find the path-enclosing intervals for the first and last statement of the selected node.
// 2. Filter out only the statement node and check for LabelStatements,
// 3. If there are label-statements in the path, delete the immediate child node of that labeled statement
// 4. After this  compare the path enclosing interval of both the statements(First and Last Statements).
// 5. If they match, then the code is extracted right, else throw an error.
func (r *ExtractFunc) checkValidSelection(stmtArr []ast.Stmt) bool {

	var path1Stmt []ast.Stmt
	var path2Stmt []ast.Stmt
	var path11 []ast.Stmt
	var path22 []ast.Stmt
	var index1 []int
	var index2 []int
	if len(stmtArr) != 0 {
		sort.Sort(nodeStmt(stmtArr))
		firstStmt := stmtArr[0]
		sort.Sort(nodeEnd(stmtArr)) // sorted based on the ascending order of node.End() value
		endStmt := stmtArr[len(stmtArr)-1]
		path1, _ := astutil.PathEnclosingInterval(r.File, firstStmt.Pos(), firstStmt.End())
		path2, _ := astutil.PathEnclosingInterval(r.File, endStmt.Pos(), endStmt.End())
		for i, _ := range path1 {
			if aa, ok := path1[i].(ast.Stmt); ok {
				if _, ok := path1[i].(*ast.LabeledStmt); ok {
					index1 = append(index1, i-1)
				}
				path1Stmt = append(path1Stmt, aa)
			}
		}
		for i, _ := range path2 {
			if aa, ok := path2[i].(ast.Stmt); ok {
				if _, ok := aa.(*ast.LabeledStmt); ok {
					index2 = append(index2, i-1)
				}
				path2Stmt = append(path2Stmt, aa)
			}
		}
		if len(index1) != 0 {
			for i, a := range path1Stmt {
				for _, b := range index1 {
					if i != b {
						path11 = append(path11, a)
					}
				}
			}
		} else {
			for _, a := range path1Stmt {
				path11 = append(path11, a)
			}
		}
		if len(index2) != 0 {
			for i, a := range path2Stmt {
				for _, b := range index2 {
					if i != b {
						path22 = append(path22, a)
					}
				}
			}
		} else {
			for _, a := range path2Stmt {
				path22 = append(path22, a)
			}
		}

	}
	return sliceCompare(path11, path22)
}
func sliceCompare(path1 []ast.Stmt, path2 []ast.Stmt) bool {
	var result bool = false
	if len(path1) == len(path2) {
		for i := 1; i < len(path1); i++ {
			if path2[i] == path1[i] {
				result = true
			} else {
				result = false
			}
		}
	}
	return result
}

// this function parses through the extracted code,
// - performs the 'Single Entry/Single Exit' condition
// - performs Live Variable analysis on the extracted code to get the list of variables that are :
// 				1. Passed as arguments
// 				2. Returned as variables
// 				3. Declared as variables in the new function

func (r *ExtractFunc) parseCode(node *ast.BlockStmt, path []ast.Node) ([]*types.Var, []*types.Var, []*types.Var, bool) {
	var paramVarList []*types.Var
	var returnVarList []*types.Var
	var varDecl []*types.Var
	var assign []*types.Var
	var defined []*types.Var
	var initVars []*types.Var
	var funcCFG *cfg.CFG
	var defArr []*types.Var
	for _, n := range path {
		switch a := n.(type) {
		case *ast.FuncDecl:
			funcCFG = cfg.FromFunc(a)
		}
	}
	breakConditionCheck, stmtArr, InitStmts := r.checkEntryExitCondtion(funcCFG, node) // returns the initStmt slice as well
	valid := r.checkValidSelection(stmtArr)
	for _, stmt := range InitStmts { // initStatment conversion to variables
		switch a := stmt.(type) {
		case *ast.AssignStmt:
			for _, v := range a.Lhs { // get the list of variables that are declared in the init of for loop
				switch tempv := v.(type) {
				case *ast.Ident:
					if i, ok := r.SelectedNodePkg.ObjectOf(tempv).(*types.Var); ok {
						initVars = append(initVars, i)
					}
				case *ast.IndexExpr:
					if i, ok := r.SelectedNodePkg.ObjectOf(tempv.X.(*ast.Ident)).(*types.Var); ok { // name of the array
						initVars = append(initVars, i)
					}
					if i, ok := r.SelectedNodePkg.ObjectOf(tempv.Index.(*ast.Ident)).(*types.Var); ok { // index variable
						initVars = append(initVars, i)
					}
				case *ast.StarExpr:
					if i, ok := r.SelectedNodePkg.ObjectOf(tempv.X.(*ast.Ident)).(*types.Var); ok {
						initVars = append(initVars, i)
					}
				}
			}
		case *ast.RangeStmt: // HERE WHEN YOU COME ACROSS RANGE STATEMENTS, THOSE VARIABLES TOTHE LEFT OF THE := ARE PART OF THE INIT STATATEMENTS
			if i, ok := r.SelectedNodePkg.ObjectOf(a.Key.(*ast.Ident)).(*types.Var); ok {
				if i.Name() != "_" {
					initVars = append(initVars, i)
				}
			}
			if i, ok := r.SelectedNodePkg.ObjectOf(a.Value.(*ast.Ident)).(*types.Var); ok {
				if i.Name() != "_" {
					initVars = append(initVars, i)
				}
			}
		}
	}
	sort.Sort(typeVar(initVars))
	if breakConditionCheck && valid == true {
		aliveFirst := r.returnEntryLiveVar(funcCFG, node)
		useArr := r.returnUse(stmtArr)
		assign = r.returnAssigned(stmtArr, r.SelectedNodePkg)
		defined = r.returnDefined(stmtArr, r.SelectedNodePkg)
		defArr = r.removeDuplicates(unionOp(assign, defined))
		aliveLast := r.returnExitLiveVar(stmtArr, funcCFG, node)
		aliveFirst = differenceOp(aliveFirst, initVars) // incase of a for loop
		aliveLast = differenceOp(aliveLast, initVars)
		paramVarList, _ = isIntersection(aliveFirst, useArr) // Params = LIVE_IN[Entry(selection node)] INTERSECTION USE[selection] //--original
		returnVarList, _ = isIntersection(aliveLast, defArr) // returns = LIVE_OUT[exit(sel)] INTERSECTION DEF[sel]
		varDecl = unionOp(differenceOp(assign, paramVarList), differenceOp(useArr, aliveFirst))
		varDecl = differenceOp(varDecl, initVars)

	} else {
		r.Log.Error("The code cannot be extracted since the 'Single Entry/Single Exit' conditon failed (OR) a Valid selection wasn't made")
	}
	return paramVarList, returnVarList, differenceOp(varDecl, defined), r.setShortAssignmentFlag(stmtArr, returnVarList)
}

// this function removes the duplicates from the variable list that is passed into it
func (r *ExtractFunc) removeDuplicates(varList []*types.Var) []*types.Var {
	found := make(map[*types.Var]bool)
	var result []*types.Var
	for _, x := range varList {
		if !found[x] {
			found[x] = true
			result = append(result, x)
		}
	}
	sort.Sort(typeVar(result))
	return result
}

//this function parses through the funcCFG.Blocks() and returns the correct list of statements that are extracted
//- if the stmtArr contains just one node that is an *ast.AssignStmt which is a part of the Init of the different arguments, then throw an error

func (r *ExtractFunc) returnStmtArray(funcCFG *cfg.CFG, node *ast.BlockStmt) ([]ast.Stmt, []ast.Stmt) {
	var stmtArr []ast.Stmt
	var InitStmts []ast.Stmt
	endStmt := r.findEndPosition(node)
	startStmt := r.findStartPosition(node)
	for _, s := range funcCFG.Blocks() {
		if s.Pos() >= startStmt.Pos() && s.End() <= (endStmt.End()) {
			switch x := s.(type) {
			case *ast.IfStmt:
				if x.Init != nil {
					InitStmts = append(InitStmts, x.Init)
				}
				stmtArr = append(stmtArr, x)
			case *ast.SwitchStmt:
				if x.Init != nil {
					InitStmts = append(InitStmts, x.Init)
				}
				stmtArr = append(stmtArr, x)
			case *ast.TypeSwitchStmt:
				if x.Assign != nil {
					InitStmts = append(InitStmts, x.Assign)
					if x.Init != nil {
						InitStmts = append(InitStmts, x.Init)
					}
				}
				stmtArr = append(stmtArr, x)
			case *ast.ForStmt:
				if x.Init != nil { // what about range statements
					InitStmts = append(InitStmts, x.Init)
				}
				stmtArr = append(stmtArr, x)
			case *ast.RangeStmt:
				InitStmts = append(InitStmts, x) // the entire range statement becomes a part of initstatement list,
				// the left side of the := becomes a part of the init statement
				stmtArr = append(stmtArr, x)
			case *ast.CommClause:
				if x.Comm != nil {
					InitStmts = append(InitStmts, x.Comm)
				}
				stmtArr = append(stmtArr, x)
			case *ast.LabeledStmt:
				stmtArr = append(stmtArr, x)
				stmtArr = append(stmtArr, x.Stmt)
			case *ast.ReturnStmt:
				r.Log.Error("RETURN statement cannot be extracted.")
			default:
				stmtArr = append(stmtArr, x)
			}
		}
	}
	if len(stmtArr) == 1 {
		if _, ok := stmtArr[0].(*ast.AssignStmt); ok {
			path, _ := astutil.PathEnclosingInterval(r.File, stmtArr[0].Pos(), stmtArr[0].End())
			switch path[1].(type) {
			case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.ForStmt, *ast.CommClause:
				r.Log.Error("The Assignment statement extracted is not a valid statement ")
			}
		}
	}
	return stmtArr, InitStmts
}

func (r *ExtractFunc) compareStmt(a *ast.AssignStmt, InitStmts []ast.Stmt) bool {
	flag := true
	for _, b := range InitStmts {
		if a == b {
			r.Log.Error("The Assignment statement extracted is not a valid statement ")
			flag = false
		}
	}
	return flag
}

// Check the extracted code for the Single Entry and Single Exit Criteria :
// 1. len(entry[SEL]).Preds == 1 must be TRUE for the first statement of the extracted code
// 2. len(exit[SEL]).Succs  == 1 must be TRUE for the last statement of the extracted code
func (r *ExtractFunc) checkEntryExitCondtion(funcCFG *cfg.CFG, node *ast.BlockStmt) (bool, []ast.Stmt, []ast.Stmt) {
	endStmt := r.findEndPosition(node)
	startStmt := r.findStartPosition(node)
	stmtArr, InitStmts := r.returnStmtArray(funcCFG, node)
	if funcCFG.Defers != nil {
		for _, d := range funcCFG.Defers {
			if d.Pos() >= startStmt.Pos() && d.End() <= (endStmt.End()) {
				r.Log.Errorf("A defer statement cannot be extracted")
				r.Log.AssociateNode(d)
			}
		}
	}
	sort.Sort(nodeStmt(stmtArr))
	breakConditionCheck := r.checkBranchCondition(stmtArr)
	return breakConditionCheck, stmtArr, InitStmts
}

// when *ast.BranchStmt is encountered,check if it is 'goto','break','continue'and 'fallthrough' all may or maynot have LABEL,
// 1. if they have LABEL, then the LABELED Stmt must be a part of the extracted code
// 2. if BREAK STATEMENT is encountered then it should be inside a 'for','switch','select' statement block
// 3. if CONTINUE stmt then it should be inside a 'for' statement block
// 4. if GOTO stmt is encountered then it must have LABELED stmt along with it
// 5. When *ast.ReturnStmt is encountered, throw error
func (r *ExtractFunc) checkBranchCondition(stmtArr []ast.Stmt) bool {
	var xLabelArr []string
	var zLabelArr []string
	var exitValid bool = false
	for i, n := range stmtArr {
		switch x := n.(type) {
		case *ast.BranchStmt:
			switch x.Tok {
			case token.BREAK:
				exitValid = r.handleBreakStmt(x, stmtArr, i)
				break
			case token.CONTINUE:
				exitValid = r.handleContinueStmt(x, stmtArr, i)
				break
			case token.GOTO:
				xLabelArr, zLabelArr = r.handleGotoStmt(x, stmtArr, i)
				break
			}
		case *ast.ReturnStmt:
			exitValid = false
			r.Log.Error("The RETURN statement is not extracted right")
			r.Log.AssociateNode(x)
		default:
			continue
		}
	}
	if r.labelComparison(xLabelArr, zLabelArr) == false {
		r.Log.Error("The Single Entry/Single Exit condition failed in goto statement")
		exitValid = false
	} else {
		exitValid = true
	}
	return exitValid
}

// checks for 'for','switch','TypeSwitch' and 'select' statements before of the break statement
// For break Label statements, checks if the Label statement is the part of the extracted code
func (r *ExtractFunc) handleBreakStmt(x *ast.BranchStmt, stmtArr []ast.Stmt, i int) bool {
	var exitValid bool = false
	var errorFlag bool = false
	if x.Label == nil { // break without label
		for j := i; j >= 0; j-- {
			switch stmtArr[j].(type) {
			case *ast.ForStmt, *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
				exitValid = true
				errorFlag = false
				break
			default:
				errorFlag = true
				continue
			}
		}
	} else {
		for j := i + 1; j >= 0; j-- {
			switch s := stmtArr[j].(type) {
			case *ast.LabeledStmt:
				if x.Label.Name == s.Label.Name {
					exitValid = true
					errorFlag = false
					break
				} else {
					exitValid = false
					r.Log.Error("The BREAK's label doesn't match the LabeledStmt")
					r.Log.AssociateNode(x)
					break
				}
			default:
				errorFlag = true
				continue
			}
		}
	}
	if errorFlag == true {
		exitValid = false
		r.Log.Error("The BREAK statement is not extracted right")
		r.Log.AssociateNode(x)
		exitValid = false
	}
	return exitValid
}

// checks for 'for' statements before of the continue statement
// For continue Label statements, checks if the Label statement is the part of the extracted code
func (r *ExtractFunc) handleContinueStmt(x *ast.BranchStmt, stmtArr []ast.Stmt, i int) bool {
	var exitValid bool = false
	var errorFlag bool = false
	if x.Label == nil { // CONTINUE without label
		for j := i + 1; j >= 0; j-- {
			switch stmtArr[j].(type) {
			case *ast.ForStmt:
				exitValid = true
				errorFlag = false
				break
			default:
				errorFlag = true
				continue
			}
		}
	} else {
		for j := i + 1; j >= 0; j-- {
			switch s := stmtArr[j].(type) {
			case *ast.LabeledStmt:
				if x.Label.Name == s.Label.Name {
					exitValid = true
					errorFlag = false
					break
				} else {
					exitValid = false
					r.Log.Error("The Continue statement's label doesn't match the LabeledStmt")
					r.Log.AssociateNode(x)
					break
				}
			default:
				errorFlag = true
				continue
			}
		}
	}
	if errorFlag == true {
		exitValid = false
		r.Log.Error("The CONTINUE statement is not extracted right")
		r.Log.AssociateNode(x)
	}
	return exitValid
}

// Handles GOTO statements in the extracted code and verifies if the associated Label statement is present
func (r *ExtractFunc) handleGotoStmt(x *ast.BranchStmt, stmtArr []ast.Stmt, i int) ([]string, []string) {
	var xLabelArr []string
	var zLabelArr []string
	xLabelArr = append(xLabelArr, x.Label.Name)
	for j := 0; j < len(stmtArr); j++ {
		switch z := stmtArr[j].(type) {
		case *ast.LabeledStmt: // add these to a set of statement array and then do the comparison
			if x.Label.Name == z.Label.Name {
				zLabelArr = append(zLabelArr, z.Label.Name)
			}
		}
	}
	return xLabelArr, zLabelArr
}

//Compares Labels in the GOTO statement and the Label Statments
func (r *ExtractFunc) labelComparison(xLabelArr, zLabelArr []string) bool {
	if len(xLabelArr) != len(zLabelArr) {
		return false
	} else {
		return true
	}
}

// When returning arguments from the new function, based on the variables returned and those defined in the extracted code,
// we check if the returned variables is defined in the new code, if so then ':=' must be used while returning variables
func (r *ExtractFunc) setShortAssignmentFlag(stmtArr []ast.Stmt, returnVarList []*types.Var) bool {
	var flagShortAssign bool = false
	var define []*types.Var
	define = r.returnDefined(stmtArr, r.SelectedNodePkg)
	if _, ok := isIntersection(returnVarList, define); ok {
		flagShortAssign = true
	}
	return flagShortAssign
}

// returns the defs and uses variables from the analysis code
func (r *ExtractFunc) returnUse(stmtArr []ast.Stmt) []*types.Var {
	_, use := dataflow.ReferencedVars(stmtArr, r.SelectedNodePkg) // passing only the extracted statements
	var useArr []*types.Var
	for key, _ := range use {
		if !key.IsField() { // removes the field Vars
			useArr = append(useArr, key)
		}
	}
	sort.Sort(typeVar(useArr))

	return useArr
}

// returns variables that are live at the first line of the extracted code
func (r *ExtractFunc) returnEntryLiveVar(funcCFG *cfg.CFG, node *ast.BlockStmt) []*types.Var {
	var aliveFirst []*types.Var
	in, _ := dataflow.LiveVars(funcCFG, r.SelectedNodePkg)

	startStmt := r.findStartPosition(node)
	for key, value := range in {
		if key.Pos() == startStmt.Pos() { // something is wrong figure it our
			for valKey, _ := range value {
				if !valKey.IsField() {
					aliveFirst = append(aliveFirst, valKey)
				}
			}
		}
	}
	sort.Sort(typeVar(aliveFirst))
	return aliveFirst
}

// returns variables that rae live at the last line of the extracted code
func (r *ExtractFunc) returnExitLiveVar(stmtArr []ast.Stmt, funcCFG *cfg.CFG, node *ast.BlockStmt) []*types.Var {
	var aliveLast []*types.Var
	var flagForExit bool = false
	in, out := dataflow.LiveVars(funcCFG, r.SelectedNodePkg)
	sort.Sort(nodeEnd(stmtArr))
	for key, value := range out {
		if key.End() == stmtArr[len(stmtArr)-1].End() {
			switch key.(type) {
			case *ast.ForStmt, *ast.LabeledStmt, *ast.RangeStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				flagForExit = true
				break
			default:
				for valKey, _ := range value {
					if !valKey.IsField() {
						aliveLast = append(aliveLast, valKey)
					}
				}
			}
		}
	}
	_, afterLast := r.findBeforeAfterNodes(funcCFG, node)
	if flagForExit == true { // when a forloop is encountered in the end of the extracted statement
		for key, value := range in { // we focus on the LIVE_IN of the stmt after the last statement
			if key == afterLast {
				for valKey, _ := range value {
					aliveLast = append(aliveLast, valKey)
				}
			}
		}
	}
	sort.Sort(typeVar(aliveLast))
	return aliveLast
}

//returns the nodes that are immedeatly
// - before the first node of the extracted code
// - after the last node of the extracted code
func (r *ExtractFunc) findBeforeAfterNodes(funcCFG *cfg.CFG, node *ast.BlockStmt) (ast.Stmt, ast.Stmt) {
	var beforeFirst ast.Stmt
	var afterLast ast.Stmt
	var Arr []ast.Stmt
	endStmt := r.findEndPosition(node)
	startStmt := r.findStartPosition(node)
	for _, s := range funcCFG.Blocks() {
		Arr = append(Arr, s)
	}
	sort.Sort(nodeStmt(Arr))
	for i := 1; i < len(Arr); i++ {
		if Arr[i].Pos() == startStmt.Pos() {
			beforeFirst = Arr[i-1]
			break
		}
	}
	sort.Sort(nodeEnd(Arr))
	for i := 0; i < len(Arr); i++ {
		if Arr[i].Pos() > endStmt.End()+1 {
			afterLast = Arr[i]
			break
		}
	}
	if afterLast == nil {
		afterLast = funcCFG.Exit
	} else if beforeFirst == nil {
		beforeFirst = funcCFG.Entry
	}
	return beforeFirst, afterLast
}

//This creates the editstring for
//	1. function call
//	2. function definition
func (r *ExtractFunc) createEditString(varDecString, passstr, funcdefparamstr, retstr string, retfuncdefparamstr string,
	retVarLen int, readStr []byte, receieverName, receiverType string, flagShortAssign bool) (string, string) {
	var replacementStr string
	var funcCallStr string
	var varStr string = ""
	var retString string = ""
	var assignSymbol string = " = "
	var bracketStr string = fmt.Sprintf("%s()", r.funcName)
	var funcDefStr string = fmt.Sprintf("%s()", r.funcName)
	if flagShortAssign == true {
		// if(receiver is the only variable being returned... change the assign symbol to '=')
		assignSymbol = " := "
	}
	if varDecString != "" {
		varStr = varDecString
	}
	if retstr != "" {
		retString = "return "
	}
	if receieverName != "" && receiverType != "" {
		bracketStr = fmt.Sprintf("%s.%s()", receieverName, r.funcName)
		funcDefStr = fmt.Sprintf("(%s %s) %s()", receieverName, receiverType, r.funcName)
		if passstr != "" {
			bracketStr = fmt.Sprintf("%s.%s(%s)", receieverName, r.funcName, passstr)
			funcDefStr = fmt.Sprintf("(%s %s) %s(%s)", receieverName, receiverType, r.funcName, funcdefparamstr)
		}
		if retstr == "" {
			replacementStr = fmt.Sprintf("\nfunc %s {\n%s%s\n}\n", funcDefStr, varStr, readStr)
			funcCallStr = fmt.Sprintf("%s", bracketStr)
		} else {
			if retVarLen > 1 {
				replacementStr = fmt.Sprintf("\nfunc %s(%s) {\n%s%s\n%s%s\n}\n",
					funcDefStr, retfuncdefparamstr, varStr, readStr, retString, retstr)
				funcCallStr = fmt.Sprintf("%s%s%s", retstr, assignSymbol, bracketStr)
			} else {
				replacementStr = fmt.Sprintf("\nfunc %s %s {\n%s%s\n%s%s\n}\n",
					funcDefStr, retfuncdefparamstr, varStr, readStr, retString, retstr)
				funcCallStr = fmt.Sprintf("%s%s%s", retstr, assignSymbol, bracketStr)
			}
		}
	} else {
		if passstr != "" {
			bracketStr = fmt.Sprintf("%s(%s)", r.funcName, passstr)
			funcDefStr = fmt.Sprintf("%s(%s)", r.funcName, funcdefparamstr)
		}
		if retstr == "" {
			replacementStr = fmt.Sprintf("\nfunc %s {\n%s%s\n}\n", funcDefStr, varStr, readStr)
			funcCallStr = fmt.Sprintf("%s", bracketStr)
		} else {
			if retVarLen > 1 {
				replacementStr = fmt.Sprintf("\nfunc %s(%s) {\n%s%s\n%s%s\n}\n",
					funcDefStr, retfuncdefparamstr, varStr, readStr, retString, retstr)
				funcCallStr = fmt.Sprintf("%s%s%s", retstr, assignSymbol, bracketStr)
			} else {
				replacementStr = fmt.Sprintf("\nfunc %s %s {\n%s%s\n%s%s\n}\n",
					funcDefStr, retfuncdefparamstr, varStr, readStr, retString, retstr)
				funcCallStr = fmt.Sprintf("%s%s%s", retstr, assignSymbol, bracketStr)
			}
		}
	}
	return replacementStr, funcCallStr
}

func (r *ExtractFunc) createString(strarr []string, typearr []string, flagVar bool) string {
	var buf bytes.Buffer
	if flagVar == true {
		for k := 0; k < len(strarr); k++ {
			buf.WriteString("var " + strarr[k] + " " + typearr[k])
			if k > 1 || k <= len(strarr)-1 {
				buf.WriteString("\n")
			}
		}
	} else {
		if typearr == nil {
			for k := 0; k < len(strarr); k++ {
				buf.WriteString(strarr[k])
				if k == len(strarr)-1 {
					break
				}
				if k > 1 || k < len(strarr)-1 {
					buf.WriteString(", ")
				}
			}
		} else if strarr == nil {
			for k := 0; k < len(typearr); k++ {
				buf.WriteString(typearr[k])
				if k == len(typearr)-1 {
					break
				}
				if k > 1 || k < len(typearr)-1 {
					buf.WriteString(", ")
				}
			}
		} else {
			for k := 0; k < len(strarr); k++ {
				buf.WriteString(strarr[k] + " " + typearr[k])
				if k > 1 || k < len(strarr)-1 {
					buf.WriteString(", ")
				}
			}

		}
	}
	return buf.String()
}

func isIntersection(s1 []*types.Var, s2 []*types.Var) ([]*types.Var, bool) {
	var result []*types.Var
	var flag bool = false
	for i := 0; i < len(s2); i++ {
		for j := 0; j < len(s1); j++ {
			if s2[i].Name() == s1[j].Name() { //&& s2[i].Type() == s1[i].Type() { // when trying to make sure they have the same variables types, throws error ????
				flag = true
				result = append(result, s2[i])
				break
			} else {
				flag = false
			}
		}
	}
	return result, flag
}

func unionOp(v1, v2 []*types.Var) []*types.Var {
	insec, _ := isIntersection(v1, v2) // check for duplicates and removes them
	for _, a := range v2 {
		v1 = append(v1, a)
	}
	v1 = differenceOp(v1, insec)
	for _, b := range insec {
		v1 = append(v1, b) // adding back the variables to the array only once
	}
	return v1
}

func differenceOp(use, in []*types.Var) []*types.Var {
	var flag bool
	var result []*types.Var
	for i := 0; i < len(use); i++ {
		flag = false
		for j := 0; j < len(in); j++ {
			if use[i].Name() == in[j].Name() || use[i].Name() == "_" {
				flag = true
				break
			}
		}
		if flag == false {
			result = append(result, use[i])
		}
	}
	return result
}

func (r *ExtractFunc) isIdentifierValid(newName string) bool {
	matched, err := regexp.MatchString("^[A-Za-z_][0-9A-Za-z_]*$", newName)
	if matched && err == nil {
		return true
	}
	return false
}

// sorting []*types.Var variables based on the Names()
type typeVar []*types.Var

func (t typeVar) Len() int           { return len(t) }
func (t typeVar) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t typeVar) Less(i, j int) bool { return t[i].Name() < t[j].Name() }

// sorting nodes based on positions
type nodeStmt []ast.Stmt

func (n nodeStmt) Len() int           { return len(n) }
func (n nodeStmt) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n nodeStmt) Less(i, j int) bool { return n[i].Pos() < n[j].Pos() }

// sorting nodes based on End()
type nodeEnd []ast.Stmt

func (n nodeEnd) Len() int           { return len(n) }
func (n nodeEnd) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n nodeEnd) Less(i, j int) bool { return n[i].End() < n[j].End() }

// part of DataFlow analysis
func union(one, two map[*ast.Ident]struct{}) map[*ast.Ident]struct{} {
	for o, _ := range one {
		two[o] = struct{}{}
	}
	return two
}

func idents(node ast.Node) map[*ast.Ident]struct{} {
	idents := make(map[*ast.Ident]struct{})
	if node == nil {
		return idents
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			idents[n] = struct{}{}
		}
		return true
	})
	return idents
}

func (r *ExtractFunc) returnAssigned(stmtArr []ast.Stmt, info *loader.PackageInfo) []*types.Var {
	// defAssign := make(map[*types.Var]struct{})
	var assign []*types.Var
	for _, stmt := range stmtArr {
		for _, d := range r.Assigned(stmt, info) {
			assign = append(assign, d)
		}
	}
	return assign
}

// this returns the variables that are only ASSIGNED using 'i++/i--' or '='
func (r *ExtractFunc) Assigned(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	idntsAssign := make(map[*ast.Ident]struct{})
	switch stmt := stmt.(type) {
	case *ast.AssignStmt: // for statements with '='
		if stmt.Tok != token.DEFINE || stmt.Tok != token.AND_ASSIGN {
			for _, x := range stmt.Lhs {
				indExp := false
				switch x.(type) {
				case *ast.IndexExpr:
					indExp = true
				case *ast.SelectorExpr:
					//indExp = true // changed
					indExp = false
				}
				if !indExp {
					idntsAssign = union(idntsAssign, idents(x))
				}
			}
		}
	case *ast.IncDecStmt: // i++, i--
		if _, ok := stmt.X.(*ast.SelectorExpr); !ok {
			idntsAssign = idents(stmt.X)
		}
	}
	var varsAssign []*types.Var
	for i, _ := range idntsAssign {
		if v, ok := info.ObjectOf(i).(*types.Var); ok {
			if !v.IsField() && v.Name() != "_" {
				varsAssign = append(varsAssign, v)
			}
		}
	}
	return varsAssign
}

func (r *ExtractFunc) returnDefined(stmtArr []ast.Stmt, info *loader.PackageInfo) []*types.Var {

	var define []*types.Var
	for _, stmt := range stmtArr {
		for _, d := range r.Defined(stmt, info) {
			define = append(define, d)
		}
	}
	return define
}

// this returns the variables that are only ASSIGNED using 'i++/i--' or '='
func (r *ExtractFunc) Defined(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	idnts := make(map[*ast.Ident]struct{})

	switch stmt := stmt.(type) {
	case *ast.DeclStmt: // vars (1+) in decl; zero values
		ast.Inspect(stmt, func(n ast.Node) bool {
			if v, ok := n.(*ast.ValueSpec); ok {
				idnts = union(idnts, idents(v))
			}
			return true
		})
	case *ast.AssignStmt: // :=, &= are the only operators that define
		if stmt.Tok == token.DEFINE || stmt.Tok == token.AND_ASSIGN {
			for _, x := range stmt.Lhs {
				indExp := false
				switch x.(type) {
				case *ast.IndexExpr:
					indExp = true
				case *ast.SelectorExpr:
					indExp = false
				}
				if !indExp {
					idnts = union(idnts, idents(x))
				}
			}
		}
	case *ast.RangeStmt: // only [ x, y ] on Lhs
		idnts = union(idents(stmt.Key), idents(stmt.Value))
	case *ast.TypeSwitchStmt:
		// The assigned variable does not have a types.Var
		// associated in this stmt; rather, the uses of that
		// variable in the case clauses have several different
		// types.Vars associated with them, according to type
		var vars []*types.Var
		ast.Inspect(stmt.Body, func(n ast.Node) bool {
			switch cc := n.(type) {
			case *ast.CaseClause:
				v := typeCaseVar(info, cc)
				if v != nil {
					vars = append(vars, v)
				}
				return false
			default:
				return true
			}
		})
		return vars
	}

	var vars []*types.Var
	// should all map to types.Var's, if not we don't want anyway
	for i, _ := range idnts {
		if v, ok := info.ObjectOf(i).(*types.Var); ok {
			if !v.IsField() && v.Name() != "_" {
				vars = append(vars, v)
			}
		}
	}
	return vars
}

// typeCaseVar returns the implicit variable associated with a case clause in a
// type switch statement.
func typeCaseVar(info *loader.PackageInfo, cc *ast.CaseClause) *types.Var {
	// Removed from go/loader
	if v := info.Implicits[cc]; v != nil {
		return v.(*types.Var)
	}
	return nil
}

// This file implements printing of types.

var GcCompatibilityMode bool

// TypeString returns the string representation of typ.
// Named types are printed package-qualified if they
// do not belong to this package.
func (r *ExtractFunc) TypeString(this *types.Package, typ types.Type) string {
	var buf bytes.Buffer
	r.WriteType(&buf, this, typ)
	return buf.String()
}

// WriteType writes the string representation of typ to buf.
// Named types are printed package-qualified if they
// do not belong to this package.
func (r *ExtractFunc) WriteType(buf *bytes.Buffer, this *types.Package, typ types.Type) {
	r.writeType(buf, this, typ, make([]types.Type, 8))
}

func (r *ExtractFunc) writeType(buf *bytes.Buffer, this *types.Package, typ types.Type, visited []types.Type) {
	// Theoretically, this is a quadratic lookup algorithm, but in
	// practice deeply nested composite types with unnamed component
	// types are uncommon. This code is likely more efficient than
	// using a map.
	var structFields []*types.Var
	var allMethods []*types.Func
	var expMethods []*types.Func
	var embeds []*types.Named
	for _, t := range visited {
		if t == typ {
			fmt.Fprintf(buf, "○%T", typ) // cycle to typ
			return
		}
	}
	visited = append(visited, typ)
	switch t := typ.(type) {
	case nil:
		buf.WriteString("<nil>")

	case *types.Basic:
		if t.Kind() == types.UnsafePointer {
			buf.WriteString("unsafe.")
		}
		if GcCompatibilityMode {
			// forget the alias names
			switch t.Kind() { // -- > changes
			case types.Byte:
				t = types.Typ[types.Uint8]
			case types.Rune:
				t = types.Typ[types.Int32]
			}
		}
		buf.WriteString(t.Name())

	case *types.Array:
		fmt.Fprintf(buf, "[%d]", t.Len())
		r.writeType(buf, this, t.Elem(), visited)

	case *types.Slice:
		buf.WriteString("[]")
		r.writeType(buf, this, t.Elem(), visited)

	case *types.Struct:
		buf.WriteString("struct{")
		for i := 0; i < t.NumFields(); i++ {
			structFields = append(structFields, t.Field(i))
		}
		// for i, f := range t.fields {
		for i, f := range structFields {
			if i > 0 {
				buf.WriteString("; ")
			}
			if !f.Anonymous() {
				buf.WriteString(f.Name())
				buf.WriteByte(' ')
			}
			// writeType(buf, this, f.typ, visited)
			r.writeType(buf, this, f.Type(), visited)
			if tag := t.Tag(i); tag != "" {
				fmt.Fprintf(buf, " %q", tag)
			}
		}

		buf.WriteByte('}')

	case *types.Pointer:
		buf.WriteByte('*')
		r.writeType(buf, this, t.Elem(), visited)

	case *types.Tuple:
		r.writeTuple(buf, this, t, false, visited)

	case *types.Signature:
		buf.WriteString("func")
		r.writeSignature(buf, this, t, visited)

	case *types.Interface:
		// We write the source-level methods and embedded types rather
		// than the actual method set since resolved method signatures
		// may have non-printable cycles if parameters have anonymous
		// interface types that (directly or indirectly) embed the
		// current interface. For instance, consider the result type
		// of m:
		//
		//     type T interface{
		//         m() interface{ T }
		//     }
		//

		buf.WriteString("interface{")
		if GcCompatibilityMode {
			// print flattened interface
			// (useful to compare against gc-generated interfaces)
			for i := 0; i < t.NumMethods(); i++ {
				allMethods = append(allMethods, t.Method(i))
			}
			for i, m := range allMethods {
				if i > 0 {
					buf.WriteString("; ")
				}
				buf.WriteString(m.Name())
				r.writeSignature(buf, this, m.Type().(*types.Signature), visited)
			}
		} else {
			// print explicit interface methods and embedded types
			for i := 0; i < t.NumExplicitMethods(); i++ {
				expMethods = append(expMethods, t.ExplicitMethod(i))
			}
			for i := 0; i < t.NumEmbeddeds(); i++ {
				embeds = append(embeds, t.Embedded(i))
			}
			for i, m := range expMethods {
				if i > 0 {
					buf.WriteString("; ")
				}
				buf.WriteString(m.Name())
				r.writeSignature(buf, this, m.Type().(*types.Signature), visited)
			}
			for i, typ := range embeds {
				if i > 0 || t.NumMethods() > 0 {
					buf.WriteString("; ")
				}
				r.writeType(buf, this, typ, visited)
			}
		}
		buf.WriteByte('}')

	case *types.Map:
		buf.WriteString("map[")
		r.writeType(buf, this, t.Key(), visited)
		buf.WriteByte(']')
		r.writeType(buf, this, t.Elem(), visited)

	case *types.Chan:
		var s string
		var parens bool
		switch t.Dir() {
		case types.SendRecv:
			s = "chan "
			if c, _ := t.Elem().(*types.Chan); c != nil && c.Dir() == types.RecvOnly {
				parens = true
			}
		case types.SendOnly:
			s = "chan<- "
		case types.RecvOnly:
			s = "<-chan "
		default:
			panic("unreachable")
		}
		buf.WriteString(s)
		if parens {
			buf.WriteByte('(')
		}
		r.writeType(buf, this, t.Elem(), visited)
		if parens {
			buf.WriteByte(')')
		}

	case *types.Named:
		s := "<Named w/o object>"
		if obj := t.Obj(); obj != nil {
			if pkg := obj.Pkg(); pkg != nil && pkg != this {
				buf.WriteString(path.Base(pkg.Path()))
				buf.WriteByte('.')
			}
			s = obj.Name()
		}
		buf.WriteString(s)

	default:
		// For externally defined implementations of Type.
		buf.WriteString(t.String())
	}
}

func (r *ExtractFunc) writeTuple(buf *bytes.Buffer, this *types.Package, tup *types.Tuple, variadic bool, visited []types.Type) {
	var tuplesVar []*types.Var

	buf.WriteByte('(')
	if tup != nil {
		for i := 0; i < tup.Len(); i++ {
			tuplesVar = append(tuplesVar, tup.At(i))
		}
		for i, v := range tuplesVar {
			if i > 0 {
				buf.WriteString(", ")
			}
			if v.Name() != "" {
				buf.WriteString(v.Name())
				buf.WriteByte(' ')
			}
			typ := v.Type()
			if variadic && i == tup.Len()-1 {
				if s, ok := typ.(*types.Slice); ok {
					buf.WriteString("...")
					typ = s.Elem()
				} else {
					r.writeType(buf, this, typ, visited)
					buf.WriteString("...")
					continue
				}
			}
			r.writeType(buf, this, typ, visited)
		}
	}
	buf.WriteByte(')')
}

// WriteSignature writes the representation of the signature sig to buf,
// without a leading "func" keyword.
// Named types are printed package-qualified if they
// do not belong to this package.
func (r *ExtractFunc) WriteSignature(buf *bytes.Buffer, this *types.Package, sig *types.Signature) {
	r.writeSignature(buf, this, sig, make([]types.Type, 8))
}

func (r *ExtractFunc) writeSignature(buf *bytes.Buffer, this *types.Package, sig *types.Signature, visited []types.Type) {
	r.writeTuple(buf, this, sig.Params(), sig.Variadic(), visited)
	n := sig.Results().Len()
	if n == 0 {
		// no result
		return
	}
	buf.WriteByte(' ')
	if n == 1 && sig.Results().At(0).Name() == "" {
		// single unnamed result
		r.writeType(buf, this, sig.Results().At(0).Type(), visited)
		return
	}
	// multiple or named result(s)
	r.writeTuple(buf, this, sig.Results(), false, visited)
}
