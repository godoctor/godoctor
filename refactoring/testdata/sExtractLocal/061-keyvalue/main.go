package main

import "fmt"

func main() {
	x := 2
	y := 5
	var m map[string]int     // map for key/value
	m = make(map[string]int) // initialize the map
	m["route"] = 66          //<<<<< extractLocal,10,4,10,10,newVar,pass
	fmt.Println("please choose: x + x, x * y?")
	//fmt.Printf("map keyvalue is: %s and route", m["route"])
	fmt.Printf("x is %s, y is %s", x, y)
}
