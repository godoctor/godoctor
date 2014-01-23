// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Null refactoring, which makes no changes to a program.
// It is for testing only (and can be used as a template for building new
// refactorings).

// Contributors: Jeff Overbey

package doctor

import (
	"fmt"
	"go/ast"
	"strings"
)

import (
//	"fmt"
//	"go/ast"
//	"go/token"
)

// The NullRefactoring makes no changes to a program.
type NullRefactoring struct {
	RefactoringBase
	optShowAST        bool
	optShowPackages   bool
	optShowReferences bool
}

func (r *NullRefactoring) Name() string {
	return "Null Refactoring"
}

func (r *NullRefactoring) GetParams() []string {
	return []string{"Options"}
}

func (r *NullRefactoring) Configure(args []string) bool {
	switch len(args) {
	case 0:
		return true
	case 1:
		return r.processOptions(args[0])
	default:
		return false
	}
}

func (r *NullRefactoring) processOptions(options string) bool {
	for _, opt := range strings.Split(options, ",") {
		switch strings.TrimSpace(opt) {
		case "showAST":
			r.optShowAST = true
		case "showPackages":
			r.optShowPackages = true
		case "showReferences":
			r.optShowReferences = true
		default:
			return false
		}
	}
	return true
}

func (r *NullRefactoring) Run() {
	if r.selectedNode == nil {
		r.log.Log(FATAL_ERROR, "selection cannot be null")
		return // SetSelection did not succeed
	}

	if r.optShowAST {
		r.showAST()
	}

	if r.optShowPackages {
		r.showLoadedPackagesAndFiles()
	}

	if r.optShowReferences {
		r.resolveIdentifiers()
	}

	// If there were any semantic errors present in the original file(s),
	// you can downgrade those to warnings as follows:
	r.log.ChangeInitialErrorsToWarnings()
	// or you can remove them altogether using
	//r.log.RemoveInitialEntries()

	// If there were no initial errors, you can check whether or not your
	// refactoring introduced new syntactic or semantic errors as follows:
	//r.checkForErrors()
}

func (r *NullRefactoring) showAST() {
	ast.Print(r.program.Fset, r.file)
}

func (r *NullRefactoring) showLoadedPackagesAndFiles() {
	fmt.Println("Packages/files loaded:")
	for _, pkgInfo := range r.program.AllPackages {
		fmt.Printf("\t%s\n", pkgInfo.Pkg.Name())
		for _, file := range pkgInfo.Files {
			fmt.Printf("\t\t%s\n", r.filename(file))
		}
	}
}

func (r *NullRefactoring) resolveIdentifiers() {
	r.forEachFile(func(file *ast.File) {
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
	/*
		for _, pkgInfo := range r.program.InitialPackages() {
			for _, file := range pkgInfo.Files {
				filename := r.program.Fset.Position(file.Package).Filename
				fmt.Printf("=====%s=====\n", filename)
				ast.Inspect(file, func(n ast.Node) bool {
					switch id := n.(type) {
					case *ast.Ident:
						position := r.program.Fset.Position(n.Pos())
						fmt.Printf("%s\t(Line %d)", id.Name, position.Line)
						if obj := pkgInfo.ObjectOf(id); obj == nil {
							fmt.Printf(" does not have an associated object\n")
						} else {
							fmt.Printf(" is a reference to %s\n", obj.Id())
								for file, occs := range r.findOccurrences(true, id) {
									fmt.Printf("    %s: ", file)
									for _, ol := range occs {
										fmt.Printf(" %s", ol.String())
									}
									fmt.Println()
								}
						}
					}
					return true
				})

			}
		}
	*/
}
