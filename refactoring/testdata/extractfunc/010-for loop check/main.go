//<<<<<extract,8,2,16,59,Foo,pass
package main

import "fmt"

func main() {
	fmt.Println("statement inside the main function")
	for i := 0; i <= 8; i++ {
		fmt.Println("i is ", i)
		if i == 5 {
			goto ABC
		}
	}
	fmt.Println("statement after for loop")
ABC:
	fmt.Println("this is the statement under the label 'ABC'")
}
