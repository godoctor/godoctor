package main // <<<<< stubInterface,1,1,1,1,Queue,MyQueueStruct,pass

import "fmt"

func main() {
	fmt.Println("works")
}

type Queue interface {

}

type Apple struct {
}

func (a *Apple) potatoe() {
}
