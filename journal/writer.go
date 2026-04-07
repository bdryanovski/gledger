package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	AST "doublebook/ast"
	"doublebook/utils"
)

// MaxJournalSize is the size threshold (in bytes) at which a new overflow
// file is started instead of appending to the current one.
const MaxJournalSize = 1 * 1024 * 1024 // 1 MB

// ---------------------------------------------------------------------------
// AppendTransaction
// ---------------------------------------------------------------------------

// AppendTransaction serialises txn and appends it to the appropriate journal
// file for the given name/dataDir pair.
//
// If the active file (the last existing file in the sequence) would exceed
// MaxJournalSize after the append, a new overflow file is created:
//
//	<name>.journal         → <name>.1.journal
//	<name>.1.journal       → <name>.2.journal
//	…
//
// The data directory is created automatically when it does not exist.
func AppendTransaction(name string, dataDir string, txn *AST.Transaction) error {
	dataDir = utils.ExpandHome(dataDir)

	// Ensure the data directory exists.
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory %q: %w", dataDir, err)
	}

	// Determine the base prefix for file naming.
	prefix := journalPrefix(name, dataDir)

	// Collect existing files to find the active one.
	existing := Resolve(name, dataDir)

	// Serialise the transaction (always ends with a trailing newline).
	content := txn.String() + "\n"

	// Decide which file to write to.
	targetPath := chooseTargetFile(prefix, existing, int64(len(content)))

	// Open the target file in append mode (create if it doesn't exist).
	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("opening journal file %q: %w", targetPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("writing to journal file %q: %w", targetPath, err)
	}
	return nil
}

// chooseTargetFile returns the path to write the next transaction to.
// If no files exist yet, it returns <prefix>.journal.
// If the active file has room, it returns that path.
// Otherwise it returns the next overflow path.
func chooseTargetFile(prefix string, existing []string, addBytes int64) string {
	if len(existing) == 0 {
		// No files yet — start with the base file.
		return prefix + ".journal"
	}

	active := existing[len(existing)-1]

	// Check whether the active file would exceed the limit.
	info, err := os.Stat(active)
	if err == nil && info.Size()+addBytes > MaxJournalSize {
		// Need a new overflow file.
		return nextOverflowPath(prefix, existing)
	}

	return active
}

// nextOverflowPath returns the path for the next file in the overflow
// sequence, e.g. given existing = [..., "data.2.journal"] → "data.3.journal".
func nextOverflowPath(prefix string, existing []string) string {
	// The highest index is implied by the count of existing files:
	//   existing[0] = base       (index 0, no number in name)
	//   existing[1] = .1.journal (index 1)
	//   existing[2] = .2.journal (index 2)
	// So the next index = len(existing).
	nextIdx := len(existing)
	return fmt.Sprintf("%s.%d.journal", prefix, nextIdx)
}

// ---------------------------------------------------------------------------
// WriteAll
// ---------------------------------------------------------------------------

// WriteAll serialises all transactions and overwrites the file at path
// atomically (write to a temp file then rename).
//
// This is intended for in-place rewrites (e.g. after re-sorting).  It does
// NOT perform size splitting — for appending use AppendTransaction instead.
func WriteAll(path string, transactions []*AST.Transaction) error {
	path = utils.ExpandHome(path)

	// Build the full content.
	var b strings.Builder
	for i, txn := range transactions {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(txn.String())
	}

	// Write atomically via a sibling temp file.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %q: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("renaming temp file to %q: %w", path, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// journalPrefix returns the filesystem path prefix used for constructing
// journal file names (without the ".journal" extension).
func journalPrefix(name string, dataDir string) string {
	name = utils.ExpandHome(name)
	if strings.ContainsAny(name, "/\\") || strings.HasSuffix(name, ".journal") {
		// Absolute/relative path — strip the extension if present.
		return strings.TrimSuffix(name, ".journal")
	}
	return filepath.Join(dataDir, name)
}
