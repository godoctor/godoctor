package main

import "fmt"

func main() {
	x := new(Node)
	if x.next == nil { // <<<<< extractLocal,7,15,7,18,newVar,fail
		fmt.Println("there's a name in there")
	}
}

type Node struct {
	next *Node
}
