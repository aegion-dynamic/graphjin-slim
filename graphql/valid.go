package graphql

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
)

type Validator struct {
	Description string
	Type        string
	List        bool
	Types       []graph.ParserType
	NewFn       qcode.NewValidFn
}

func (co *Compiler) newConstraint(varName string, dargs []graph.Arg) (con qcode.Constraint, err error) {
	con = qcode.Constraint{VarName: varName}

	for _, a := range dargs {
		if a.Name == "variable" {
			continue
		}

		v, ok := co.c.Validators[a.Name]
		if !ok {
			err = qcode.ErrUnknownValidator
			return
		}

		if err = validateArg(a, v.Types...); err != nil {
			return
		}

		var args []string

		switch a.Val.Type {
		case graph.NodeStr:
			args = []string{quoteStr(a.Val.Val)}

		case graph.NodeNum, graph.NodeBool:
			args = []string{a.Val.Val}

		case graph.NodeObj:
			for _, v := range a.Val.Children {
				if v.Type == graph.NodeStr {
					// wrap so we don't have to unwrap at checktime
					args = append(args, v.Name, quoteStr(v.Val))
				} else {
					args = append(args, v.Name, v.Val)
				}
			}

		case graph.NodeList:
			for _, v := range a.Val.Children {
				if v.Type == graph.NodeStr {
					// wrap so we don't have to unwrap at checktime
					args = append(args, quoteStr(v.Val))
				} else {
					args = append(args, v.Val)
				}
			}
		}

		var fn qcode.ValidFn
		if fn, err = v.NewFn(args); err != nil {
			return
		}
		con.AddConstFn(a.Name, fn)
	}
	return
}

func quoteStr(v string) string {
	return `"` + v + `"`
}
