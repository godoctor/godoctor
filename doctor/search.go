// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
	"unicode/utf8"

	"code.google.com/p/go.tools/go/loader"
	"code.google.com/p/go.tools/go/types"
)

type SearchEngine struct {
	program *loader.Program
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
// method returns a set containing the reflexive-transitive closure of objects
// that must be renamed if the given identifier is renamed.
func (r *SearchEngine) FindDeclarationsAcrossInterfaces(ident *ast.Ident) (map[types.Object]bool, error) {
	pkgInfo := r.pkgInfo(r.fileContaining(ident))
	obj := pkgInfo.ObjectOf(ident)
	if obj == nil {
		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}

	if isMethod(obj) {
		// If obj is a method, search across interfaces: there may be
		// many other methods that need to change to ensure that all
		// types continue to implement the same interfaces
		return r.findReachableMethods(obj, r.program.AllPackages[obj.Pkg()]), nil
	} else {
		// If obj is not a method, then only one object needs to change
		return map[types.Object]bool{obj: true}, nil
	}

}

// isMethod reports whether obj is a method.
func isMethod(obj types.Object) bool {
	return methodReceiver(obj) != nil
}

// methodReceiver returns the receiver if obj is a method and nil otherwise.
func methodReceiver(obj types.Object) *types.Var {
	if fn, isFunc := obj.(*types.Func); isFunc {
		return fn.Type().(*types.Signature).Recv()
	} else {
		return nil
	}
}

// findReachableMethods receives an object for a method (i.e., a types.Func with
// a non-nil receiver) and the PackageInfo in which it was declared and returns
// a set of objects that must be renamed if that method is renamed.
func (r *SearchEngine) findReachableMethods(obj types.Object, pkgInfo *loader.PackageInfo) map[types.Object]bool {
	// Find methods and interfaces defined in the given package that have
	// the same signature as the argument method (obj)
	sig := obj.(*types.Func).Type().(*types.Signature)
	methods, interfaces := r.findMethodDeclsMatchingSig(sig, pkgInfo)

	// Map methods to interfaces their receivers implement and vice versa
	methodInterfaces := map[types.Object]map[*types.Interface]bool{}
	interfaceMethods := map[*types.Interface]map[types.Object]bool{}
	for iface := range interfaces {
		interfaceMethods[iface] = map[types.Object]bool{}
	}
	for method := range methods {
		methodInterfaces[method] = map[*types.Interface]bool{}
		recv := methodReceiver(method).Type()
		for iface := range interfaces {
			if types.Implements(recv, iface) {
				methodInterfaces[method][iface] = true
				interfaceMethods[iface][method] = true
			}
		}
	}

	// The two maps above define a graph with edges between methods and the
	// interfaces implemented by their receivers.  Perform a breadth-first
	// search of this graph, starting from obj, to find the
	// reflexive-transitive closure of methods affected by renaming obj.
	affectedMethods := map[types.Object]bool{obj: true}
	affectedInterfaces := map[*types.Interface]bool{}
	queue := []interface{}{obj}
	for i := 0; i < len(queue); i++ {
		switch elt := queue[i].(type) {
		case types.Object:
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

// findMethodDeclsMatchingSig walks all of the ASTs in the given package and
// returns methods with the given signature and interfaces that explicitly
// define a method with the given signature.
func (r *SearchEngine) findMethodDeclsMatchingSig(sig *types.Signature, pkgInfo *loader.PackageInfo) (methods map[types.Object]bool, interfaces map[*types.Interface]bool) {
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
					if types.Identical(sig, methodSig) {
						methods[method] = true
					}
				}
				return true
			case *ast.FuncDecl:
				obj := pkgInfo.ObjectOf(n.Name)
				fnSig := obj.Type().Underlying().(*types.Signature)
				if fnSig.Recv() != nil && types.Identical(sig, fnSig) {
					methods[obj] = true
				}
				return true
			default:
				return true
			}
		})
	}
	return methods, interfaces
}

