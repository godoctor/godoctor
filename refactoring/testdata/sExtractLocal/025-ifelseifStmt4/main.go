package main

import "fmt"

func main() {
	x := 5
	if x >= 4 {
		if x < 0 {

		} else if x > 18 {
			fmt.Println("divisible by 5:")
		} else if x < 10 {
			fmt.Println("divisible by 5:")
		}

		if x < 0 {

		} else if x > 18 {
			fmt.Println("divisible by 5:")
		} else if x > 10 { // <<<<< extractLocal,20,12,20,18,newVar,pass
			fmt.Println("divisible by 5:")
		}

		if x < 0 {

		} else if x > 18 {
			fmt.Println("divisible by 5:")
		} else if x < 15 {
			fmt.Println("divisible by 5:")
		}
	}
}
