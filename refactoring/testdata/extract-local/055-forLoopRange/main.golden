package main

import "fmt"

func main() {
	x := make([]string, 5)
	for _, name := range x { // <<<<< extractLocal,7,6,7,13,newVar,pass
		if name != "" {
			fmt.Println("there's a name in there")
		}
	}
}
