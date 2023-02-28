// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/godoctor/godoctor/analysis/loader"
	"github.com/godoctor/godoctor/analysis/names"
	"github.com/godoctor/godoctor/text"

	"golang.org/x/tools/go/packages"
)

// -=- Utility Functions -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

func setup(t *testing.T) *loader.Program {
	var lconfig packages.Config
	lconfig.Dir = filepath.Join("testdata/src/foo")
	prog, err := loader.Load(&lconfig)
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func findPackage(p *loader.Program, pkgName string, t *testing.T) *packages.Package {
	for pkg, info := range p.AllPackages {
		if pkg.Name() == pkgName {
			return info
		}
	}
	t.Fatalf("Package %s not found", pkgName)
	return nil
}

func lookup(p *loader.Program, pkgName, name string, t *testing.T) types.Object {
	result := findPackage(p, pkgName, t).Types.Scope().Lookup(name)
	if result == nil {
		t.Fatalf("%s.%s not found", pkgName, name)
	}
	return result
}

func lookupFieldOrMethod(p *loader.Program, pkgName, container, name string, t *testing.T) types.Object {
	typ := lookup(p, pkgName, container, t).Type()
	obj, _, _ := types.LookupFieldOrMethod(typ, true, findPackage(p, pkgName, t).Types, name)
	if obj == nil {
		t.Fatalf("%s not found for %s.%s", name, pkgName, container)
	}
	return obj
}

func equals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findOccurrences(pkgName, identName string, t *testing.T) []string {
	prog := setup(t)
	ident, pkg := findFirstIdent(prog, pkgName, identName, t)
	occs := names.FindOccurrences(pkg.TypesInfo.ObjectOf(ident), prog)

	result := []string{}
	for id := range occs {
		pos := prog.Fset.Position(id.Pos())
		result = append(result, fmt.Sprintf("%s:%d",
			pos.Filename, pos.Offset))
	}
	sort.Strings(result)
	return result
}

func sortKeys(m map[string][]text.Extent) []string {
	result := []string{}
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func findInComments(pkgName, identName string, t *testing.T) []string {
	prog := setup(t)
	file := findPackage(prog, pkgName, t).Syntax[0]
	filename := prog.Fset.Position(file.Pos()).Filename

	result := []string{}
	for _, extent := range names.FindInComments(identName, file, nil, prog.Fset) {
		result = append(result, fmt.Sprintf("%s:%d", filename, extent.Offset))
	}
	return result
}

func findFirstIdent(p *loader.Program, pkgName, ident string, t *testing.T) (*ast.Ident, *packages.Package) {
	pkgInfo := findPackage(p, pkgName, t)
	var result *ast.Ident
	ast.Inspect(pkgInfo.Syntax[0],
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
	return result, pkgInfo
}

// -=- Tests -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
/*
func TestIsMethod(t *testing.T) {
	prog := setup(t)
	if names.isMethod(lookup(prog, "bar", "Exported", t)) == true {
		t.Fatal("bar.Exported is not a method")
	}
	if names.isMethod(lookupFieldOrMethod(prog, "bar", "t", "Method", t)) == false {
		t.Fatal("(t) Method in bar is a method")
	}
}

func TestMethodReceiver(t *testing.T) {
	prog := setup(t)
	if names.methodReceiver(lookup(prog, "bar", "Exported", t)) != nil {
		t.Fatal("bar.Exported should not have a receiver")
	}
	if names.methodReceiver(lookupFieldOrMethod(prog, "bar", "t", "Method", t)) == nil {
		t.Fatal("Receiver of (t) Method in bar should not be nil")
	}
}
*/

func TestFindOccurrences(t *testing.T) {
	check(findOccurrences("foo", "Exported", t),
		[]string{
			"testdata/src/foo/foo.go:32"}, t)
	check(findOccurrences("bar", "Exported", t),
		[]string{
			"testdata/src/foo/foo.go:71",
			"testdata/src/foo/vendor/bar/bar.go:18",
		}, t)
	check(findOccurrences("bar", "t", t),
		[]string{
			"testdata/src/foo/vendor/bar/bar.go:107",
			"testdata/src/foo/vendor/bar/bar.go:95"}, t)
	check(findOccurrences("bar", "Method", t),
		[]string{
			"testdata/src/foo/foo.go:247",
			"testdata/src/foo/vendor/bar/bar.go:174",
			"testdata/src/foo/vendor/bar/bar.go:74",
		}, t)
	check(findOccurrences("foo", "q", t),
		[]string{
			"testdata/src/foo/foo.go:137"}, t)
	check(findInComments("foo", "q", t),
		[]string{
			"testdata/src/foo/foo.go:145",
			"testdata/src/foo/foo.go:152",
			"testdata/src/foo/foo.go:164",
			"testdata/src/foo/foo.go:211",
			"testdata/src/foo/foo.go:262",
			"testdata/src/foo/foo.go:285"}, t)
}

func check(actual, expect []string, t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("couldn't get wd")
	}
	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		t.Fatalf("couldn't evaluate symlinks on %q: %q", wd, err)
	}
	for i := range expect {
		expect[i] = filepath.Join(wd, expect[i])
	}
	if !equals(actual, expect) {
		t.Fatalf("FindOccurrences: Expected %v, got %v", expect, actual)
	}
}

// (r *Finder) FindDeclarationsAcrossInterfaces(ident *ast.Ident) (map[types.Object]bool, error)
// (r *Finder) FindOccurrences(ident *ast.Ident) (map[string][]text.Extent, error)
