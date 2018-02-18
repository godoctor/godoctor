// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE File.

// This File defines a "debug" refactoring, which is not really a refactoring
// at all.  It does not change any files; rather, it is invoked to print
// information about the Go refactoring engine and its internals.  For example,
// it can display the AST for a File, output a GraphViz DOT File with a File's
// control flow graphs, display what package(s) are loaded, or display what
// identifiers resolve to what objects.

package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/analysis/names"
)

const usage = `Usage: debug <options>
where <options> can be any or all of the following:

Information about the entire file:
    showast           Show the abstract syntax tree for the selected file
    showidentifiers   Show name references (ast.Object) in initial packages
    showpackages      List all packages loaded (due to --scope)

If anything is selected:
    fmt               Format the node enclosing the selection using go/printer

If an identifier is selected:
    showaffected      Show names affected if the selected identifier is renamed
    showreferences    Show all direct references to the selected identifier

If a function is selected...
    showcfg           Output the control flow graph (CFG) in GraphViz DOT format
    showdefuse        Output CFG with def-use information GraphViz DOT format

Use GraphViz's "dotty" tool to view DOT files.  For example:
    $ godoctor -file main.go -pos 5,1:5,1 debug showdefuse > output.dot
    $ dotty output.dot`

type Debug struct {
	base RefactoringBase
}

func (r *Debug) Description() *Description {
	return &Description{
		Name:      "Debug Refactoring",
		Synopsis:  "Provides assorted debugging outputs",
		Usage:     "<command>",
		HTMLDoc:   "",
		Multifile: false,
		Params: []Parameter{Parameter{
			Label:        "Command",
			Prompt:       "Command",
			DefaultValue: "",
		}},
		Hidden: true,
	}
}

func (r *Debug) Run(config *Config) *Result {
	r.base.Run(config)

	r.base.Log.ChangeInitialErrorsToWarnings()
	if r.base.Log.ContainsErrors() {
		return &r.base.Result
	}

	if len(config.Args) == 0 {
		r.base.Log.Error(usage)
		return &r.base.Result
	}

	if !ValidateArgs(config, r.Description(), r.base.Log) {
		return &r.base.Result
	}
	command := strings.ToLower(strings.TrimSpace(config.Args[0].(string)))

	switch command {
	case "fmt":
		r.fmt()
		return &r.base.Result
	case "showaffected":
		r.showAffected(&r.base.DebugOutput)
		return &r.base.Result
	case "showast":
		r.showAST(&r.base.DebugOutput)
		return &r.base.Result
	case "showcfg":
		r.showCFG(&r.base.DebugOutput)
		return &r.base.Result
	case "showdefuse":
		r.showDefUse(&r.base.DebugOutput)
		return &r.base.Result
	case "showidentifiers":
		r.showIdentifiers(&r.base.DebugOutput)
		return &r.base.Result
	case "showpackages":
		r.showLoadedPackagesAndFiles(&r.base.DebugOutput)
		return &r.base.Result
	case "showreferences":
		r.showReferences(&r.base.DebugOutput)
		return &r.base.Result
	default:
		r.base.Log.Errorf("Unknown option %s", command)
		return &r.base.Result
	}
}

func (r *Debug) fmt() {
	// Find the smallest formattable node enclosing the selection
	_, nodes, _ := r.base.Program.PathEnclosingInterval(r.base.SelectionStart, r.base.SelectionEnd)
	for _, node := range nodes {
		if canFormat(node) {
			cnode := &printer.CommentedNode{
				Node:     node,
				Comments: r.base.File.Comments}
			printConfig := &printer.Config{
				Mode:     printer.UseSpaces | printer.TabIndent,
				Tabwidth: 8}
			var b bytes.Buffer
			err := printConfig.Fprint(&b, r.base.Program.Fset, cnode)
			if err != nil {
				r.base.Log.Error(err)
				return
			}

			r.base.Edits[r.base.Filename].Add(r.base.Extent(node), b.String())
			return
		}
	}
}

func canFormat(node interface{}) bool {
	switch node.(type) {
	case ast.Expr:
		return true
	case ast.Stmt:
		return true
	case ast.Spec:
		return true
	case ast.Decl:
		return true
	case *ast.File:
		return true
	default:
		return false
	}

}

func (r *Debug) showAffected(out io.Writer) {
	errorMsg := "Please select an identifier for showaffected"

	if r.base.SelectedNode == nil {
		r.base.Log.Error(errorMsg)
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		return
	}
	switch id := r.base.SelectedNode.(type) {
	case *ast.Ident:
		fmt.Fprintf(out, "Affected Declarations:\n")
		obj := r.base.SelectedNodePkg.ObjectOf(id)
		searchResult := names.FindDeclarationsAcrossInterfaces(obj, r.base.Program)
		result := []string{}
		for obj := range searchResult {
			p := r.base.Program.Fset.Position(obj.Pos())
			result = append(result,
				fmt.Sprintf("  %s - %s, Line %d\n",
					obj.Name(), p.Filename, p.Line))
		}
		sort.Strings(result)
		for _, line := range result {
			fmt.Fprintf(out, line)
		}
	default:
		r.base.Log.Error(errorMsg)
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		return
	}
}

func (r *Debug) showAST(out io.Writer) {
	ast.Fprint(out, r.base.Program.Fset, r.base.File, nil)
}

