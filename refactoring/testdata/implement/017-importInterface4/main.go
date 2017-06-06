package main // <<<<< stubInterface,1,1,1,1,fmt.Scanner,ScanStruct,pass

import (
	"fmt"
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
