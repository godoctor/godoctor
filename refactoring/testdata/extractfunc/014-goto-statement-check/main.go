//<<<<<extract,8,2,24,25,Foo,pass
package main

import "fmt"

func main() {
	i := 5
	for i < 10 {
		i++
		if i == 8 {
			goto ABC
		} else if i == 18 {
			goto DEF
		} else {
			fmt.Println(i)
		}
	}
	fmt.Println("this must be not executed all the time")
ABC:
	fmt.Println("The value of i is 8")
DEF:
	fmt.Println("The value of i is 18")
	i = 99
	fmt.Println("i is ", 99)
}
