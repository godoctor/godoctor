// Copyright 2015-2018 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"sort"
	"strings"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/willf/bitset"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/loader"
)

// File defines reaching definitions for a statement level
// control flow graph.
//
// based on algo from ch 9.2, p.607 Dragonbook, v2.2,
// "Iterative algorithm to compute reaching definitions":
//
// OUT[ENTRY] = {};
// for(each basic block B other than ENTRY) OUT[B] = {};
// for(changes to any OUT occur)
//    for(each basic block B other than ENTRY) {
//      IN[B] = Union(P a pred of B) OUT[P];
//      OUT[B] = gen[b] Union (IN[B] - kill[b]);
//    }

// DefUse builds reaching definitions for a given control flow graph, returning
// a map that maps each statement that defines a variable (i.e., declares or
// assigns it) to the set of statements that use that variable.
//
// Note: An assignment to a struct field or array element is treated as both a
// use and a definition of that variable, since only part of its value is
// assigned.  For analysis purposes, it's treated as though the entire value
// is read, then part of it is modified, then the entire value is assigned back
// to the variable.  (This is necessary for the analysis to produce correct
// results.)
//
// No nodes from the cfg.Defers list will be returned in the output of
// this function as they are disjoint from a cfg's blocks.
// For analyzing the statements in the cfg.Defers list, each defer
// should be treated as though it has the same in and out sets as the cfg.Exit node.
func DefUse(cfg *cfg.CFG, info *loader.PackageInfo) map[ast.Stmt]map[ast.Stmt]struct{} {
	blocks, gen, kill := genKillBitsets(cfg, info)
	ins, _ := reachingDefBitsets(cfg, gen, kill)
	return defUseResultSet(blocks, ins)
}

// DefsReaching builds reaching definitions for a given control flow graph,
// returning the set of statements that define a variable (i.e., declare or
// assign it) where that definition reaches the given statement.
func DefsReaching(stmt ast.Stmt, cfg *cfg.CFG, info *loader.PackageInfo) map[ast.Stmt]struct{} {
	blocks, gen, kill := genKillBitsets(cfg, info)
	ins, _ := reachingDefBitsets(cfg, gen, kill)
	return defsReachingResultSet(stmt, blocks, ins)
}

// genKillBitsets builds the gen and kill bitsets for each block in a cfg,
// these are used to compute reaching definitions.
func genKillBitsets(cfg *cfg.CFG, info *loader.PackageInfo) (blocks []ast.Stmt, gen, kill map[ast.Stmt]*bitset.BitSet) {
	okills := make(map[*types.Var]*bitset.BitSet)
	gen = make(map[ast.Stmt]*bitset.BitSet)
	kill = make(map[ast.Stmt]*bitset.BitSet)
	blocks = cfg.Blocks()

	for _, b := range blocks { // prime
		gen[b] = new(bitset.BitSet)
		kill[b] = new(bitset.BitSet)
	}

	// Iterate over all blocks twice, because a block may not know the entirety of what
	// it kills until all blocks have been iterated over.
	for i := 0; i < 2; i++ {
		for j, block := range blocks {
			j := uint(j)

			def := defs(block, info)

			for _, d := range def {
				if _, ok := okills[d]; !ok {
					okills[d] = new(bitset.BitSet)
				}
				gen[block].Set(j) // GEN this obj
				okills[d].Set(j)  // KILL this obj for everyone else
				// our kills are KILL[obj] - GEN[B]
				kill[block] = kill[block].Union(okills[d]).Difference(gen[block])
			}
		}
	}
	return blocks, gen, kill
}

// reachingDefBitsets will compute the reaching definitions in and out sets from gen and kill bitsets.
func reachingDefBitsets(cfg *cfg.CFG, gen, kill map[ast.Stmt]*bitset.BitSet) (in, out map[ast.Stmt]*bitset.BitSet) {
	in = make(map[ast.Stmt]*bitset.BitSet)
	out = make(map[ast.Stmt]*bitset.BitSet)
	blocks := cfg.Blocks()

	// OUT[ENTRY] = {};
	// for(each basic block B other than ENTRY) OUT[B} = {};
	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		in[block] = new(bitset.BitSet)
		out[block] = new(bitset.BitSet)
		if block == cfg.Entry {
			blocks = append(blocks[:i], blocks[i+1:]...)
			i--
		}
	}

	// for(changes to any OUT occur)
	for {
		var changed bool

		// for(each basic block B other than ENTRY) {
		for _, block := range blocks {

			// IN[B] = Union(P a pred of B) OUT[P];
			for _, p := range cfg.Preds(block) {
				in[block].InPlaceUnion(out[p])
			}

			old := out[block].Clone()

			// OUT[B] = gen[b] Union (IN[B] - kill[b]);
			out[block] = gen[block].Union(in[block].Difference(kill[block]))

			changed = changed || !old.Equal(out[block])
		}

		if !changed {
			break
		}
	}
	return in, out
}

