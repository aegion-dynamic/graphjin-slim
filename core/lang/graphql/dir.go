package graphql

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

func (co *Compiler) compileOpDirectives(qc *QCode, dirs []graph.Directive) error {
	var err error

	for _, d := range dirs {
		switch d.Name {
		case "cacheControl":
			err = co.compileDirectiveCacheControl(qc, d)

		case "constraint", "validate":
			err = co.compileDirectiveConstraint(qc, d)

		default:
			err = fmt.Errorf("unknown operation directive: %s", d.Name)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

// directives need to run before the relationship resolution code
func (co *Compiler) compileSelectorDirectives(qc *QCode,
	sel *Select, dirs []graph.Directive,
) (err error) {
	for _, d := range dirs {
		switch d.Name {
		case "include":
			err = co.compileDirectiveSkipInclude(false, sel, &sel.Field, d)

		case "skip":
			err = co.compileDirectiveSkipInclude(true, sel, &sel.Field, d)

		case "schema":
			err = co.compileDirectiveSchema(sel, d)

		case "database":
			err = co.compileDirectiveDatabase(sel, d)

		case "notRelated", "not_related":
			err = co.compileDirectiveNotRelated(sel, d)

		case "through":
			err = co.compileDirectiveThrough(sel, d)

		case "object":
			sel.Singular = true
			sel.Paging.Limit = 1

		default:
			err = fmt.Errorf("no such selector directive: %s", d.Name)
		}

		if err != nil {
			return fmt.Errorf("directive @%s: %w", d.Name, err)
		}
	}
	return
}

func (co *Compiler) compileFieldDirectives(sel *Select,
	f *Field, dirs []graph.Directive,
) (err error) {
	for _, d := range dirs {
		switch d.Name {
		case "include":
			err = co.compileDirectiveSkipInclude(false, sel, f, d)

		case "skip":
			err = co.compileDirectiveSkipInclude(true, sel, f, d)

		case "window":
			err = co.compileDirectiveWindow(sel, f, d)

		case "running", "moving", "previous", "next", "first", "last", "rank", "denseRank", "rowNumber":
			err = co.compileDirectiveAnalytics(sel, f, d)

		default:
			err = fmt.Errorf("unknown field directive: %s", d.Name)
		}
		if err != nil {
			return fmt.Errorf("directive @%s: %w", d.Name, err)
		}
	}
	return
}

func (co *Compiler) compileDirectiveSchema(sel *Select, d graph.Directive) (err error) {
	arg, err := getArg(d.Args, "name", graph.NodeStr)
	if err != nil {
		return
	}
	sel.Schema = arg.Val.Val
	return
}

func (co *Compiler) compileDirectiveDatabase(sel *Select, d graph.Directive) (err error) {
	arg, err := getArg(d.Args, "name", graph.NodeStr)
	if err != nil {
		return
	}
	sel.Database = arg.Val.Val
	return
}

func (co *Compiler) compileDirectiveSkipInclude(
	skip bool,
	sel *Select,
	f *Field,
	d graph.Directive,
) (err error) {
	if len(d.Args) == 0 {
		err = fmt.Errorf("argument 'if' or 'if_var' expected")
		return
	}

	for _, arg := range d.Args {
		switch arg.Name {
		case "if", "ifVar", "if_var":
			if err = validateArg(arg, graph.NodeVar); err != nil {
				return
			}
			var ex *Exp
			if skip {
				ex = newExpOp(OpNotEqualsTrue)
			} else {
				ex = newExpOp(OpEqualsTrue)
			}
			ex.Right.ValType = ValVar
			ex.Right.Val = arg.Val.Val
			addAndFilter(&f.FieldFilter, ex)

			if f.Type == FieldTypeTable {
				addAndFilter(&sel.Where, ex)
			}

		default:
			return unknownArg(arg)
		}
	}
	return
}

func (co *Compiler) compileDirectiveCacheControl(qc *QCode, d graph.Directive) (err error) {
	var hdr []string

	if len(d.Args) == 0 {
		err = fmt.Errorf("arguments 'maxAge' or 'maxAge' expected")
		return
	}

	for _, arg := range d.Args {
		switch arg.Name {
		case "maxAge":
			if err = validateArg(arg, graph.NodeNum); err != nil {
				return
			}
			hdr = append(hdr, "max-age="+arg.Val.Val)
		case "scope":
			if err = validateArg(arg, graph.NodeStr); err != nil {
				return
			}
			hdr = append(hdr, arg.Val.Val)

		default:
			return unknownArg(arg)

		}
	}
	if len(hdr) != 0 {
		qc.Cache.Header = strings.Join(hdr, " ")
	}
	return nil
}

func (co *Compiler) compileDirectiveConstraint(qc *QCode, d graph.Directive) (err error) {
	a, err := getArg(d.Args, "variable", graph.NodeStr)
	if err != nil {
		return
	}
	varName := a.Val.Val

	con, err := co.newConstraint(varName, d.Args)
	if err == ErrUnknownValidator {
		return unknownArg(a)
	}
	if err != nil {
		return
	}

	qc.Consts = append(qc.Consts, con)
	return
}

func (co *Compiler) compileDirectiveNotRelated(sel *Select, d graph.Directive) error {
	sel.Rel.Type = sdata.RelSkip
	return nil
}

func (co *Compiler) compileDirectiveThrough(sel *Select, d graph.Directive) (err error) {
	if len(d.Args) == 0 {
		return fmt.Errorf("required argument 'table' or 'column'")
	}

	for _, a := range d.Args {
		switch a.Name {
		case "table":
			if err = validateArg(a, graph.NodeStr, graph.NodeLabel); err != nil {
				return
			}
			sel.through = a.Val.Val
			sel.throughKind = "table"
			return

		case "column":
			if err = validateArg(a, graph.NodeStr); err != nil {
				return
			}
			sel.through = a.Val.Val
			sel.throughKind = "column"
			return

		default:
			return unknownArg(a)
		}
	}
	return
}
