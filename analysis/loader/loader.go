// Package loader wraps golang.org/x/tools/go/packages with utility types and methods
package loader

// this package mostly exists due to cruft in migrating from
// golang.org/x/tools/go/loader to golang.org/x/tools/go/packages
// where certain types and functions were more convenient to put here
// rather than do significant reworking of existing package APIs (which,
// looked to worsen APIs and not improve them, thus...). this package
// relies heavily on the packages package to do the heavy lifting.

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

// Program provides access to various elements of a type checked set of packages.
// Fset and AllPackages are not so easily combined, we often need to query the
// entire program ast which contains all packages, often needing additional
// information within any given package after that.
type Program struct {
	// Fset contains the entire program Fset
	Fset *token.FileSet

	// AllPackages is the list of all loaded packages for a program.
	// It is this exposed this way to match the old loader API.
	AllPackages map[*types.Package]*packages.Package
}

// Load loads a package, calling packages.Load
func Load(conf *packages.Config) (*Program, error) {
	// TODO(reed): we do kinda need to ensure types is set so that
	// files are parsed, but punting as it's hard enough to define the scope
	// of this package (ie since it's simply for our uses...)

	if conf.Fset != nil {
		// we need this
		conf.Fset = token.NewFileSet()
	}
	prog, err := packages.Load(conf)
	if err != nil {
		return nil, err
	}

	pkgs := make(map[*types.Package]*packages.Package, len(prog))
	for _, p := range prog {
		pkgs[p.Types] = p
	}

	return &Program{
		Fset:        conf.Fset,
		AllPackages: pkgs,
	}, nil
}

// PathEnclosingInterval returns the PackageInfo and ast.Node that
// contain source interval [start, end), and all the node's ancestors
// up to the AST root.  It searches all ast.Files of all packages in prog.
// exact is defined as for astutil.PathEnclosingInterval.
//
// The zero value is returned if not found.
//
func (prog *Program) PathEnclosingInterval(start, end token.Pos) (pkg *packages.Package, path []ast.Node, exact bool) {
	for _, info := range prog.AllPackages {
		for _, f := range info.Syntax {
			if f.Pos() == token.NoPos {
				// This can happen if the parser saw
				// too many errors and bailed out.
				// (Use parser.AllErrors to prevent that.)
				continue
			}
			if !tokenFileContainsPos(prog.Fset.File(f.Pos()), start) {
				continue
			}
			if path, exact := astutil.PathEnclosingInterval(f, start, end); path != nil {
				return info, path, exact
			}
		}
	}
	return nil, nil, false
}

func tokenFileContainsPos(f *token.File, pos token.Pos) bool {
	p := int(pos)
	base := f.Base()
	return base <= p && p < base+f.Size()
}
