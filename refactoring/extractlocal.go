// Copyright 2014-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/text"

	"golang.org/x/tools/go/ast/astutil"
)

type ExtractLocal struct {
	RefactoringBase
	varName        string
	enclosingNodes []ast.Node
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

	r.enclosingNodes, _ = astutil.PathEnclosingInterval(r.File,
		r.SelectedNode.Pos(),
		r.SelectedNode.End())
	//for i, node := range r.enclosingNodes {
	//	fmt.Printf("%d: %s\n", i, reflect.TypeOf(node))
	//}

	var insertBefore ast.Stmt
	if r.checkSelectedNodeIsExpr() &&
		r.checkExpressionType() &&
		r.checkExpressionContext() &&
		r.checkStatementContext(&insertBefore) {
		r.addEdits(insertBefore)
		r.FormatFileInEditor()
		r.UpdateLog(config, false)
	}
	return &r.Result
}

// checkSelectedNodeIsExpr checks that the user has selected an expression,
// logging an error and returning false iff it does not.
//
// If this function returns true, r.SelectedNode.(ast.Expr) can be asserted.
func (r *ExtractLocal) checkSelectedNodeIsExpr() bool {
	if r.SelectedNode == nil {
		r.Log.Error("Please select an expression to extract.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	if _, ok := r.SelectedNode.(ast.Expr); !ok {
		r.Log.Error("Please select an expression to extract.")
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	return true
}

// checkExpressionType determines the type of the selected expression and
// determines whether it can be assigned to a variable, logging an error and
// returning false if it cannot.
func (r *ExtractLocal) checkExpressionType() bool {
	exprType := r.SelectedNodePkg.TypeOf(r.SelectedNode.(ast.Expr))

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

	return true
}

// checkExpressionContext looks at the enclosing expressions, if any, and
// determines if the selected expression is a subexpression that cannot be
// extracted, logging an error and returning false if it cannot.
func (r *ExtractLocal) checkExpressionContext() bool {
	parentNode := r.enclosingNodes[1]
	if selectorExpr, ok := parentNode.(*ast.SelectorExpr); ok {
		if selectorExpr.Sel == r.SelectedNode {
			r.Log.Error("A field selector cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	// This isn't completely correct, since &((((x)))) also takes an address,
	// but it's close enough for now
	if unary, ok := parentNode.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		r.Log.Error("An expression cannot be extracted if its address is taken.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	for _, node := range r.enclosingNodes {
		if isTypeNode(node) {
			r.Log.Error("An expression used to specify a type cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	for i, node := range r.enclosingNodes {
		if kv, ok := node.(*ast.KeyValueExpr); ok && i > 0 && r.enclosingNodes[i-1] == kv.Key {
			r.Log.Error("The key in a key-value expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}

		if ta, ok := node.(*ast.TypeAssertExpr); ok && i > 0 && r.enclosingNodes[i-1] == ta.Type {
			r.Log.Error("The selected expression cannot be extracted since it is part of the type in a type assertion.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	parent := r.enclosingNodes[1]
	if call, ok := parent.(*ast.CallExpr); ok {
		if r.SelectedNode == call.Fun {
			r.Log.Error("The function name in a function call expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	return true
}

// isTypeNode returns true iff the given AST node is an ArrayType,
// InterfaceType, MapType, StructType, or TypeSpec node.
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

// checkStatementContext looks at the statement in which the selected
// expression appears, if any, and determines if the expression can be
// extracted, logging an error and returning false if it cannot.  If this
// method returns true, the insertBefore argument will be set to an ast.Stmt;
// the extracted assignment statement should be inserted before this statement.
func (r *ExtractLocal) checkStatementContext(insertBefore *ast.Stmt) bool {
	enclosingStmt := -1
	for i, node := range r.enclosingNodes {
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

	*insertBefore = r.enclosingNodes[enclosingStmt].(ast.Stmt)

	switch stmt := r.enclosingNodes[enclosingStmt].(type) {
	case *ast.AssignStmt:
		return r.checkIfSelectedNodeIsAssignStmtLhs(stmt) &&
			r.checkIfSelectedNodeIsInIfStmtInit()

	case *ast.CaseClause:
		// grandparent will be switch or type switch statement
		grandparent := r.enclosingNodes[enclosingStmt+2].(ast.Stmt)
		*insertBefore = grandparent
		return r.checkIfIsTypeSwitch(grandparent)

	//case *ast.DeclStmt: // const, type, or var
	//case *ast.DeferStmt:
	//case *ast.EmptyStmt: impossible

	case *ast.ExprStmt:
		return true

	//case *ast.ForStmt: not allowed (control flow) unless in init expr
	//case *ast.GoStmt:

	case *ast.IfStmt:
		*insertBefore = r.findOutermostIfStmt(enclosingStmt)
		return r.checkIfIfStmtHasInit(stmt)

	//case *ast.IncDecStmt:
	//case *ast.LabeledStmt not allowed - label cannot be extracted

	case *ast.RangeStmt:
		return true

	case *ast.ReturnStmt:
		return true

	//case *ast.SelectStmt:
	//case *ast.SendStmt:
	//case *ast.SwitchStmt:
	//case *ast.TypeSwitchStmt:

	default:
		r.Log.Error("The selected expression cannot be extracted.")
		r.Log.Errorf("(Enclosing statement is %s)", reflect.TypeOf(r.enclosingNodes[enclosingStmt]))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

}

// checkIfSelectedNodeIsAssignStmtLhs determines if the selected node is one of
// the LHS expressions for the given assignment statement, logging an error and
// returning false if it is.
//
// Note that it is acceptable to extract a subexpression of an LHS expression
// (e.g., the subscript expression in a[i+2]=...), but not the entire expression.
func (r *ExtractLocal) checkIfSelectedNodeIsAssignStmtLhs(asgt *ast.AssignStmt) bool {
	for _, lhsExpr := range asgt.Lhs {
		if r.SelectedNode == lhsExpr {
			r.Log.Error("The selected expression cannot be extracted " +
				"since it is in the left-hand side of an assignment.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// checkIfSelectedNodeIsInIfStmtInit determines if the selected node appears in
// the initialization statement of an if statement, logging an error and
// returning false if it is.
func (r *ExtractLocal) checkIfSelectedNodeIsInIfStmtInit() bool {
	for i, node := range r.enclosingNodes {
		if ifStmt, ok := node.(*ast.IfStmt); ok && i > 0 && r.enclosingNodes[i-1] == ifStmt.Init {
			r.Log.Error("The selected expression cannot be extracted " +
				"since it is in an if statement initialization.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// checkIfIsTypeSwitch checks if the given node is a type switch statement.
// If it is, an error is logged, and false is returned.
func (r *ExtractLocal) checkIfIsTypeSwitch(node ast.Node) bool {
	if _, ok := node.(*ast.TypeSwitchStmt); ok {
		r.Log.Error("The selected expression cannot be extracted " +
			"since it is in a case clause for a type switch statement.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

// findOutermostIfStatement searches r.enclosingNodes starting at the given
// index, which should contain an *ast.IfStmt, and skips consecutive
// *ast.IfStmt entries to find the outermost enclosing *ast.IfStmt.
//
// When an if statement appears as an "else if", possibly deeply nested, this
// finds the outermost if statement.  The assignment to the extracted variable
// should be placed before the outermost if statement.
func (r *ExtractLocal) findOutermostIfStmt(start int) *ast.IfStmt {
	var result *ast.IfStmt
	// If inside an else if statement, find outermost if
	for j := start; j < len(r.enclosingNodes); j++ {
		if ifStmt, ok := r.enclosingNodes[j].(*ast.IfStmt); ok {
			result = ifStmt
		} else {
			break
		}
	}
	return result
}

// checkIfIfStmtHasInit determines if the given if statement has an
// initialization, logging an error and returning false if it does.
//
// There are several problems with such if statements:
// (1) if x := 3; x < 5
//     Here, the definition (and declaration) of x in the init statement
//     reaches the condition, so an expression involving x cannot be
//     extracted from the condition.
// (2) if thing, ok := x.(*Thing); ok
//     The type assertion cannot be extracted, since it assigns both thing and
//     ok in this context.
// (3) if value, found := myMap[entry]
//     Similar.
func (r *ExtractLocal) checkIfIfStmtHasInit(ifStmt *ast.IfStmt) bool {
	if ifStmt.Init != nil {
		r.Log.Error("Expressions cannot be extracted from an " +
			"if statement with an initialization.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

// checkForReachingDefinition determines if a local variable is possibly
// defined in the "from" statement and used in the "to" statement.
//
// For example, consider "if x := 3; x < 5".  The definition of x in the
// initialization statement reaches the if statement (specifically, its
// condition), so the expression "x < 5" cannot be extracted to a location
// above the if statement.
func (r *ExtractLocal) checkForReachingDefinition(from, to ast.Stmt) bool {
	var enclosingFunc *ast.FuncDecl
	for _, node := range r.enclosingNodes {
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

	c := cfg.FromFunc(enclosingFunc)
	in, _ := dataflow.ReachingDefs(c, r.SelectedNodePkg)
	if _, found := in[to][from]; found {
		r.Log.Error("The selected expression cannot be extracted " +
			"since the test condition depends on variables " +
			"defined in the initialization.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

// addEdits adds source code edits for this refactoring.
func (r *ExtractLocal) addEdits(insertBefore ast.Stmt) {
	selectedExprOffset := r.getOffset(r.SelectedNode)
	selectedExprEnd := r.getEndOffset(r.SelectedNode)
	selectedExprLen := selectedExprEnd - selectedExprOffset

	expression := string(r.FileContents[selectedExprOffset:selectedExprEnd])
	assignment := r.varName + " := " + expression + "\n"
	r.Edits[r.Filename].Add(&text.Extent{r.getOffset(insertBefore), 0}, assignment)

	r.Edits[r.Filename].Add(&text.Extent{selectedExprOffset, selectedExprLen}, r.varName)
}

func (r *ExtractLocal) getOffset(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Offset
}

func (r *ExtractLocal) getEndOffset(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Offset
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
