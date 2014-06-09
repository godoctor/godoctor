package main

import "fmt"

var hello = ":-(" // This is a different hello

// Test for renaming the local variable hello
func hi() {
	hello = ":-)" // Don't change this

	var helooooooooo string = "Hello" // <<<<< rename,11,6,11,6,renamed,pass
	var world string = "world"   // <<<<< rename,12,6,12,6,hello,fail
	helooooooooo = helooooooooo + ", " + world
	helooooooooo += "!"
	fmt.Println(helooooooooo)
}
