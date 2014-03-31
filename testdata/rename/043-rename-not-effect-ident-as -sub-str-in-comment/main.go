//This testcase is to rename the varibale hello but not helloworld
package main

import "fmt"

var hello = ":-(" // This is a different hello

// Test for renaming the local variable hello,only hello variable should be changed to renamed  
func main() {
	hello = ":-)"  // Don't change this 

	var hello string = "Hello"	// <<<<< rename,12,6,12,6,renamed,pass
	var world string = "world"	
	hello = hello + ", " + world
	hello += "!"
	fmt.Println(hello)
}
