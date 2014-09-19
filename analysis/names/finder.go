// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package names provides search facilities required by the Rename refactoring,
// including the ability to find references to a particular name.
package names

import (
	"go/ast"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
)

type Finder struct {
	program *loader.Program
}

func NewFinder(program *loader.Program) *Finder {
	return &Finder{program}
}

/*
 * Finder's API methods are each defined in their own files in this package.
 */

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func (r *Finder) IsPackageName(ident *ast.Ident) bool {
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	if r.pkgInfo(r.fileContaining(ident)).Pkg.Name() == ident.Name && obj == nil {
		return true
	}

	return false
}

func (r *Finder) IsSwitchVar(ident *ast.Ident) bool {
	//pkginfo := r.pkgInfo(r.fileContaining(ident))
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if _, ok := obj.(*types.Var); !ok && obj == nil && !r.IsPackageName(ident) {
		//fmt.Println("types.var of ident",v)
		//fmt.Println("selected var in switch  clasue of type switch ")
		// fmt.Println("slected  switch var and types.var is",obj.(*types.Var))
		return true
	}

	//fmt.Println(" var is not swithvar")
	return false
}

// TODO: These are duplicated from refactoring.go
func (r *Finder) fileContaining(node ast.Node) *ast.File {
	tfile := r.program.Fset.File(node.Pos())
	for _, pkgInfo := range r.program.AllPackages {
		for _, thisFile := range pkgInfo.Files {
			if r.program.Fset.File(thisFile.Package) == tfile {
				return thisFile
			}
		}
	}
	panic("No ast.File for node")
}

func (r *Finder) pkgInfo(file *ast.File) *loader.PackageInfo {
	for _, pkgInfo := range r.program.AllPackages {
		for _, thisFile := range pkgInfo.Files {
			if thisFile == file {
				return pkgInfo
			}
		}
	}
	return nil
}
