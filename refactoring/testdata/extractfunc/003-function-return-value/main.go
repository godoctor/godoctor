// <<<<<extract,9,2,11,38,changeB,pass
package main

import "fmt"

func main() {
	a := 5 + 0
	b := 4 + 0
	fmt.Println("Value of a and b", a, b)
	b = 77
	fmt.Println("Modified Value of b", b)
}
