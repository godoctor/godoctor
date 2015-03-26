//<<<<<extract,8,2,20,2,Foo,pass
package main

import "fmt"

func main() {
	j := 4
	switch j {
	case 1, 3, 5, 7, 9:
		fmt.Println(j, "is odd")
	case 0, 2, 4, 6, 8:
		fmt.Println(j, "is even")
		if j == 4 {
			fmt.Println("this statement is executed")
			break
		}
		fallthrough
	default:
		fmt.Println("DEFAULT STATEMENT EXECUTED!")
	}
}
