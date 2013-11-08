package main // <<<<< fiximports,1,1,1,1,fail

import (
	"fmt"
	"math/rand"
)

func main() {
  r := rand.New(rand.NewSource(99))
  fmt.Println(r.Float32())
  bytes()
}
