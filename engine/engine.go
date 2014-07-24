// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package engine is the programmatic entrypoint to the Go refactoring engine.
package engine

import (
	"fmt"

	"golang-refactoring.org/go-doctor/refactoring"
)

// All available refactorings, keyed by a unique, one-short, all-lowercase name
var refactorings map[string]refactoring.Refactoring

func init() {
	refactorings = map[string]refactoring.Refactoring{
		"rename": new(refactoring.Rename),
		"toggle": new(refactoring.ToggleVar),
		"godoc":  new(refactoring.AddGoDoc),
		"debug":  new(refactoring.Debug),
		"null":   new(refactoring.Null),
	}
}

// AllRefactorings returns all of the transformations that can be performed.
// The keys of the returned map are short, single-word, all-lowercase names
// (rename, fiximports, etc.); the values implement the Refactoring interface.
func AllRefactorings() map[string]refactoring.Refactoring {
	return refactorings
}

// GetRefactoring returns a Refactoring keyed by the given short name.  The
// short name must be one of the keys in the map returned by AllRefactorings.
func GetRefactoring(shortName string) refactoring.Refactoring {
	return refactorings[shortName]
}

// AddRefactoring allows custom refactorings to be added to the refactoring
// engine.  Invoke this method before starting the command line or protocol
// driver.
func AddRefactoring(shortName string, newRefac refactoring.Refactoring) error {
	if r, ok := refactorings[shortName]; ok {
		return fmt.Errorf("The short name \"%s\" is already "+
			"associated with a refactoring (%s)",
			shortName,
			r.Description().Name)
	}
	refactorings[shortName] = newRefac
	return nil
}
