// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package names provides search facilities required by the Rename refactoring,
// including the ability to find references to a particular name.
package names

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"regexp/syntax"
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

	if obj == nil && !r.IsPackageName(ident) && !r.IsSwitchVar(ident) {
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
func (r *SearchEngine) FindOccurrences(ident *ast.Ident) (map[string][]text.Extent, error) {

	var pkgs map[*loader.PackageInfo]bool
	var result map[string][]text.Extent

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)

	if obj == nil && !r.IsPackageName(ident) && !r.IsSwitchVar(ident) {

		return nil, fmt.Errorf("Unable to find declaration of %s", ident.Name)
	}
     
        if r.IsSwitchVar(ident) {
       
            fmt.Println("selected switch var inside the names")
            return r.SwitchRename(ident),nil  
         }       
	if r.IsPackageName(ident) {
                 //fmt.Println("selected package name")
		return r.PackageRename(ident.Name), nil
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


func (r *SearchEngine) SwitchRename(ident *ast.Ident) map[string][]text.Extent {
  //TODO change to perform switch and case variable rename
	result := r.occurrencesofCaseVar(ident.Name)
	pkgs := allPackages(r.program)
	return r.occurrencesInComments(ident.Name, pkgs, result)

}


func (r *SearchEngine) PackageRename(identName string) map[string][]text.Extent {

	result := r.occurrencesofpkg(identName)
	pkgs := allPackages(r.program)
	return r.occurrencesInComments(identName, pkgs, result)

}

// occurrences returns the source locations of all identifiers that resolve
// to one of the given objects.
func (r *SearchEngine) occurrences(decls map[types.Object]bool) map[string][]text.Extent {
	result := make(map[string][]text.Extent)
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

//TODO : Make the search robust for packagenames in importspec
func (r *SearchEngine) occurrencesofpkg(identName string) map[string][]text.Extent {

	result := make(map[string][]text.Extent)
	for pkgInfo := range allPackages(r.program) {
		for id, obj := range pkgInfo.Defs {
			if obj == nil && id.Name == identName {

				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}
		for id, obj := range pkgInfo.Uses {
			if (obj == nil || obj.Name() == identName) && id.Name == identName {

				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}

		for node, pkgObject := range pkgInfo.Implicits {
                        
			if pkgObject.Name() == identName {
                                  
                                
				filename := r.positionofObject(pkgObject).Filename

				result[filename] = append(result[filename],
					r.offsetLengthofObject(node, pkgObject))
			}
                               
                  for _, file := range pkgInfo.Files {

			ast.Inspect(file, func(node ast.Node) bool {
				switch n := node.(type) {
	                            case  *ast.ImportSpec:
                                   if n.Name != nil && strings.Replace(n.Path.Value, "\"", "", 2) == identName {
                                         //fmt.Println("pkg name with local rename")
                                           filename := r.positionofPkg(n.Path).Filename

				result[filename] = append(result[filename],
					r.offsetLengthofPkg(n.Path))


   
                                 }

                        }
                     return true
                 })
          } 
               
                             
     


                         

   
      
  
		}

	}

	return result
}



//TODO : Make the search robust for packagenames in importspec
func (r *SearchEngine) occurrencesofCaseVar(identName string) map[string][]text.Extent {

	result := make(map[string][]text.Extent)
	for pkgInfo := range allPackages(r.program) {
		
		for id, obj := range pkgInfo.Uses {
			if (obj == nil || obj.Name() == identName) && id.Name == identName {
			//fmt.Println("slected  case var and types.var is",obj.(*types.Var))
				filename := r.position(id).Filename
				result[filename] = append(result[filename],
					r.offsetLength(id))
			}
		}

		for node, pkgObject := range pkgInfo.Implicits {
                        
			if pkgObject.Name() == identName {
                                  
                               //fmt.Println("slected  case var and types.var is",obj.(*types.Var)) 
				filename := r.positionofObject(pkgObject).Filename

				result[filename] = append(result[filename],
					r.offsetLengthofObject(node, pkgObject))
			}
      
  
		}

	}

	return result
}











func (r *SearchEngine) position(id *ast.Ident) token.Position {
	return r.program.Fset.Position(id.NamePos)
}
func (r *SearchEngine) positionofObject(pkgObject types.Object) token.Position {
	return r.program.Fset.Position(pkgObject.Pos())
}

func (r *SearchEngine) positionofPkg(id *ast.BasicLit) token.Position {
	return r.program.Fset.Position(id.ValuePos)
}

func (r *SearchEngine) offsetLength(id *ast.Ident) text.Extent {
	position := r.position(id)
	offset := position.Offset
	length := len(id.Name)
	return text.Extent{offset, length}
}

func (r *SearchEngine) offsetLengthofObject(node ast.Node, obj types.Object) text.Extent {

	var offset int
	position := r.positionofObject(obj)
      
	offset = position.Offset + 1
	length := len(obj.Name())

	switch ident := node.(type) {
	case *ast.ImportSpec:

		if strings.Replace(ident.Path.Value, "\"", "", 2) != obj.Name() {

			offset = position.Offset + len(strings.Replace(ident.Path.Value, "\"", "", 2)) - len(obj.Name()) + 1
		}
            
        case *ast.CaseClause:
           
             offset = offset-1   
	}

	return text.Extent{offset, length}
}



func (r *SearchEngine) offsetLengthofPkg(id *ast.BasicLit) text.Extent {

	var offset int
	position := r.positionofPkg(id)
	offset = position.Offset + 1
	length := len(id.Value)-2

       //fmt.Println("offset , length ", offset, length)

	/*switch ident := node.(type) {
	case *ast.ImportSpec:

		if strings.Replace(ident.Path.Value, "\"", "", 2) != obj.Name() {

			offset = position.Offset + len(strings.Replace(ident.Path.Value, "\"", "", 2)) - len(obj.Name()) + 1
		}
	}*/

	return text.Extent{offset, length}
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

// occurrencesInComments checks if the name of the selected identifier occurs as a
// word in comments, if true then all the source locations of name in comments are returned.
func (r *SearchEngine) occurrencesInComments(name string, pkgs map[*loader.PackageInfo]bool, result map[string][]text.Extent) map[string][]text.Extent {
	T := kmpFailure(name) // precompute failure function once
	len := len(name)
	for pkgInfo := range pkgs {
		for _, f := range pkgInfo.Files {
			fname := r.program.Fset.Position(f.Pos()).Filename
			for _, comment := range f.Comments {
				for _, c := range comment.List {
					offsets := kmpWord(c.Text, name, T)
					for _, o := range offsets {
						foffset := r.program.Fset.Position(c.Slash).Offset + o
						result[fname] = append(result[fname], text.Extent{foffset, len})
					}
				}
			}
		}
	}
	return result
}

// kmpWord is the Knuth-Morris-Pratt string searching algorithm,
// slightly modified to only find entire word matches.
func kmpWord(txt, pat string, T []int) (offsets []int) {
	M, N := len(pat), len(txt)
	for m, i := 0, 0; i < N; i++ {
		for m > 0 && txt[i] != pat[m] {
			m = T[m]
		}
		m++
		if m >= M {
			if isWord(txt, i-m+1, i) { // could probably compute in failure func...
				offsets = append(offsets, i-m+1)
			}
			m = 0
		}
	}
	return offsets
}

// occurrencesInFileComments finds the source location of  selected identifier
// names in comments, appends them to the already found source locations of
// selected identifier objects (result), and returns the result.
func (r *SearchEngine) occurrencesInFileComments(f *ast.File, comment *ast.CommentGroup, name string, result map[string][]text.Extent, prog *loader.Program) map[string][]text.Extent {
	var whitespaceindex int = 1
	var offset int
	//regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/*%s[\\PL]|[\\PL]%s$", name, name, name, name)
	regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/\\*%s[\\PL]|[\\PL]%s$", name, name, name, name)
	re := regexp.MustCompile(regexpstring)
	matchcount := strings.Count(comment.List[0].Text, name)
	for _, matchindex := range re.FindAllStringIndex(comment.List[0].Text, matchcount) {
		if matchindex[0] == 0 {
			offset = prog.Fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex + 1
		} else {
			offset = prog.Fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex
		}
		length := len(name)
		filename := prog.Fset.Position(f.Pos()).Filename
		result[filename] = append(result[filename], text.Extent{offset, length})
	}
	return result
}

// This function assumes it is given indices corresponding to a word,
// and i, n are the beginning and end of that word, respectively.
func isWord(txt string, i, n int) bool {
	if i == 0 {
		return !syntax.IsWordChar(rune(txt[n+1]))
	} else if n == len(txt)-1 {
		return !syntax.IsWordChar(rune(txt[i-1]))
	}
	return !syntax.IsWordChar(rune(txt[i-1])) && !syntax.IsWordChar(rune(txt[n+1]))
}

// kmpFailure is the "failure function" for KMP.
func kmpFailure(pat string) (T []int) {
	T = make([]int, len(pat))
	T[0] = -1

	for i := 1; i < len(pat); i++ {
		T[i] = T[i-1] + 1
		for T[i] > 0 && pat[i] != pat[T[i]-1] {
			T[i] = T[T[i]-1] + 1
		}
	}
	return T
}

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func (r *SearchEngine) IsPackageName(ident *ast.Ident) bool {
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	if r.pkgInfo(r.fileContaining(ident)).Pkg.Name() == ident.Name && obj == nil {
		return true
	}

	return false
}

func (r *SearchEngine) IsSwitchVar(ident *ast.Ident) bool {
	//pkginfo := r.pkgInfo(r.fileContaining(ident))
	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	          
         if   _, ok := obj.(*types.Var); !ok  && obj == nil && !r.IsPackageName(ident) {
			//fmt.Println("types.var of ident",v)
                          //fmt.Println("selected var in switch  clasue of type switch ")    
                   // fmt.Println("slected  switch var and types.var is",obj.(*types.Var))
                        return true
		}
//TODO Other identifiers mayhave types.Var with nil object , this might be potential problem 
 // need to look deeply to differentiate a case clause identifier of type switch from all other identifiers including 
 //type switch identifier itself
                   
         /*if   _, ok := obj.(*types.Var); ok  && obj != nil && !r.IsPackageName(ident) {
			fmt.Println("selected var in case clasue of type switch ")
                    //fmt.Println("slected  switch var and types.var is",obj.(*types.Var))
                        return true
		} */

  /* if _, ok := pkginfo.Implicits[ident].(*types.Var); ok {
       fmt.Println("selected var in case clasue of type switch ")
        return true
    }*/


           
              
        //fmt.Println("slected  switch var and types.var is",obj.(*types.Var))
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
