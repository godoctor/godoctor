package main // <<<<< fiximports,1,1,1,1,fail

// bytes and reflect are unused

import "bytes"
import "fmt"
import "math/rand"
import "reflect"

func main() {
  r := rand.New(rand.NewSource(99))
  fmt.Println(r.Float32())
}
