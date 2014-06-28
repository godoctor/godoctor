// Copyright 2014 The Go Authors. All rights reserved.  // Use of this source code is governed by a BSD-style // license that can be found in the LICENSE file.  // This file defines a "debug" refactoring, which is not really a refactoring // at all.  It does not change any files; rather, it is invoked to print // information about the Go refactoring engine and its internals.  For example, // it can display the AST for a file, or display what package(s) are loaded, or // display what identifiers resolve to what objects.
package doctor

import (
	"bytes"
	"fmt"
	"go/ast"
	"io"
	"os"
	"sort"
	"strings"
)

const usage = `Usage: debug <options>
where <options> can be any or all of:
    showaffected      Show names affected if the selected identifier is renamed
    showast           Show the abstract syntax tree for the selected file
    showidentifiers   Show name references (ast.Object) in initial packages
    showpackages      List all packages loaded (due to --scope)
    showreferences    Show all direct references to the selected identifier`

type debugRefactoring struct {
	refactoringBase
}

func (r *debugRefactoring) Description() *Description {
	return &Description{
		Name: "Debug Refactoring",
		Params: []Parameter{Parameter{
			Label:        "Options",
			Prompt:       "Options",
			DefaultValue: "",
		}},
		Quality: Development,
	}
}

func (r *debugRefactoring) Run(config *Config) *Result {
	r.refactoringBase.Run(config)

	if r.Log.ContainsErrors() {
		return &r.Result
	}

	if len(config.Args) == 0 {
		r.Log.Log(FATAL_ERROR, usage)
		return &r.Result
	}
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	for _, arg := range config.Args {
		var b bytes.Buffer
		switch strings.ToLower(strings.TrimSpace(arg.(string))) {
		case "showaffected":
			r.showAffected(&b)
		case "showast":
			r.showAST(&b)
		case "showidentifiers":
			r.showIdentifiers(&b)
		case "showpackages":
			r.showLoadedPackagesAndFiles(&b)
		case "showreferences":
			r.showReferences(&b)
		default:
			r.Log.Log(FATAL_ERROR, "Unknown option "+arg.(string))
			return &r.Result
		}
		if !r.Log.ContainsErrors() {
			r.Edits[r.filename(r.file)].Add(OffsetLength{0, 0}, b.String())
		}
	}

	return &r.Result
}

func (r *debugRefactoring) showAffected(out io.Writer) {
	errorMsg := "Please select an identifier for showaffected"

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
	switch id := r.selectedNode.(type) {
	case *ast.Ident:
		fmt.Fprintf(out, "Affected Declarations:\n")
		search := &SearchEngine{r.program}
		searchResult, err := search.FindDeclarationsAcrossInterfaces(id)
		if err != nil {
			r.Log.Log(FATAL_ERROR, err.Error())
			return
		}
		result := []string{}
		for obj := range searchResult {
			p := r.program.Fset.Position(obj.Pos())
			result = append(result,
				fmt.Sprintf("  %s - %s, Line %d\n",
					obj.Name(), p.Filename, p.Line))
		}
		sort.Strings(result)
		for _, line := range result {
			fmt.Fprintf(out, line)
		}
	default:
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
}

func (r *debugRefactoring) showAST(out io.Writer) {
	ast.Fprint(out, r.program.Fset, r.file, nil)
}

func (r *debugRefactoring) showIdentifiers(out io.Writer) {
	r.forEachInitialFile(func(file *ast.File) {
		fmt.Fprintf(out, "=====%s=====\n", r.filename(file))
		ast.Inspect(file, func(n ast.Node) bool {
			switch id := n.(type) {
			case *ast.Ident:
				position := r.program.Fset.Position(id.Pos())
				fmt.Fprintf(out, "%s\t(Line %d)", id.Name, position.Line)
				if obj := r.pkgInfo(file).ObjectOf(id); obj == nil {
					fmt.Fprintf(out, " does not have an associated object\n")
				} else {
					fmt.Fprintf(out, " is a reference to %s (%s)\n", obj.Id(), r.program.Fset.Position(obj.Pos()))
				}
			}
			return true
		})

	})
}

func (r *debugRefactoring) showLoadedPackagesAndFiles(out io.Writer) {
	fmt.Fprintf(out, "GOPATH is %s\n", os.Getenv("GOPATH"))
	cwd, _ := os.Getwd()

	fmt.Fprintf(out, "Working directory is %s\n", cwd)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Packages/files loaded:")
	for _, pkgInfo := range r.program.AllPackages {
		fmt.Fprintf(out, "\t%s\n", pkgInfo.Pkg.Name())
		for _, file := range pkgInfo.Files {
			fmt.Fprintf(out, "\t\t%s\n", r.filename(file))
		}
	}
}

func (r *debugRefactoring) showReferences(out io.Writer) {
	errorMsg := "Please select an identifier for showreferences"

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
	switch id := r.selectedNode.(type) {
	case *ast.Ident:
		fmt.Fprintf(out, "References to %s:\n", id.Name)
		search := &SearchEngine{r.program}
		searchResult, err := search.FindOccurrences(id)
		if err != nil {
			r.Log.Log(FATAL_ERROR, err.Error())
			return
		}
		for filename, occs := range searchResult {
			fmt.Fprintf(out, "  in %s:\n", filename)
			strs := []string{}
			for _, ol := range occs {
				strs = append(strs,
					fmt.Sprintf("    %s", ol.String()))
			}
			sort.Strings(strs)
			for _, s := range strs {
				fmt.Fprintln(out, s)
			}
		}
	default:
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
}
