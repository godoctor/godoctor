package main

import "fmt"

// Test for renaming a struct field
type myStruct struct {
	Name string // <<<<< rename,7,5,7,5,renamed,pass
}

func main() {
	m := myStruct{Name: "Foo"}
	fmt.Println(m.Name)
}
