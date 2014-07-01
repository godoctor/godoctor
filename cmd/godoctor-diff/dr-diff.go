// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements a command line tool that reads two text files, computes
// differences between them, and writes a unified diff to standard output.  It
// is intended for debugging, not for production use.

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"

	"golang-refactoring.org/go-doctor/text"
)

var helpFlag = flag.Bool("h", false, "Prints this help information")

func usage() {
	fmt.Printf("Usage: %s <file1> <file2>\n", os.Args[0])
}

func main() {
	flag.Parse()
	args := flag.Args()

	if *helpFlag {
		usage()
		os.Exit(0)
	}

	if len(args) != 2 {
		usage()
		os.Exit(1)
	}

	file1, err := readLines(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %s\n", args[0], err)
		os.Exit(1)
	}

	file2, err := readLines(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %s\n", args[1], err)
		os.Exit(1)
	}

	editSet := text.Diff(file1, file2)

	file1Reader, err := os.OpenFile(args[0], syscall.O_RDWR, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %s\n", args[0], err)
		os.Exit(1)
	}
	defer file1Reader.Close()

	patch, err := editSet.CreatePatch(bufio.NewReader(file1Reader))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %s\n", args[0], err)
		os.Exit(1)
	}

	patch.Write(args[0], args[1], os.Stdout)

	os.Exit(0)
}

func readLines(filename string) ([]string, error) {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Split(scanLinesWithEOL)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// scanLinesWithEOL is similar to bufio.ScanLines, but it retains line
// terminators: it is a split function for a Scanner that returns each line of
// text, including any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
func scanLinesWithEOL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0 : i+1], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
