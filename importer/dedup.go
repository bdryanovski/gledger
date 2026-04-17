package importer

import "doublebook/ast"

// ExtractIDs scans a slice of transactions and returns the set of all IDs
// found in their `; id: <hash>` comment fields.
// Transactions without an ID are silently ignored.
func ExtractIDs(transactions []*ast.Transaction) map[string]bool {
	ids := make(map[string]bool, len(transactions))
	for _, txn := range transactions {
		if txn.ID != "" {
			ids[txn.ID] = true
		}
	}
	return ids
}
