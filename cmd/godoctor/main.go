// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The godoctor command refactors Go code.
package main

import (
	"os"

	"github.com/godoctor/godoctor/engine/cli"
)

func main() {
	os.Exit(cli.Run(os.Stdin, os.Stdout, os.Stderr, os.Args))
}
