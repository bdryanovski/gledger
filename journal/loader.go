// Package journal handles multi-file journal loading and size-based file
// splitting.  Journal files follow the naming convention:
//
//	<name>.journal          (base file)
//	<name>.1.journal        (overflow file 1)
//	<name>.2.journal        (overflow file 2)
//	…
//
// All parts are loaded transparently and merged into a single sorted slice.
package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"doublebook/ast"
	"doublebook/parser"
	"doublebook/utils"
)

// ---------------------------------------------------------------------------
// Resolve
// ---------------------------------------------------------------------------

// Resolve returns the ordered list of existing journal file paths for the
// given name.
//
// Rules:
//  1. If name ends in ".journal" it is treated as an exact path.
//  2. If name contains a path separator it is treated as a path prefix:
//     <name>.journal, <name>.1.journal, <name>.2.journal, …
//  3. Otherwise the files are looked up inside dataDir:
//     <dataDir>/<name>.journal, <dataDir>/<name>.1.journal, …
//
// Numbered files are collected in ascending order.  The first missing index
// stops the scan (files beyond the gap are silently ignored).
func Resolve(name string, dataDir string) []string {
	name = utils.ExpandHome(name)
	dataDir = utils.ExpandHome(dataDir)

	// Exact path — the caller already knows the file.
	if strings.HasSuffix(name, ".journal") {
		return existingPaths([]string{name})
	}

	// Determine the base prefix (directory + stem).
	var prefix string
	if strings.ContainsAny(name, "/\\") {
		// name is a path prefix like "/home/user/personal"
		prefix = name
	} else {
		// name is a bare stem like "personal" — resolve inside dataDir
		prefix = filepath.Join(dataDir, name)
	}

	// Collect: <prefix>.journal  then  <prefix>.1.journal, <prefix>.2.journal, …
	var paths []string
	base := prefix + ".journal"
	if fileExists(base) {
		paths = append(paths, base)
	}
	for i := 1; ; i++ {
		p := fmt.Sprintf("%s.%d.journal", prefix, i)
		if !fileExists(p) {
			break
		}
		paths = append(paths, p)
	}
	return paths
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

// Load reads all journal files for the given name, merges their transactions,
// sorts them ascending by date, and returns the result.
//
// If no journal files are found an empty (non-nil) slice is returned — this
// is not an error; the user may not have any transactions yet.
func Load(name string, dataDir string) ([]*ast.Transaction, error) {
	paths := Resolve(name, dataDir)
	var all []*ast.Transaction
	for _, p := range paths {
		txns, err := LoadFromPath(p)
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", p, err)
		}
		all = append(all, txns...)
	}
	sortByDate(all)
	return all, nil
}

// LoadFromPath parses a single journal file at an exact filesystem path.
func LoadFromPath(path string) ([]*ast.Transaction, error) {
	path = utils.ExpandHome(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	txns, err := parser.ParseTransactions(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse error in %q: %w", path, err)
	}
	return txns, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func existingPaths(candidates []string) []string {
	var out []string
	for _, p := range candidates {
		if fileExists(p) {
			out = append(out, p)
		}
	}
	return out
}

// sortByDate sorts transactions ascending by their date field.
// YYYY-MM-DD string comparison produces the same order as chronological.
func sortByDate(txns []*ast.Transaction) {
	sort.Slice(txns, func(i, j int) bool {
		return txns[i].Date.Before(txns[j].Date)
	})
}
