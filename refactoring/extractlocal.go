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

	"github.com/godoctor/godoctor/text"
)

type ExtractLocal struct {
	RefactoringBase
	varName string
}

func (r *ExtractLocal) Description() *Description {
	return &Description{
		Name:      "Extract Local Variable",
		Synopsis:  "Extracts an expression, assigning it to a variable",
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

	// First check preconditions that cause fatal errors
	// (i.e., the transformation cannot proceed unless they are met,
	// since it won't know where to insert the extracted expression,
	// or the extraction is likely to produce invalid code)
	if r.checkSelectedNodeIsExpr() &&
		r.checkExprHasValidType() &&
		r.checkExprIsNotFieldSelector() &&
		r.checkExprAddressIsNotTaken() &&
		r.checkExprIsNotInTypeNode() &&
		r.checkExprIsNotKeyInKeyValueExpr() &&
		r.checkExprIsNotFunctionInCallExpr() &&
		r.checkExprIsNotInTypeAssertionType() &&
		r.checkExprHasEnclosingStmt() &&
		r.checkEnclosingStmtIsAllowed() &&
		r.checkExprIsNotAssignStmtLhs() &&
		r.checkExprIsNotInIfStmtWithInit() &&
		r.checkExprIsNotRangeStmtLhs() &&
		r.checkExprIsNotInCaseClauseOfTypeSwitchStmt() {
		// Now, check preconditions that are only for semantic
		// preservation (i.e., they should not block the refactoring,
		// but the user should be made aware of a potential problem)
		r.checkForNameConflict()
		// Finally, perform the transformation
		r.addEdits(r.findStmtToInsertBefore())
		r.FormatFileInEditor()
		r.UpdateLog(config, false)
	}
	return &r.Result
}

// checkSelectedNodeIsExpr checks that the user has selected an expression,
// logging an error and returning false iff not.
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
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectedNode.Pos(), r.SelectedNode.Pos())
		return false
	}
	return true
}

