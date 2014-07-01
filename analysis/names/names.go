// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
	"golang-refactoring.org/go-doctor/text"
)

type SearchEngine struct {
	program *loader.Program
}

func NewSearchEngine(program *loader.Program) *SearchEngine {
	return &SearchEngine{program}
}

/* -=-=- Search Across Interfaces =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// FindDeclarationsAcrossInterfaces finds all objects that might need to be
// renamed if the given identifier is renamed.  In the case of a method, there
// may be indirect relationships such as the following:
//
//      Interface1  Interface2
//         /  \      /  \
//        /  implements  \
//       /      \   /     \
//     Type1    Type2    Type3
//
// where renaming a method in Type1 could force a method with the same
// signature to be renamed in Interface1, Interface2, Type2, and Type3.  This
// method returns a set containing the reflexive, transitive closure of objects
// that must be renamed if the given identifier is renamed.
// TODO: Does this need to be API?  If not, no need to explicitly check if ident is a method
func (r *SearchEngine) FindDeclarationsAcrossInterfaces(ident *ast.Ident) (map[types.Object]bool, error) {
	pkgInfo := r.pkgInfo(r.fileContaining(ident))
	obj := pkgInfo.ObjectOf(ident)

	if obj == nil && !r.IsPackageName(ident) {
		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}

	if IsMethod(obj) {
		// If obj is a method, search across interfaces: there may be
		// many other methods that need to change to ensure that all
		// types continue to implement the same interfaces
		return r.reachableMethods(ident, obj.(*types.Func), r.program.AllPackages[obj.Pkg()]), nil
	} else {
		// If obj is not a method, then only one object needs to change
		return map[types.Object]bool{obj: true}, nil
	}

}

// IsMethod reports whether obj is a method.
func IsMethod(obj types.Object) bool {
	return MethodReceiver(obj) != nil
}

// MethodReceiver returns the receiver if obj is a method and nil otherwise.
func MethodReceiver(obj types.Object) *types.Var {
	if obj, ok := obj.(*types.Func); ok {
		return obj.Type().(*types.Signature).Recv()
	}

	return nil
}

// reachableMethods receives an object for a method (i.e., a types.Func with
// a non-nil receiver) and the PackageInfo in which it was declared and returns
// a set of objects that must be renamed if that method is renamed.
func (r *SearchEngine) reachableMethods(ident *ast.Ident, obj *types.Func, pkgInfo *loader.PackageInfo) map[types.Object]bool {
	// Find methods and interfaces defined in the given package that have
	// the same signature as the argument method (obj)
	sig := obj.Type().(*types.Signature)
	methods, interfaces := r.methodDeclsMatchingSig(ident, sig, pkgInfo)

	// Map methods to interfaces their receivers implement and vice versa
	methodInterfaces := map[types.Object]map[*types.Interface]bool{}
	interfaceMethods := map[*types.Interface]map[types.Object]bool{}
	for iface := range interfaces {
		interfaceMethods[iface] = map[types.Object]bool{}
	}
	for method := range methods {
		methodInterfaces[method] = map[*types.Interface]bool{}
		recv := MethodReceiver(method).Type()
		for iface := range interfaces {
			if types.Implements(recv, iface) {
				methodInterfaces[method][iface] = true
				interfaceMethods[iface][method] = true
			}
		}
	}

	// The two maps above define a bipartite graph with edges between
	// methods and the interfaces implemented by their receivers.  Perform
	// a breadth-first search of this graph, starting from obj, to find the
	// reflexive, transitive closure of methods affected by renaming obj.
	affectedMethods := map[types.Object]bool{obj: true}
	affectedInterfaces := map[*types.Interface]bool{}
	queue := []interface{}{obj}
	for i := 0; i < len(queue); i++ {
		switch elt := queue[i].(type) {
		case *types.Func:
			for iface := range methodInterfaces[elt] {
				if !affectedInterfaces[iface] {
					affectedInterfaces[iface] = true
					queue = append(queue, iface)
				}
			}
		case *types.Interface:
			for method := range interfaceMethods[elt] {
				if !affectedMethods[method] {
					affectedMethods[method] = true
					queue = append(queue, method)
				}
			}
		}
	}

	return affectedMethods
}

// methodDeclsMatchingSig walks all of the ASTs in the given package and
// returns methods with the given signature and interfaces that explicitly
// define a method with the given signature.
// TODO(review D7): This looks quite expensive to do in a relatively low-level
// function. Consider doing an initial pass over the ASTs to gather this
// information if performance becomes an issue.
// TODO(review D7): Two identifiers are identical iff (a) they are spelled the
// same and (b) they are exported or they appear within the same package. So
// really you need to know ident's package too, construct a types.Id instance
// for each side, and compare those.
// I doubt it's a major practical problem in this case, but it's something
// important corner case to bear in mind if you're building Go tools. It means
// you can have a legal struct or interface with two fields/methods both named
// "f", if they come from different packages.
func (r *SearchEngine) methodDeclsMatchingSig(ident *ast.Ident, sig *types.Signature, pkgInfo *loader.PackageInfo) (methods map[types.Object]bool, interfaces map[*types.Interface]bool) {
	methods = map[types.Object]bool{}
	interfaces = map[*types.Interface]bool{}
	for _, file := range pkgInfo.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.InterfaceType:
				iface := pkgInfo.TypeOf(n).Underlying().(*types.Interface)
				interfaces[iface] = true
				for i := 0; i < iface.NumExplicitMethods(); i++ {
					method := iface.ExplicitMethod(i)
					methodSig := method.Type().(*types.Signature)
					if method.Name() == ident.Name && types.Identical(sig, methodSig) {
						methods[method] = true
					}
				}
			case *ast.FuncDecl:
				obj := pkgInfo.ObjectOf(n.Name)
				fnSig := obj.Type().Underlying().(*types.Signature)
				if fnSig.Recv() != nil && n.Name.Name == ident.Name && types.Identical(sig, fnSig) {
					methods[obj] = true
				}
			}
			return true
		})
	}
	return methods, interfaces
}

/* -=-=- Search by Identifier  -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// TODO(review D7): I think I mentioned this before but this function has a
// strange signature: it mixes objects from two non-adjacent layers of the
// design abstraction: semantic objects (e.g. types.Object) and concrete syntax
// (text.OffsetLength). In between these two layers is that of abstract syntax (e.g.
// ast.Ident). This suggests that the function is doing too much; perhaps it
// should just be returning a set of *ast.Idents for a later function to map
// down to concrete syntax.

// FindOccurrences finds the location of all identifiers that are direct or
// indirect references to the same object as given identifier.  The returned
// map maps filenames to a slice of (offset, length) pairs describing locations
// at which the given identifier is referenced.
func (r *SearchEngine) FindOccurrences(ident *ast.Ident) (map[string][]text.OffsetLength, error) {

	var pkgs map[*loader.PackageInfo]bool
	var result map[string][]text.OffsetLength

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if obj == nil && !r.IsPackageName(ident) {

		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}
	if r.IsPackageName(ident) {
		result = r.occurrencesofpkg(ident)
		pkgs = allPackages(r.program)

	} else {

		var decls map[types.Object]bool
		if IsMethod(obj) {
			var err error
			decls, err = r.FindDeclarationsAcrossInterfaces(ident)
			if err != nil {
				return nil, err
			}
		} else {
			decls = map[types.Object]bool{obj: true}
		}

		result = r.occurrences(decls)
		pkgs = r.packages(decls)
	}

	return r.occurrencesInComments(ident.Name, pkgs, result), nil
}

// occurrences returns the source locations of all identifiers that resolve
// to one of the given objects.
func (r *SearchEngine) occurrences(decls map[types.Object]bool) map[string][]text.OffsetLength {
	result := make(map[string][]text.OffsetLength)
	for pkgInfo := range r.packages(decls) {
		for id, obj := range pkgInfo.Defs {
			if decls[obj] {
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
		for id, obj := range pkgInfo.Uses {
			if decls[obj] {
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
	}

	return result
}

func (r *SearchEngine) occurrencesofpkg(ident *ast.Ident) map[string][]text.OffsetLength {

	result := make(map[string][]text.OffsetLength)

	for pkgInfo := range allPackages(r.program) {
		for id, obj := range pkgInfo.Defs {
			if obj == nil && id.Name == ident.Name {
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
		for id, obj := range pkgInfo.Uses {
			if obj == nil && id.Name == ident.Name {
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
	}

	return result

}

func (r *SearchEngine) position(id *ast.Ident) token.Position {
	return r.program.Fset.Position(id.NamePos)
}

func (r *SearchEngine) offsetLength(id *ast.Ident) text.OffsetLength {
	position := r.position(id)
	offset := position.Offset
	length := len(id.Name)
	return text.OffsetLength{offset, length}
}

// packages returns a set of PackageInfos that may reference the given
// Objects.  If at least one of the given declarations is exported, the method
// returns all the packages of this program; otherwise, it returns the
// package(s) containing the given declarations.
// TODO(review D7); If performance is a concern, you could return only the
// packages in the reverse transitive closure of the package import graph,
// rather than all the packages.
func (r *SearchEngine) packages(decls map[types.Object]bool) map[*loader.PackageInfo]bool {
	pkgs := make(map[*loader.PackageInfo]bool)
	for decl := range decls {
		if decl.Exported() {
			return allPackages(r.program)
		}
		pkgInfo := r.program.AllPackages[decl.Pkg()]
		pkgs[pkgInfo] = true
	}
	return pkgs
}

func (r *SearchEngine) pkgInfoForPkg(pkg *types.Package) *loader.PackageInfo {
	return r.program.AllPackages[pkg]
}

func allPackages(prog *loader.Program) map[*loader.PackageInfo]bool {
	pkgs := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range prog.AllPackages {
		pkgs[pkgInfo] = true
	}
	return pkgs
}

// occurrencesincomments checks if the name of the selected identifier occurs as a word in comments,if true then
// all the source locations of name in comments are returned.
func (r *SearchEngine) occurrencesInComments(name string, pkgs map[*loader.PackageInfo]bool, result map[string][]text.OffsetLength) map[string][]text.OffsetLength {
	for pkgInfo := range pkgs {
		for _, f := range pkgInfo.Files {
			for _, comment := range f.Comments {
				if strings.Contains(comment.List[0].Text, name) {
					result = r.occurrencesInFileComments(f, comment, name, result, r.program)
				}
			}
		}
	}
	return result
}

// occurrencesInFileComments finds the source location of  selected identifier names in
// comments, appends them to the already found source locations of
// selected identifier objects (result), and returns the result.
func (r *SearchEngine) occurrencesInFileComments(f *ast.File, comment *ast.CommentGroup, name string, result map[string][]text.OffsetLength, prog *loader.Program) map[string][]text.OffsetLength {
	var whitespaceindex int = 1
	regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/*%s[\\PL]|[\\PL]%s$", name, name, name, name)
	re := regexp.MustCompile(regexpstring)
	matchcount := strings.Count(comment.List[0].Text, name)
	for _, matchindex := range re.FindAllStringIndex(comment.List[0].Text, matchcount) {
		offset := prog.Fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex
		length := len(name)
		filename := prog.Fset.Position(f.Pos()).Filename
		result[filename] = append(result[filename], text.OffsetLength{offset, length})
	}
	return result
}

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func (r *SearchEngine) IsPackageName(ident *ast.Ident) bool {
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	if r.pkgInfo(r.fileContaining(ident)).Pkg.Name() == ident.Name && obj == nil {
		return true
	}

	return false
}

// TODO: These are duplicated from refactoring.go
func (r *SearchEngine) fileContaining(node ast.Node) *ast.File {
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

func (r *SearchEngine) pkgInfo(file *ast.File) *loader.PackageInfo {
	for _, pkgInfo := range r.program.AllPackages {
		for _, thisFile := range pkgInfo.Files {
			if thisFile == file {
				return pkgInfo
			}
		}
	}
	return nil
}
