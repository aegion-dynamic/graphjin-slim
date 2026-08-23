package qcode

import (
	"encoding/json"
	"errors"
)

// Constraint is one named validation bound to an input variable.
// Frontends build constraints from directive arguments; ProcessConstraints
// evaluates them before execution.
type Constraint struct {
	VarName string
	fns     []constFn
}

type constFn struct {
	name string
	fn   ValidFn
}

// Vars carries request variables into validators.
type Vars map[string]json.RawMessage

// ValidFn reports whether the constraint holds for the given vars.
type ValidFn func(Vars, Constraint) bool

// NewValidFn compiles validator arguments into a check function.
type NewValidFn func(args []string) (fn ValidFn, err error)

var ErrUnknownValidator = errors.New("unknown validator")

// ValidErr is a failed constraint, reported without executing the query.
type ValidErr struct {
	FieldName  string `json:"field_name"`
	Constraint string `json:"constraint"`
}

// ProcessConstraints evaluates every constraint attached to the query.
func (qc *QCode) ProcessConstraints(vmap map[string]json.RawMessage) (errs []ValidErr) {
	for _, c := range qc.Consts {
		if err := validate(vmap, c); err != nil {
			errs = append(errs, err...)
		}
	}
	return
}

func validate(vmap map[string]json.RawMessage, c Constraint) (errs []ValidErr) {
	for _, fn := range c.fns {
		if ok := fn.fn(vmap, c); !ok {
			err := ValidErr{
				FieldName:  c.VarName,
				Constraint: fn.name,
			}
			errs = append(errs, err)
		}
	}
	return
}

// AddConstFn registers one named check on the constraint. Frontends use
// it while building constraints from their own directive arguments.
func (c *Constraint) AddConstFn(name string, fn ValidFn) {
	c.fns = append(c.fns, constFn{name: name, fn: fn})
}
