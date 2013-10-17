package main

import "fmt"

var hello = ":-(" // This is a different hello

// Test for renaming the local variable hello
func main() {
	hello = ":-)"  // Don't change this

	var hello string = "Hello"	// <<<<< 11,6,11,6,renamed,pass
	var world string = "world"	// <<<<< 12,6,12,6,hello,fail
	hello = hello + ", " + world
	hello += "!"
	fmt.Println(hello)
}
