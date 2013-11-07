package main

import "fmt"
import "reflect"

func main() {
  i := 3+4.5    // <<<<< shortassign,7,3,7,12,pass
  fmt.Println("type of i is", reflect.TypeOf(i))
  fmt.Println("value of i is", i)
}
