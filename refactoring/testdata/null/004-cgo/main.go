// NOTE: this is currently broken, leaving for coverage.
// may be a bug in surfacing type checking file names,
// this file disappears and one from the cache bubbles up.
package main // <<<<< null,1,1,1,1,false,pass

/*
#include <stdlib.h>

int myVar = 42;

// simple square calculation
int sqr(int a) {
  return a * a;
}

// return a global variable
int returnMyVar() {
  return myVar;
}


*/
import "C"
import "fmt"

// NOTE: cgo doesn't work properly if import ( ... ) is used instead

func main() {
	fmt.Println(C.sqr(2))
	fmt.Println(C.returnMyVar())
	fmt.Println(C.myVar)
	fmt.Println(C.rand())
}
