//<<<<<extract,9,2,17,35,Foo,pass
package main

import "fmt"

func main() {
	i := 5
	i++
	if i == 6 {
		goto ABC
	} else if i == 8 {
		goto DEF
	}
ABC:
	fmt.Println("The value of i is 6")
DEF:
	fmt.Println("The value of i is 8")
}
