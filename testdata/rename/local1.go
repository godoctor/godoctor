package main

import "fmt"

var hello = ":-(" // This is a different hello

// Test for renaming the local variable hello
func main() {
	hello = ":-)"  // Change the global first

	var hello string = "Hello"
	hello = hello + ", world"
	hello += "!"
	fmt.Println(hello)
}
