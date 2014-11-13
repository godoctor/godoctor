// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// This file uses the Null refactoring as a template, and
// adds comments to a program.
// It will be for any program that needs checking for documentation.
package refactoring

import (
	"fmt"
	"go/ast"
	"path"
	"strings"

	"golang.org/x/tools/go/types"
	"github.com/godoctor/godoctor/text"
)

type ImplementIface struct {
	refactoringBase
	// interface to get methods for
	interfaceName string
	// name used to create a struct
	structureName string
}

func (r *ImplementIface) Description() *Description {
	return &Description{
		Name: "Interface stub Refactoring",
		Params: []Parameter{
			Parameter{
				Label:        "interface Name: ",
				Prompt:       "Please select a name for the interface to get the methods from.",
				DefaultValue: "",
			},
			Parameter{
				Label:        "structure Name: ",
				Prompt:       "Please select a name for the structure to be used by the interface methods.",
				DefaultValue: "",
			}},
		Hidden: false,
	}
}
func (r *ImplementIface) Run(config *Config) *Result {
	r.refactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}
	r.interfaceName = config.Args[0].(string)
	r.structureName = config.Args[1].(string)
	if r.interfaceName == "" {
		r.Log.Error("you must enter a name for the interface")
		return &r.Result
	}
	if r.structureName == "" {
		r.Log.Error("you must enter a name for the structure")
		return &r.Result
	}
	r.addStubs(config)
	return &r.Result
}

// check whether the interface to be looked at is
// in the loaded file or an import
func (r *ImplementIface) addStubs(config *Config) {

	if strings.Contains(r.interfaceName, ".") {
		r.fLoader()
	} else {
		r.fLoader2()
	}
}

// if interface is from import, use coding from go.tools
// to get all the methods
func (r *ImplementIface) fLoader() {
	splitString := strings.Split(r.interfaceName, ".")
	fileName := splitString[0]
	interfaceName := splitString[1]
	if pkg, ok := r.program.ImportMap[fileName]; ok {
		obj := pkg.Scope().Lookup(interfaceName)
		if obj != nil {
			r.msSearchnMake(obj)
		}

	}
}

// use coding from go.tools for initial file interface is in
func (r *ImplementIface) fLoader2() {
	for _, pkgInfo := range r.program.AllPackages {
		if pkgInfo != nil {
			obj := pkgInfo.Pkg.Scope().Lookup(r.interfaceName)
			if obj != nil {
				r.msSearchnMake(obj)
			}
		}
	}
}

// looks for methods and structs to see if they exist already for
// the selected interface and if not then creates them in the r.file
func (r *ImplementIface) msSearchnMake(obj types.Object) {
	structNameIfFound := ""
	typ := obj.Type().Underlying()
	if iface, ok := typ.(*types.Interface); ok {
		structNameIfFound = r.checkForStruct(iface)
		r.methodchecknCreate(iface, structNameIfFound)
		structCheck := r.checkIfStructThere()
		if structNameIfFound != "" || structCheck {
		} else {
			newStruct := "\n\ntype " + r.structureName + " struct {\n}"
			endOffset := r.program.Fset.Position(r.file.End()).Offset
			r.Edits[r.filename].Add(&text.Extent{endOffset, 0}, newStruct)
		}
	}
}

// check to see if a struct exists for the interface methods
func (r *ImplementIface) checkForStruct(iface *types.Interface) string {
	structNameIfFound := ""
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		_, structNameIfFound = r.checkForCreatedMethods(method.Name())
		if structNameIfFound != "" {
			break
		}
	}
	return structNameIfFound
}

// use method check function and create method function if
// the method isn't in the file already
func (r *ImplementIface) methodchecknCreate(iface *types.Interface, structNameIfFound string) {
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		found, _ := r.checkForCreatedMethods(method.Name())
		if found {
		} else {
			r.getParamsnResults(method, structNameIfFound)
		}
	}
}

