// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names_test

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"sort"
	"testing"

	"golang-refactoring.org/go-doctor/analysis/names"
	"golang-refactoring.org/go-doctor/text"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
)

// -=- Utility Functions -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

func setup(t *testing.T) *loader.Program {
	var lconfig loader.Config
	build := build.Default
	build.GOPATH = "testdata"
	lconfig.Build = &build
	lconfig.ParserMode = parser.ParseComments | parser.DeclarationErrors
	lconfig.AllowErrors = false
	lconfig.SourceImports = true
	lconfig.TypeChecker.Error = func(err error) {
		t.Fatal(err)
	}
	lconfig.FromArgs([]string{"foo"}, true)
	prog, err := lconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func findPackage(p *loader.Program, pkgName string, t *testing.T) *loader.PackageInfo {
	for pkg, info := range p.AllPackages {
		if pkg.Name() == pkgName {
			return info
		}
	}
	t.Fatalf("Package %s not found", pkgName)
	return nil
}

func lookup(p *loader.Program, pkgName, name string, t *testing.T) types.Object {
	result := findPackage(p, pkgName, t).Pkg.Scope().Lookup(name)
	if result == nil {
		t.Fatalf("%s.%s not found", pkgName, name)
	}
	return result
}

func lookupFieldOrMethod(p *loader.Program, pkgName, container, name string, t *testing.T) types.Object {
	typ := lookup(p, pkgName, container, t).Type()
	obj, _, _ := types.LookupFieldOrMethod(typ, true, findPackage(p, pkgName, t).Pkg, name)
	if obj == nil {
		t.Fatalf("%s not found for %s.%s", name, pkgName, container)
	}
	return obj
}

func equals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, _ := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findOccurrences(pkgName, identName string, t *testing.T) []string {
	prog := setup(t)
	ident := findFirstIdent(prog, pkgName, identName, t)
	searchResult, err := names.NewFinder(prog).FindOccurrences(ident)

	result := []string{}
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range sortKeys(searchResult) {
		for _, extent := range searchResult[filename] {
			result = append(result, fmt.Sprintf("%s:%d", filename, extent.Offset))
		}
	}
	return result
}

func findFirstIdent(p *loader.Program, pkgName, ident string, t *testing.T) *ast.Ident {
	var result *ast.Ident
	ast.Inspect(findPackage(p, pkgName, t).Files[0],
		func(n ast.Node) bool {
			switch id := n.(type) {
			case *ast.Ident:
				if result == nil && id.Name == ident {
					result = id
				}
			}
			return true
		})
	if result == nil {
		t.Fatal("No identifiers found")
	}
	return result
}

func sortKeys(m map[string][]text.Extent) []string {
	result := []string{}
	for k, _ := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// -=- Tests -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

func TestIsMethod(t *testing.T) {
	prog := setup(t)
	if names.IsMethod(lookup(prog, "bar", "Exported", t)) == true {
		t.Fatal("bar.Exported is not a method")
	}
	if names.IsMethod(lookupFieldOrMethod(prog, "bar", "t", "Method", t)) == false {
		t.Fatal("(t) Method in bar is a method")
	}
}

func TestMethodReceiver(t *testing.T) {
	prog := setup(t)
	if names.MethodReceiver(lookup(prog, "bar", "Exported", t)) != nil {
		t.Fatal("bar.Exported should not have a receiver")
	}
	if names.MethodReceiver(lookupFieldOrMethod(prog, "bar", "t", "Method", t)) == nil {
		t.Fatal("Receiver of (t) Method in bar should not be nil")
	}
}

func TestFindOccurrences(t *testing.T) {
	check(findOccurrences("foo", "Exported", t),
		[]string{
			"testdata/src/foo/foo.go:32"}, t)
	check(findOccurrences("bar", "Exported", t),
		[]string{
			"testdata/src/bar/bar.go:18",
			"testdata/src/foo/foo.go:71"}, t)
	check(findOccurrences("bar", "t", t),
		[]string{
			"testdata/src/bar/bar.go:95",
			"testdata/src/bar/bar.go:107"}, t)
	check(findOccurrences("bar", "Method", t),
		[]string{
			"testdata/src/bar/bar.go:74",
			"testdata/src/bar/bar.go:174",
			"testdata/src/foo/foo.go:246"}, t)
	check(findOccurrences("foo", "q", t),
		[]string{
			"testdata/src/foo/foo.go:136",
			"testdata/src/foo/foo.go:144",
			"testdata/src/foo/foo.go:151",
			"testdata/src/foo/foo.go:163",
			"testdata/src/foo/foo.go:210"}, t)
}

func check(actual, expect []string, t *testing.T) {
	if !equals(actual, expect) {
		t.Fatalf("FindOccurrences: Expected %v, got %v", expect, actual)
	}
}

// (r *Finder) FindDeclarationsAcrossInterfaces(ident *ast.Ident) (map[types.Object]bool, error)
// (r *Finder) FindOccurrences(ident *ast.Ident) (map[string][]text.Extent, error)
// (r *Finder) IsPackageName(ident *ast.Ident)
