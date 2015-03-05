//<<<<<extract,10,12,13,3,foo,fail
package main

import "fmt"

func main() {
	a := 7
	b := 5
	b = b + 2
	if a == b {
		fmt.Println("a and b are equal")
		b = b + 1
	}
	fmt.Println("the new value of b is ", b)
}