// checks to see if a method of the interface already exists in the file
// and if so then gets the struct name it is using and also returns true
func (r *ImplementIface) checkForCreatedMethods(methodName string) (bool, string) {
	found := false
	structForMethod := ""
	for _, n := range r.file.Decls {
		switch funcName := n.(type) {
		case *ast.FuncDecl:
			if methodName == funcName.Name.Name {
				found = true
				if funcName.Recv != nil {
					typ := funcName.Recv.List[0].Type
					switch t := typ.(type) {
					case *ast.StarExpr:
						structForMethod = t.X.(*ast.Ident).Name
					case *ast.Ident:
						structForMethod = t.Name
					}
				}
			}
		}
	}
	return found, structForMethod
}

// get the input parameters and return results from the method
// if there are any, and then send to a func to create the method
func (r *ImplementIface) getParamsnResults(method *types.Func, structNameIfFound string) {
	params, results := make([]*types.Var, 0), make([]*types.Var, 0)
	varFound := false
	sig := method.Type().(*types.Signature)
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		if i == sig.Params().Len()-1 && sig.Variadic() {
			varFound = true
		}
		params = append(params, p)
	}
	for i := 0; i < sig.Results().Len(); i++ {
		r := sig.Results().At(i)
		results = append(results, r)
	}
	if structNameIfFound != "" {
		r.createImportInterfaceMethods(params, results, method.Name(), structNameIfFound, varFound)
	} else {
		r.createImportInterfaceMethods(params, results, method.Name(), r.structureName, varFound)
	}
}

// check to see if the structure the user inputs
// is already created in the r.file
func (r *ImplementIface) checkIfStructThere() bool {
	found := false
	for _, n := range r.file.Decls {
		switch structName := n.(type) {
		case *ast.GenDecl:
			for _, spec := range structName.Specs {
				if typespec, ok := spec.(*ast.TypeSpec); ok {
					if typespec.Name.Name == r.structureName {
						found = true
					}
				}
			}
		}
	}
	return found
}

// create the methods of the imported interface
func (r *ImplementIface) createImportInterfaceMethods(parameters []*types.Var, results []*types.Var, methodName string, structName string, varFound bool) {
	paramsName, newMethod, resultsName, resultsString := "", "", "", ""
	resultHasString := false
	counter := 0
	if cap(parameters) != 0 && cap(results) == 0 {
		paramsName = r.paramsString(parameters, varFound)
		newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s(%s) {\n\n}", structName, methodName, paramsName)
	} else if cap(results) != 0 && cap(parameters) == 0 {
		resultsName, resultsString, counter, resultHasString = r.resultsString(results)
		if counter == 1 && resultHasString == false {
			newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s() %s {\n\treturn%s\n}", structName, methodName, resultsName, resultsString)
		} else if counter == 1 && resultHasString == true {
			newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s() %s {\n\tvar eString string\n\treturn%s\n}", structName, methodName, resultsName, resultsString)
		} else {
			if resultHasString == false {
				newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s() (%s) {\n\treturn%s\n}", structName, methodName, resultsName, resultsString)
			} else {
				newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s() (%s) {\n\tvar eString string\n\treturn%s\n}", structName, methodName, resultsName, resultsString)
			}
		}

	} else {
		paramsName = r.paramsString(parameters, varFound)
		resultsName, resultsString, counter, resultHasString = r.resultsString(results)
		if counter == 1 && resultHasString == false {
			newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s(%s) %s {\n\treturn%s\n}", structName, methodName, paramsName, resultsName, resultsString)
		} else if counter == 1 && resultHasString == true {
			newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s(%s) %s {\n\tvar eString string\n\treturn%s\n}", structName, methodName, paramsName, resultsName, resultsString)
		} else {
			if resultHasString == false {
				newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s(%s) (%s) {\n\treturn%s\n}", structName, methodName, paramsName, resultsName, resultsString)
			} else {
				newMethod = fmt.Sprintf("\n\nfunc (s *%s) %s(%s) (%s) {\n\tvar eString string\n\treturn%s\n}", structName, methodName, paramsName, resultsName, resultsString)
			}
		}
	}
	endOffset := r.program.Fset.Position(r.file.End()).Offset
	r.Edits[r.filename].Add(&text.Extent{endOffset, 0}, newMethod)
}

