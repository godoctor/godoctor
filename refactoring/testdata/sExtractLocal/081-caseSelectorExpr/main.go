package main

import "fmt"

func main() {

	var apple math

	apple.x = 10
	apple.y = 5

	switch {
	case apple.y < apple.x: // <<<<< extractLocal,13,7,13,11,newVar,pass
		fmt.Printf("this is a test and only a test :D")
	case apple.y > 10:
		fmt.Printf("welcome to crazy town")
	default:
	}
}

type math struct {
	x int
	y int
}
