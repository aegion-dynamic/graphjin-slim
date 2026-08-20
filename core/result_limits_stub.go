package core

import "github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"

// RootLimitInfo describes paging posture of a result root (slim stub).
type RootLimitInfo struct {
	FieldName string
	Table     string
	Database  string
	Path      []string
	Limit     int32
	NoLimit   bool
	Aggregate bool
	Singular  bool
}

func rootLimitInfoFromQCode(qc *qcode.QCode) []RootLimitInfo { return nil }

func (r *Result) RootLimits() []RootLimitInfo { return r.rootLimits }