/* -=-=- Search by Identifier  -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// FindOccurrences finds the location of all identifiers that are direct or
// indirect references to the same object as given identifier.  The returned
// map maps filenames to a slice of (offset, length) pairs describing locations
// at which the given identifier is referenced.
func (r *SearchEngine) FindOccurrences(ident *ast.Ident) (map[string][]OffsetLength, error) {
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	if obj == nil {
		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}

	decls := map[types.Object]bool{obj: true}
	if isMethod(obj) {
		var err error
		decls, err = r.FindDeclarationsAcrossInterfaces(ident)
		if err != nil {
			return nil, err
		}
	}

	result := r.findOccurrences(decls)
	return r.findOccurrencesInComments(ident.Name, decls, result), nil
}

// findOccurrences returns the source locations of all identifiers that resolve
// to one of the given objects.
func (r *SearchEngine) findOccurrences(decls map[types.Object]bool) map[string][]OffsetLength {
	result := make(map[string][]OffsetLength)
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

func (r *SearchEngine) position(id *ast.Ident) token.Position {
	return r.program.Fset.Position(id.NamePos)
}

func (r *SearchEngine) offsetLength(id *ast.Ident) OffsetLength {
	position := r.position(id)
	offset := position.Offset
	length := len(id.Name)
	return OffsetLength{offset, length}
}

// packages returns a set of PackageInfos that may reference the given
// Objects.  If at least one of the given declarations is exported, the method
// returns all the packages of this program; otherwise, it returns the
// package(s) containing the given declarations.
func (r *SearchEngine) packages(decls map[types.Object]bool) map[*loader.PackageInfo]bool {
	pkgs := make(map[*loader.PackageInfo]bool)
	for decl := range decls {
		if decl.Exported() {
			return r.allPackages()
		} else {
			pkgInfo := r.program.AllPackages[decl.Pkg()]
			pkgs[pkgInfo] = true
		}
	}
	return pkgs
}

func (r *SearchEngine) pkgInfoForPkg(pkg *types.Package) *loader.PackageInfo {
	return r.program.AllPackages[pkg]
}

func (r *SearchEngine) allPackages() map[*loader.PackageInfo]bool {
	pkgs := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range r.program.AllPackages {
		pkgs[pkgInfo] = true
	}
	return pkgs
}

// FindOccurrencesincomments checks if identifier occurs as a part in comments,if true then
// all the source locations of identifier  in comments are returned.

func (r *SearchEngine) findOccurrencesInComments(name string, decls map[types.Object]bool, result map[string][]OffsetLength) map[string][]OffsetLength {

	for pkgInfo := range r.packages(decls) {
		for _, f := range pkgInfo.Files {
			for _, comment := range f.Comments {

				if strings.Contains(comment.List[0].Text, name) {
					result = r.findOccurrencesInFileComments(f, comment, name, result)
				}
			}
		}
	}
	return result
}

// findOccurrencesInFileComments finds the source location of identifiers in
// comments, adds them to the already existng occurrences of
// identifier(result), and returns the result.
func (r *SearchEngine) findOccurrencesInFileComments(f *ast.File, comment *ast.CommentGroup, name string, result map[string][]OffsetLength) map[string][]OffsetLength {

	var whitespaceindex int = 1

	re := regexp.MustCompile("[^0-9A-Za-z_]hello[^0-9A-Za-z_]|//hello[^0-9A-Za-z_]|/*hello[^0-9A-Za-z_]|[^0-9A-Za-z_]hello$")
	matchcount := strings.Count(comment.List[0].Text, name)

	for _, matchindex := range re.FindAllStringIndex(comment.List[0].Text, matchcount) {

		offset := r.program.Fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex
		length := utf8.RuneCountInString(name)
		filename := r.program.Fset.Position(f.Pos()).Filename
		result[filename] = append(result[filename], OffsetLength{offset, length})

	}

	return result
}

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */
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
