// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a "debug" refactoring, which is not really a refactoring
// at all.  It does not change any files; rather, it is invoked to print
// information about the Go refactoring engine and its internals.  For example,
// it can display the AST for a file, or display what package(s) are loaded, or
// display what identifiers resolve to what objects.

package doctor

import (
	"fmt"
	"go/ast"
	"os"
	"strings"
)

type debugRefactoring struct {
	refactoringBase
}

func (r *debugRefactoring) Description() *Description {
	return &Description{
		Name:   "Debug Refactoring",
		Params: []string{"Options"},
	}
}

func (r *debugRefactoring) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	r.Log.ChangeInitialErrorsToWarnings()

	if len(config.Args) == 0 {
		fmt.Println("Usage: debug <options>")
		fmt.Println("where <options> can be any or all of:")
		fmt.Println("    showast")
		fmt.Println("    showpackages")
		fmt.Println("    showidentifiers")
		fmt.Println("    showreferences")
		fmt.Println("    showaffected")
		return &r.Result
	}

	for _, opt := range config.Args {
		switch strings.ToLower(strings.TrimSpace(opt)) {
		case "showast":
			r.showAST()
		case "showpackages":
			r.showLoadedPackagesAndFiles()
		case "showidentifiers":
			r.showIdentifiers()
		case "showreferences":
			r.showReferences()
		case "showaffected":
			r.showAffected()
		default:
			r.Log.Log(FATAL_ERROR, "Unknown option "+opt)
			return &r.Result
		}
	}

	r.Edits = map[string]EditSet{}
	return &r.Result
}

func (r *debugRefactoring) showAST() {
	r.forEachInitialFile(func(file *ast.File) {
		ast.Print(r.program.Fset, file)
	})
}

func (r *debugRefactoring) showLoadedPackagesAndFiles() {
	fmt.Printf("GOPATH is %s\n", os.Getenv("GOPATH"))
	cwd, _ := os.Getwd()
	fmt.Printf("Working directory is %s\n", cwd)
	fmt.Println()
	fmt.Println("Packages/files loaded:")
	for _, pkgInfo := range r.program.AllPackages {
		fmt.Printf("\t%s\n", pkgInfo.Pkg.Name())
		for _, file := range pkgInfo.Files {
			fmt.Printf("\t\t%s\n", r.filename(file))
		}
	}
}

func (r *debugRefactoring) showIdentifiers() {
	r.forEachInitialFile(func(file *ast.File) {
		fmt.Printf("=====%s=====\n", r.filename(file))
		ast.Inspect(file, func(n ast.Node) bool {
			switch id := n.(type) {
			case *ast.Ident:
				position := r.program.Fset.Position(id.Pos())
				fmt.Printf("%s\t(Line %d)", id.Name, position.Line)
				if obj := r.pkgInfo(file).ObjectOf(id); obj == nil {
					fmt.Printf(" does not have an associated object\n")
				} else {
					fmt.Printf(" is a reference to %s (%s)\n", obj.Id(), r.program.Fset.Position(obj.Pos()))
				}
			}
			return true
		})

	})
}

func (r *debugRefactoring) showReferences() {
	errorMsg := "Please select an identifier for showreferences"

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
	switch id := r.selectedNode.(type) {
	case *ast.Ident:
		fmt.Printf("References to %s:\n", id.Name)
		search := &SearchEngine{r.program}
		searchResult, err := search.FindOccurrences(id)
		if err != nil {
			r.Log.Log(FATAL_ERROR, err.Error())
			return
		}
		for filename, occs := range searchResult {
			fmt.Printf("  in %s:\n", filename)
			for _, ol := range occs {
				fmt.Printf("    %s\n", ol.String())
			}
		}
	default:
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
}

func (r *debugRefactoring) showAffected() {
	errorMsg := "Please select an identifier for showaffected"

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
	switch id := r.selectedNode.(type) {
	case *ast.Ident:
		fmt.Printf("Affected Declarations:\n")
		search := &SearchEngine{r.program}
		searchResult, err := search.FindDeclarationsAcrossInterfaces(id)
		if err != nil {
			r.Log.Log(FATAL_ERROR, err.Error())
			return
		}
		for obj := range searchResult {
			p := r.program.Fset.Position(obj.Pos())
			fmt.Printf("  %s - %s, Line %d\n",
				obj.Name(), p.Filename, p.Line)
		}
	default:
		r.Log.Log(FATAL_ERROR, errorMsg)
		return
	}
}
