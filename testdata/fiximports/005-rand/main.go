package main // <<<<< fiximports,1,1,1,1,fail

import "fmt"
//import "math/rand"

// There are two rand packages...
func main() {
  r := rand.New(rand.NewSource(99))
  fmt.Println(r.Float32())
}
