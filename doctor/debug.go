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
	r.Edits = map[string]*EditSet{}

	if r.Log.ContainsErrors() {
		return &r.Result
	}

	if len(config.Args) == 0 {
		fmt.Println(usage)
		return &r.Result
	}
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	for _, arg := range config.Args {
		switch strings.ToLower(strings.TrimSpace(arg.(string))) {
		case "showaffected":
			r.showAffected()
		case "showast":
			r.showAST()
		case "showidentifiers":
			r.showIdentifiers()
		case "showpackages":
			r.showLoadedPackagesAndFiles()
		case "showreferences":
			r.showReferences()
		default:
			r.Log.Log(FATAL_ERROR, "Unknown option "+arg.(string))
			return &r.Result
		}
	}

	return &r.Result
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

func (r *debugRefactoring) showAST() {
	ast.Print(r.program.Fset, r.file)
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
