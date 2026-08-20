package core

import "github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"

// Source-mode access rules removed in slim; multi-DB SQL path is unchanged.
func (gj *graphjinEngine) applySourceAccessRules(dbinfo *sdata.DBInfo, database string) error {
	return nil
}
