// Copyright 2013 The Go Authors. All rights reserved.
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
	RefactoringBase
	optShowAST         bool
	optShowPackages    bool
	optShowIdentifiers bool
	optShowReferences  bool
}

func (r *debugRefactoring) Name() string {
	return "Null Refactoring"
}

func (r *debugRefactoring) GetParams() []string {
	return []string{"Options"}
}

func (r *debugRefactoring) Configure(options []string) bool {
	if len(options) == 0 {
		fmt.Println("Usage: debug <options>")
		fmt.Println("where <options> can be any or all of:")
		fmt.Println("    showast")
		fmt.Println("    showpackages")
		fmt.Println("    showidentifiers")
		fmt.Println("    showreferences")
		return false
	}

	for _, opt := range options {
		switch strings.ToLower(strings.TrimSpace(opt)) {
		case "showast":
			r.optShowAST = true
		case "showpackages":
			r.optShowPackages = true
		case "showidentifiers":
			r.optShowIdentifiers = true
		case "showreferences":
			r.optShowReferences = true
		default:
			r.log.Log(FATAL_ERROR, "Unknown option "+opt)
			return false
		}
	}
	return true
}

func (r *debugRefactoring) Run() {
	r.log.ChangeInitialErrorsToWarnings()

	if r.optShowAST {
		r.showAST()
	}

	if r.optShowPackages {
		r.showLoadedPackagesAndFiles()
	}

	if r.optShowIdentifiers {
		r.showIdentifiers()
	}

	if r.optShowReferences {
		r.showReferences()
	}

	r.editSet = map[string]EditSet{}
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
				position := r.program.Fset.Position(n.Pos())
				fmt.Printf("%s\t(Line %d)", id.Name, position.Line)
				if obj := r.pkgInfo(file).ObjectOf(id); obj == nil {
					fmt.Printf(" does not have an associated object\n")
				} else {
					fmt.Printf(" is a reference to %s\n", obj.Id())
				}
			}
			return true
		})

	})
}

func (r *debugRefactoring) showReferences() {
	errorMsg := "Please select an identifier for showreferences"

	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, errorMsg)
		return
	}
	switch id := r.selectedNode.(type) {
	case *ast.Ident:
		fmt.Printf("References to %s:\n", id.Name)
		for filename, occs := range r.findOccurrences(true, id) {
			fmt.Printf("  in %s:\n", filename)
			for _, ol := range occs {
				fmt.Printf("    %s\n", ol.String())
			}
		}
	default:
		r.log.Log(FATAL_ERROR, errorMsg)
		return
	}
}
