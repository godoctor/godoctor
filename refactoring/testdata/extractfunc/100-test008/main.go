//<<<<<extract,7,5,7,13,FOO,fail
package main

import "fmt"

func main() {
	if num := 9; num < 0 {
		A := 5
		fmt.Println(num, "is negative")
		A += 10
		fmt.Println("Value of A is", A)
	} else if num < 10 {
		fmt.Println(num, "has 1 digit")
	} else {
		fmt.Println(num, "has multiple digits")
	}
}
