package main

import "fmt"

func main() {
	a := 1
	b := 2
	c := 3
	if a >= b {
		fmt.Println("this is the area of a rectangle: ")
	}
	if a <= b {
		fmt.Println("this is the area of a square: ")
	}
	if a+b < c { // <<<<< extractLocal,15,4,15,7,newVar,pass
		fmt.Println("")
	}
}
