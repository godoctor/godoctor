package main // <<<<< stubInterface,1,1,1,1,Queue,MyQueueStruct,pass

import "fmt"

func main() {
	fmt.Println("works")
}

type Queue interface {
	Enqueue(x int)
	Dequeue() int
	ThreeQueue(x int, y string, z bool) int
}
