//<<<<<extract,10,2,14,15,Foo,pass
package main

import "fmt"

func main() {
	a := 3
	b := a
	c := 1
	for a <= b {
		a += b
	}
	x := a + b
	fmt.Println(x)
	z := a + x + c
	fmt.Println(z)
}