// defUseResultSet maps reaching definitions in bitsets back to their corresponding statements, using
// this information to determine use-def and def-use information.
// blocks should be the blocks used to generate the analyses, as their indices are what will be used to map
// bits in each bitset back to the corresponding statement.
func defUseResultSet(blocks []ast.Stmt, ins map[ast.Stmt]*bitset.BitSet) map[ast.Stmt]map[ast.Stmt]struct{} {
	du := make(map[ast.Stmt]map[ast.Stmt]struct{})

	// map bits from in and out sets back to corresponding blocks (with cfg.Entry)
	for _, block := range blocks {
		du[block] = make(map[ast.Stmt]struct{})
	}

	for _, block := range blocks {
		for i, ok := uint(0), true; ok; i++ {
			if i, ok = ins[block].NextSet(i); ok {
				du[blocks[i]][block] = struct{}{}
			}
		}
	}
	return du
}

// defUseResultSet maps reaching definitions in bitsets back to their
// corresponding statements, returning the set of definition statements that
// reach the given stmt.
func defsReachingResultSet(stmt ast.Stmt, blocks []ast.Stmt, ins map[ast.Stmt]*bitset.BitSet) map[ast.Stmt]struct{} {
	result := make(map[ast.Stmt]struct{})
	insStmt, found := ins[stmt]
	if !found {
		panic("stmt not in CFG")
	}
	for i, ok := uint(0), true; ok; i++ {
		if i, ok = insStmt.NextSet(i); ok {
			result[blocks[i]] = struct{}{}
		}
	}
	return result
}

func PrintDot(f io.Writer, fset *token.FileSet, info *loader.PackageInfo, cfg *cfg.CFG) {
	du := DefUse(cfg, info)

	fmt.Fprintf(f, `digraph mgraph {
mode="heir";
splines="ortho";

`)

	blocks := cfg.Blocks()
	cfg.Sort(blocks)

	// Assign a number to each CFG node/statement
	// List all vertices before listing edges connecting them
	stmtNum := map[ast.Stmt]uint{}
	lastNum := uint(0)
	for _, stmt := range blocks {
		lastNum++
		stmtNum[stmt] = lastNum
		fmt.Fprintf(f, "\ts%d [label=\"%s\"];\n",
			lastNum, printStmt(stmt, cfg, fset))
	}

	for _, from := range blocks {
		// Show control flow edges in black
		succs := cfg.Succs(from)
		cfg.Sort(succs)
		for _, to := range succs {
			fmt.Fprintf(f, "\ts%d -> s%d\n", stmtNum[from], stmtNum[to])
		}

		// Show def-use edges in red, dotted
		usedIn := []ast.Stmt{}
		for stmt, _ := range du[from] {
			usedIn = append(usedIn, stmt)
		}
		cfg.Sort(usedIn)
		for _, stmt := range usedIn {
			varList := reachingVars(from, stmt, info)
			if len(varList) > 0 {
				fmt.Fprintf(f, "\ts%d -> s%d [xlabel=\"%s\",style=dotted,fontcolor=red,color=red]\n",
					stmtNum[from],
					stmtNum[stmt],
					strings.TrimSpace(varList))
			}
		}

	}

	fmt.Fprintf(f, "}\n")
}

func printStmt(stmt ast.Stmt, cfg *cfg.CFG, fset *token.FileSet) string {
	switch stmt {
	case cfg.Entry:
		return "ENTRY"
	case cfg.Exit:
		return "EXIT"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%s (line %d)",
			astutil.NodeDescription(stmt),
			fset.Position(stmt.Pos()).Line)
	}
}

func reachingVars(from, to ast.Stmt, info *loader.PackageInfo) string {
	asgt, updt, decl, _ := ReferencedVars([]ast.Stmt{from}, info)
	_, _, _, use := ReferencedVars([]ast.Stmt{to}, info)

	vars := map[string]struct{}{}
	for variable, _ := range asgt {
		if _, used := use[variable]; used {
			vars[variable.Name()] = struct{}{}
		}
	}
	for variable, _ := range updt {
		if _, used := use[variable]; used {
			vars[variable.Name()] = struct{}{}
		}
	}
	for variable, _ := range decl {
		if _, used := use[variable]; used {
			vars[variable.Name()] = struct{}{}
		}
	}

	varList := []string{}
	for name, _ := range vars {
		varList = append(varList, name)
	}
	sort.Sort(sort.StringSlice(varList))

	var b bytes.Buffer
	for _, name := range varList {
		b.WriteString(fmt.Sprintf(" %s", name))
	}
	return b.String()
}
