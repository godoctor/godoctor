// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow_test

import (
	"fmt"
	"go/ast"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/analysis/loader"
	"golang.org/x/tools/go/packages"
)

func ExampleDefsReaching() {
	src := `
    package main

    import "fmt"

    func main() {
      a := 1
      b := 2
      c := 3
      a := b
      a, b := b, a
      c := a + b
    }
  `

	// can ignore overlay stuff, you can just load from disk normally
	var config packages.Config
	config.Dir = "testdata"
	config.Overlay = map[string][]byte{"main.go": []byte(src)}

	prog, err := loader.Load(&config)
	if err != nil {
		return
	}

	var info *packages.Package
	for _, info = range prog.AllPackages {
		if info.Name == "main" {
			break
		}
	}

	funcOne := info.Syntax[0].Decls[1].(*ast.FuncDecl)
	c := cfg.FromFunc(funcOne)
	du := dataflow.DefUse(c, info)

	ast.Inspect(info.Syntax[0], func(n ast.Node) bool {
		switch stmt := n.(type) {
		case ast.Stmt:
			fmt.Println(len(du[stmt]))
			// do as you please
		}
		return true
	})
}

func ExampleLiveVars() {
	src := `
    package main

    import "fmt"

    func main() {
      a := 1
      b := 2
      c := 3
      a := b
      a, b := b, a
      c := a + b
    }
  `

	// can ignore overlay stuff, you can just load from disk normally
	var config packages.Config
	config.Dir = "testdata"
	config.Overlay = map[string][]byte{"main.go": []byte(src)}

	prog, err := loader.Load(&config)
	if err != nil {
		return
	}

	var info *packages.Package
	for _, info = range prog.AllPackages {
		if info.Name == "main" {
			break
		}
	}

	funcOne := info.Syntax[0].Decls[1].(*ast.FuncDecl)
	cfg := cfg.FromFunc(funcOne)
	in, out := dataflow.LiveVars(cfg, info)

	ast.Inspect(info.Syntax[0], func(n ast.Node) bool {
		switch stmt := n.(type) {
		case ast.Stmt:
			_, _ = in[stmt], out[stmt]
			// do as you please
		}
		return true
	})
}