// create parameters string
func (r *ImplementIface) paramsString(parameters []*types.Var, varFound bool) string {
	paramsName := ""
	for index, _ := range parameters {
		if parameters[index].Name() != "" {
			if paramsName == "" {
				if varFound && index == len(parameters)-1 {
					parType := strings.Replace(parameters[index].Type().String(), "[]", "...", -1)
					paramsName = parameters[index].Name() + " " + path.Base(parType)
				} else {
					paramsName = parameters[index].Name() + " " + path.Base(parameters[index].Type().String())
				}
			} else {
				if varFound && index == len(parameters)-1 {
					parType := strings.Replace(parameters[index].Type().String(), "[]", "...", -1)
					paramsName = paramsName + ", " + parameters[index].Name() + " " + path.Base(parType)
				} else {
					paramsName = paramsName + ", " + parameters[index].Name() + " " + path.Base(parameters[index].Type().String())
				}
			}
		} else {
			if paramsName == "" {
				if varFound && index == len(parameters)-1 {
					parType := strings.Replace(parameters[index].Type().String(), "[]", "...", -1)
					paramsName = path.Base(parType)
				} else {
					paramsName = path.Base(parameters[index].Type().String())
				}
			} else {
				if varFound && index == len(parameters)-1 {
					parType := strings.Replace(parameters[index].Type().String(), "[]", "...", -1)
					paramsName = paramsName + ", " + parameters[index].Name() + path.Base(parType)
				} else {
					paramsName = paramsName + ", " + path.Base(parameters[index].Type().String())
				}
			}
		}
	}
	return paramsName
}

// resultsString create results string by calling the functions needed to do it
func (r *ImplementIface) resultsString(results []*types.Var) (string, string, int, bool) {
	resultsName, returnResultsType, counter := r.createStringSlice_And_Name(results)
	hasString := r.checkIfHasString(returnResultsType)
	resultsString := r.createResultString(returnResultsType)
	return resultsName, resultsString, counter, hasString
}

// createStringSlice_And_Name place results types into a string slice
func (r *ImplementIface) createStringSlice_And_Name(results []*types.Var) (string, []string, int) {
	resultsName := ""
	returnResultsType := make([]string, 0)
	counter := 0
	for _, index := range results {
		if index.Name() != "" {
			if resultsName == "" {
				resultsName = index.Name() + " " + path.Base(index.Type().String())
				counter++
				returnResultsType = append(returnResultsType, r.checkType(index))
			} else {
				resultsName = resultsName + ", " + index.Name() + " " + path.Base(index.Type().String())
				counter++
				returnResultsType = append(returnResultsType, r.checkType(index))
			}
		} else {
			if resultsName == "" {
				resultsName = path.Base(index.Type().String())
				returnResultsType = append(returnResultsType, r.checkType(index))
				counter++
			} else {
				resultsName = resultsName + ", " + path.Base(index.Type().String())
				returnResultsType = append(returnResultsType, r.checkType(index))
				counter++
			}
		}
	}

	return resultsName, returnResultsType, counter
}

// checkIfHasString check if the string slice of the results has a value of type string
func (r *ImplementIface) checkIfHasString(returnResultsType []string) bool {
	hasString := false
	for _, index := range returnResultsType {
		if index == "eString" {
			hasString = true
		}
	}
	return hasString
}

// createResultString convert the string slice to just a big string
func (r *ImplementIface) createResultString(returnResultsType []string) string {
	resultsString := " "
	for _, index := range returnResultsType {
		if resultsString == " " {
			resultsString = " " + index
		} else {
			resultsString = resultsString + ", " + index
		}
	}
	return resultsString
}

// checkType will do a typeswitch to see what type it is, and will do it for the following: int, int64, float, float64, complex, bool, string, and everything else is nil
func (r *ImplementIface) checkType(index *types.Var) string {
	switch index.Type().String() {
	case "float", "float64":
		return "0.0"
	case "int", "int64", "complex":
		return "0"
	case "bool":
		return "false"
	case "string":
		return "eString"
	default:
		return "nil"
	}

}