// checkExprHasValidType determines the type of the selected expression and
// determines whether it can be assigned to a variable, logging an error and
// returning false if it cannot.
func (r *ExtractLocal) checkExprHasValidType() bool {
	exprType := r.SelectedNodePkg.TypeOf(r.SelectedNode.(ast.Expr))
	// fmt.Printf("Node is %s\n", reflect.TypeOf(r.SelectedNode))
	// fmt.Printf("Type is %s\n", exprType)

	if _, isFunctionType := exprType.(*types.Tuple); isFunctionType {
		r.Log.Errorf("The selected expression cannot be assigned to a variable since it has a tuple type %s", exprType)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	if basic, isBasic := exprType.(*types.Basic); isBasic && (basic.Kind() == types.Invalid || basic.Info() == types.IsUntyped) {
		r.Log.Error("The selected expression cannot be assigned to a variable.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	return true
}

func (r *ExtractLocal) checkExprIsNotFieldSelector() bool {
	parentNode := r.PathEnclosingSelection[1]
	if selectorExpr, ok := parentNode.(*ast.SelectorExpr); ok {
		if selectorExpr.Sel == r.SelectedNode {
			r.Log.Error("A field selector cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

func (r *ExtractLocal) checkExprAddressIsNotTaken() bool {
	// This isn't completely correct, since &((((x)))) also takes an
	// address, but it's close enough for now
	parentNode := r.PathEnclosingSelection[1]
	if unary, ok := parentNode.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		r.Log.Error("An expression cannot be extracted if its address is taken.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

func (r *ExtractLocal) checkExprIsNotInTypeNode() bool {
	for _, node := range r.PathEnclosingSelection {
		if isTypeNode(node) {
			r.Log.Error("An expression used to specify a type cannot be extracted.")
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

func (r *ExtractLocal) checkExprIsNotKeyInKeyValueExpr() bool {
	for i, node := range r.PathEnclosingSelection {
		if kv, ok := node.(*ast.KeyValueExpr); ok && i > 0 && r.PathEnclosingSelection[i-1] == kv.Key {
			r.Log.Error("The key in a key-value expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

func (r *ExtractLocal) checkExprIsNotInTypeAssertionType() bool {
	for i, node := range r.PathEnclosingSelection {
		if ta, ok := node.(*ast.TypeAssertExpr); ok && i > 0 && r.PathEnclosingSelection[i-1] == ta.Type {
			r.Log.Error("The selected expression cannot be extracted since it is part of the type in a type assertion.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

func (r *ExtractLocal) checkExprIsNotFunctionInCallExpr() bool {
	parent := r.PathEnclosingSelection[1]
	if call, ok := parent.(*ast.CallExpr); ok {
		if r.SelectedNode == call.Fun {
			r.Log.Error("The function name in a function call expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// enclosingStmtIndex returns the index into r.PathEnclosingSelection of the
// smallest ast.Stmt enclosing the selection, or -1 if the selection is not
// in a statement.
//
// If this returns a nonnegative value,
// r.PathEnclosingSelection[r.enclosingStmtIndex()].(ast.Stmt)
// can be asserted.
//
// See enclosingStmt
func (r *ExtractLocal) enclosingStmtIndex() int {
	for i, node := range r.PathEnclosingSelection {
		if _, ok := node.(ast.Stmt); ok {
			return i
		}
	}
	return -1
}

// enclosingStmt returns the smallest ast.Stmt enclosing the selection.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) enclosingStmt() ast.Stmt {
	return r.PathEnclosingSelection[r.enclosingStmtIndex()].(ast.Stmt)
}

func (r *ExtractLocal) checkExprHasEnclosingStmt() bool {
	if r.enclosingStmtIndex() < 0 {
		r.Log.Error("The selected expression cannot be extracted " +
			"since it is not in an executable statement.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

// checkEnclosingStmtIsAllowed looks at the statement in which the selected
// expression appears and determines if the expression can be extracted,
// logging an error and returning false if it cannot.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkEnclosingStmtIsAllowed() bool {
	// fmt.Printf("Enclosing stmt is %s\n", reflect.TypeOf(r.enclosingStmt()))
	switch r.enclosingStmt().(type) {
	case *ast.AssignStmt:
		return true
	case *ast.CaseClause:
		return true
	//case *ast.DeclStmt: // const, type, or var
	//case *ast.DeferStmt:
	//case *ast.EmptyStmt: impossible
	case *ast.ExprStmt:
		return true
	//case *ast.ForStmt: not allowed (control flow) unless in init expr
	//case *ast.GoStmt:
	case *ast.IfStmt:
		return true
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
		r.Log.Errorf("The selected expression cannot be extracted.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Enclosing statement is %s)", reflect.TypeOf(r.enclosingStmt()))
		r.Log.AssociatePos(r.enclosingStmt().Pos(), r.enclosingStmt().Pos())
		return false
	}
}

// checkExprIsNotAssignStmtLhs determines if the selected node is one of the
// LHS expressions for the given assignment statement, logging an error and
// returning false if it is.
//
// Note, in particular, that this prevents extracting _.
//
// Note that it is acceptable to extract a subexpression of an LHS expression
// (e.g., the subscript expression in a[i+2]=...), but not the entire expression.
func (r *ExtractLocal) checkExprIsNotAssignStmtLhs() bool {
	for _, node := range r.PathEnclosingSelection {
		if asgt, ok := node.(*ast.AssignStmt); ok {
			for _, lhsExpr := range asgt.Lhs {
				if r.SelectedNode == lhsExpr {
					r.Log.Error("The selected expression cannot be extracted since it is assigned to.")
					r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
					return false
				}
			}
		}
	}
	return true
}

// checkExprIsNotInIfStmtWithInit determines if the selected expression is in
// the condition of an if statement (or nested else-if) with an initialization,
// logging an error and returning false if so.
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
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkExprIsNotInIfStmtWithInit() bool {
	if _, isIfStmt := r.enclosingStmt().(*ast.IfStmt); !isIfStmt {
		return true
	}

	first := r.enclosingStmtIndex()
	last := r.findBeginningOfElseIfChain(first)
	for i := first; i <= last; i++ {
		if ifStmt := r.PathEnclosingSelection[i].(*ast.IfStmt); ifStmt.Init != nil {
			r.Log.Error("Expressions cannot be extracted from an " +
				"if statement with an initialization.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// checkExprIsNotRangeStmtLhs determines if the selected node is either the key
// or value expression for a range statement, logging an error and returning
// false if it is.
//
// Note that it is acceptable to extract a subexpression of an LHS expression
// (e.g., the subscript expression in a[i+2]=...), but not the entire expression.
func (r *ExtractLocal) checkExprIsNotRangeStmtLhs() bool {
	for _, node := range r.PathEnclosingSelection {
		if asgt, ok := node.(*ast.RangeStmt); ok {
			if asgt.Key == r.SelectedNode || asgt.Value == r.SelectedNode {
				r.Log.Error("The selected expression cannot be extracted since it is the key or value expression for a range statement.")
				r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
				return false
			}
		}
	}
	return true
}

// checkExprIsNotInCaseClauseOfTypeSwitchStmt checks if the selected expression
// appears in a case clause for a type switch statement.  If it is, an error is
// logged, and false is returned.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkExprIsNotInCaseClauseOfTypeSwitchStmt() bool {
	if _, ok := r.enclosingStmt().(*ast.CaseClause); ok {
		// grandparent will be switch or type switch statement
		grandparent := r.PathEnclosingSelection[r.enclosingStmtIndex()+2].(ast.Stmt)
		if _, ok := grandparent.(*ast.TypeSwitchStmt); ok {
			r.Log.Error("The selected expression cannot be extracted since it is in a case clause for a type switch statement.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// checkForNameConflict determines if the new variable name will conflict with
// or shadow an existing name, logging an error and returning false if it will.
func (r *ExtractLocal) checkForNameConflict() bool {
	scope := r.scopeEnclosingSelection()
	if scope == nil {
		r.Log.Error("A scope could not be found for the selected expression.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	// TO DISPLAY THE SCOPE:
	// var buf bytes.Buffer
	// scope.WriteTo(&buf, 0, true)
	// fmt.Println(buf.String())

	existingObj := scope.Lookup(r.varName)
	if existingObj != nil {
		r.Log.Errorf("If a variable named %s is introduced, it will conflict with an existing declaration.", r.varName)
		r.Log.AssociatePos(existingObj.Pos(), existingObj.Pos())
		return false
	}

	_, existingObj = scope.LookupParent(r.varName, r.SelectedNode.Pos())
	if existingObj != nil {
		r.Log.Errorf("If a variable named %s is introduced, it will shadow an existing declaration.", r.varName)
		r.Log.AssociatePos(existingObj.Pos(), existingObj.Pos())
		return false
	}

	return true
}

// scopeEnclosingSelection returns the smallest scope in which the selected
// node exists.
func (r *ExtractLocal) scopeEnclosingSelection() *types.Scope {
	for _, node := range r.PathEnclosingSelection {
		if scope, found := r.SelectedNodePkg.Info.Scopes[node]; found {
			return scope.Innermost(r.SelectedNode.Pos())
		}
	}
	return nil
}

// findStmtToInsertBefore determines what statement the extracted variable
// assignment should be inserted before.
//
// Often, this is just the statement enclosing the selected node.  However,
// when the enclosing statement is a case clause of a switch statment, or when
// it is an if statement that serves as the else-if of another if statement,
// the assignment must be inserted earlier.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) findStmtToInsertBefore() ast.Stmt {
	switch r.enclosingStmt().(type) {
	case *ast.CaseClause:
		// grandparent will be switch or type switch statement
		grandparent := r.PathEnclosingSelection[r.enclosingStmtIndex()+2].(ast.Stmt)
		return grandparent

	case *ast.IfStmt:
		idx := r.findBeginningOfElseIfChain(r.enclosingStmtIndex())
		return r.PathEnclosingSelection[idx].(ast.Stmt)

	default:
		return r.enclosingStmt().(ast.Stmt)
	}
}

// findBeginningOfElseIfChain searches r.PathEnclosingSelection starting at the
// given index, which should contain an *ast.IfStmt, and skips consecutive
// *ast.IfStmt entries to find the outermost enclosing *ast.IfStmt for which
// all the previous entries were else-if statements.
//
// When an if statement appears as an "else if", possibly deeply nested, this
// finds the outermost if statement.  The assignment to the extracted variable
// should be placed before the outermost if statement.
//
// As the following example shows, this traces else-if chains upward.
// This is not the same as finding the outermost enclosing if statement.
//
//     if (v) {                 // The beginning of the else-if chain is NOT v;
//         if (w) {             // it is w...
//         } else if (x) {
//             if (y) {
//             } else if (z) {  // ...if we start from z
//             }
//         }
//     }
func (r *ExtractLocal) findBeginningOfElseIfChain(index int) int {
	ifStmt := r.PathEnclosingSelection[index].(*ast.IfStmt)
	if index+1 < len(r.PathEnclosingSelection) {
		if enclosingIfStmt, ok := r.PathEnclosingSelection[index+1].(*ast.IfStmt); ok && enclosingIfStmt.Else == ifStmt {
			return r.findBeginningOfElseIfChain(index + 1)
		}
	}
	return index
}

// addEdits adds source code edits for this refactoring
func (r *ExtractLocal) addEdits(insertBefore ast.Stmt) {
	selectedExprOffset := r.getOffset(r.SelectedNode)
	selectedExprEnd := r.getEndOffset(r.SelectedNode)
	selectedExprLen := selectedExprEnd - selectedExprOffset

	// First, replace the original expression.
	r.Edits[r.Filename].Add(&text.Extent{selectedExprOffset, selectedExprLen}, r.varName)

	// Then, add the assignment statement afterward.
	// If this inserts at the same position as the replacement, this
	// guarantees that it will be inserted before it, which is what we want
	expression := string(r.FileContents[selectedExprOffset:selectedExprEnd])
	assignment := r.varName + " := " + expression + "\n"
	r.Edits[r.Filename].Add(&text.Extent{r.getOffset(insertBefore), 0}, assignment)
}

// getOffset returns the token.Pos for the first character of the given node
func (r *ExtractLocal) getOffset(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Offset
}

// getEndOffset returns the token.Pos one byte beyond the end of the given node
func (r *ExtractLocal) getEndOffset(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Offset
}

const extractLocalDoc = `
  <h4>Purpose</h4>
  <p>The Extract Local Variable takes an expression, assigns it to a new
  local variable, then replaces the original expression with a use of that
  variable.</p>

  <h4>Usage</h4>
  <ol class="enum">
    <li>Select an expression in an existing statement.</li>
    <li>Activate the Introduce Local Variable refactoring.</li>
    <li>Enter a name for the new variable that will be created.</li>
  </ol>

  <p>An error or warning will be reported if the selected expression cannot be
  extracted into a variable assignment.</p>
`
