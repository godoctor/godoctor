package main

import "fmt"

type i interface {
}

type s struct {
	name string
}

func main() {
	var v i = &s{name: "foo"}
	if w, ok := v.(*s); ok { // <<<<< extractLocal,14,14,14,19,newVar,fail
		fmt.Println(w.name)
	}
}
