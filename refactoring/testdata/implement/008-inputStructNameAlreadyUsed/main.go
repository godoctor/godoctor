package main // <<<<< stubInterface,1,1,1,1,Queue,Apple,pass

import "fmt"

func main() {
	fmt.Println("works")
}

type Queue interface {
	Enqueue(x int)
	Dequeue() int
}

type Apple struct {
}

func (a *Apple) potatoe() {
}
