package main

import "fmt"

func main() {
	num := run()
	fmt.Println(num)
}

func run() int {
	fmt.Println("works")
	return 1 + 2 // <<<<< extractLocal,12,8,12,13,newVar,pass
}
