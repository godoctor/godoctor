package main // <<<<< stubInterface,1,1,1,1,io.WriterAt,WriteAtStruct,pass

import (
	"fmt"
	"io"
)

func main() {
	fmt.Println("works")

}

type Queue interface {
	Enqueue(x int)
	Dequeue() int
	ThreeQueue(x int) int
}

type Apple struct {
	
}

func (a *Apple) funcWrite (w io.Writer) {
	fmt.Fprintln(w, "Hello")
}
