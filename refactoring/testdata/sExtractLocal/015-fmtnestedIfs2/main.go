package main

import "fmt"

func main() {
	a := 1
	b := 2
	c := 2
	if a+b < c {
		fmt.Println("")
		a = 5
		b = 6
		fmt.Println(a + b) // <<<<< extractLocal,13,15,13,19,newVar,pass
		if a+b < c {
			fmt.Println("this is the area of a rectangle: ")
		}
	}
	d := a + b
	fmt.Println(d)
}
