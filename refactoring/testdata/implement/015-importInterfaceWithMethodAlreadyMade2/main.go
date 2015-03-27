package main // <<<<< stubInterface,1,1,1,1,os.FileInfo,FileInfoStruct,pass

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("works")
	os.Exit(0)
}

type Queue interface {
	Enqueue(x int)
	Dequeue() int
	ThreeQueue(x int) int
}

type Apple struct {

}

func (s *Apple) Sys() interface{} {
	return nil
}
