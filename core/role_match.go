package core

import "github.com/aegion-dynamic/graphjin-slim/core/v3/roles"

type compiledRoleMatch = roles.CompiledMatch

func compileRoleMatches(roleList []Role) ([]compiledRoleMatch, error) {
	in := make([]roles.MatchRole, 0, len(roleList))
	for _, role := range roleList {
		in = append(in, roles.MatchRole{Name: role.Name, Match: role.Match})
	}
	return roles.CompileMatches(in)
}
