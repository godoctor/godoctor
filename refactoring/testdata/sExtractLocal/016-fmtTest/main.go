package main

import "fmt"

func main() {
	a := 1
	b := 2
	c := 2
	fmt.Println(a + b + c) // <<<<< extractLocal,9,14,9,18,newVar,pass
	d := a + b
	fmt.Println(d)
}
