package main // <<<<< stubInterface,1,1,1,1,go/ast.Visitor,VisitStruct,pass

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	fmt.Println("works")
	src := `
package main // <<<<< stubInterface,1,1,1,1,pass

import "fmt"

func main() {
	fmt.Println("works")
}

type Queue interface {
	Enqueue(x int)
	Dequeue() int
	ThreeQueue(x int) int
}
	`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	ast.Print(fset, f)
}

type Queue interface {
	Enqueue(x int)
	Dequeue() int
	ThreeQueue(x int) int
}

type Apple struct {

}