func (r *Debug) showCFG(out io.Writer) {
	switch funcDecl := r.base.SelectedNode.(type) {
	case *ast.FuncDecl:
		if funcDecl.Name != nil {
			fmt.Fprintf(out, "// Control flow graph for %s\n", funcDecl.Name.Name)
		} else {
			fmt.Fprintf(out, "// Control flow graph for anonymous function\n")
		}
		cfg := cfg.FromFunc(funcDecl)
		cfg.PrintDot(out, r.base.Program.Fset, r.describeVariables)

	default:
		r.base.Log.Error("Please select a function.")
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		r.base.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.base.SelectedNode))
		r.base.Log.AssociatePos(r.base.SelectedNode.Pos(), r.base.SelectedNode.Pos())
	}
}

func (r *Debug) describeVariables(stmt ast.Stmt) string {
	var buf bytes.Buffer
	asgts, updts, decls, uses := dataflow.ReferencedVars([]ast.Stmt{stmt}, r.base.SelectedNodePkg)
	if len(asgts) > 0 {
		fmt.Fprintf(&buf, "Assigns: %s\n", listNames(asgts))
	}
	if len(updts) > 0 {
		fmt.Fprintf(&buf, "Updates: %s\n", listNames(updts))
	}
	if len(decls) > 0 {
		fmt.Fprintf(&buf, "Declares: %s\n", listNames(decls))
	}
	if len(uses) > 0 {
		fmt.Fprintf(&buf, "Uses: %s\n", listNames(uses))
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func listNames(vars map[*types.Var]struct{}) string {
	names := []string{}
	for v, _ := range vars {
		names = append(names, v.Name())
	}
	sort.Strings(names)

	var buf bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&buf, ", %s", name)
	}
	return strings.TrimPrefix(buf.String(), ", ")
}

func (r *Debug) showDefUse(out io.Writer) {
	switch funcDecl := r.base.SelectedNode.(type) {
	case *ast.FuncDecl:
		if funcDecl.Name != nil {
			fmt.Fprintf(out, "// %s\n", funcDecl.Name.Name)
		} else {
			fmt.Fprintf(out, "// (anonymous)\n")
		}
		cfg := cfg.FromFunc(funcDecl)
		dataflow.PrintDot(out, r.base.Program.Fset, r.base.SelectedNodePkg, cfg)

	default:
		r.base.Log.Error("Please select a function.")
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		r.base.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.base.SelectedNode))
		r.base.Log.AssociatePos(r.base.SelectedNode.Pos(), r.base.SelectedNode.Pos())
	}
}

func (r *Debug) showIdentifiers(out io.Writer) {
	for _, pkgInfo := range r.base.Program.InitialPackages() {
		for _, file := range pkgInfo.Files {
			r.showIdentifiersInFile(pkgInfo, file, out)
		}
	}
}

func (r *Debug) showIdentifiersInFile(pkgInfo *loader.PackageInfo, file *ast.File, out io.Writer) {
	filename, err := r.getRelativeFilename(file)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Fprintf(out, "=====%s=====\n",
		filename)
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		position := r.base.Program.Fset.Position(id.Pos())
		fmt.Fprintf(out, "%s\t(Line %d)", id.Name, position.Line)
		if obj := pkgInfo.ObjectOf(id); obj == nil {
			fmt.Fprintf(out, " does not reference an object\n")
		} else {
			fmt.Fprintf(out, " is a reference to %s (%s)\n",
				obj.Id(), r.base.Program.Fset.Position(obj.Pos()))
		}
		return true
	})
}

func (r *Debug) getRelativeFilename(file *ast.File) (string, error) {
	filename := r.base.Program.Fset.Position(file.Pos()).Filename
	cwd, err := os.Getwd()
	if err == nil {
		filename, err = filepath.Rel(cwd, filename)
	}
	return filename, err
}

func (r *Debug) showLoadedPackagesAndFiles(out io.Writer) {
	cwd, _ := os.Getwd()
	fmt.Fprintf(out, "Working directory is %s\n", cwd)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Packages/files loaded:")
	for _, pkgInfo := range r.base.Program.AllPackages {
		fmt.Fprintf(out, "\t%s\n", pkgInfo.Pkg.Name())
		for _, file := range pkgInfo.Files {
			filename := r.base.Program.Fset.Position(file.Pos()).Filename
			fmt.Fprintf(out, "\t\t%s\n", filename)
		}
	}
}

func (r *Debug) showReferences(out io.Writer) {
	errorMsg := "Please select an identifier for showreferences"

	if r.base.SelectedNode == nil {
		r.base.Log.Error(errorMsg)
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		return
	}
	switch id := r.base.SelectedNode.(type) {
	case *ast.Ident:
		fmt.Fprintf(out, "References to %s:\n", id.Name)
		ids := names.FindOccurrences(r.base.SelectedNodePkg.ObjectOf(id), r.base.Program)
		strs := []string{}
		for id, _ := range ids {
			description := fmt.Sprintf("  %s: %s",
				r.base.Program.Fset.Position(id.Pos()).Filename,
				r.base.Extent(id).String())
			strs = append(strs, description)
		}
		sort.Strings(strs)
		for _, s := range strs {
			fmt.Fprintln(out, s)
		}
	default:
		r.base.Log.Error(errorMsg)
		r.base.Log.AssociatePos(r.base.SelectionStart, r.base.SelectionEnd)
		return
	}
}
