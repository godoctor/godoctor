//<<<<< extract,14,2,16,25,Foo,pass
package main

import "fmt"

type Pt struct {
	x, y int
}

func main() {
	p := Pt{3, 4}
	fmt.Println("Old Pt", p)
	p.x = 22
	fmt.Println("Here starts the extraction")
	p.y = 66
	fmt.Println("New Pt", p)
}
