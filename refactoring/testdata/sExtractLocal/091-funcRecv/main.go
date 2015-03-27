package main

import "fmt"

func apple() string {
		a := "apple +"
		return a
}

type fruit struct {
		name string
}

func (f *fruit) orange() string {// <<<<< extractLocal,14,7,14,14,newVar,fail
	return "helloz worldz"
}

func main() {

		o2 := fruit{"os"}
	s := o2.orange()
	fmt.Println(s)
}
