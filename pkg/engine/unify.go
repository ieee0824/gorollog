package engine

import (
	"github.com/ieee0824/gorollog/pkg/types"
)

// Binding maps variable names to terms.
type Binding map[string]types.Term

// NewBinding creates an empty binding.
func NewBinding() Binding {
	return make(Binding)
}

// Clone creates a copy of the binding.
func (b Binding) Clone() Binding {
	c := make(Binding, len(b))
	for k, v := range b {
		c[k] = v
	}
	return c
}

// Resolve walks through variable bindings to find the final value.
func (b Binding) Resolve(t types.Term) types.Term {
	switch v := t.(type) {
	case *types.Variable:
		if val, ok := b[v.Name]; ok {
			return b.Resolve(val)
		}
		return v
	case *types.Compound:
		args := make([]types.Term, len(v.Args))
		for i, a := range v.Args {
			args[i] = b.Resolve(a)
		}
		return types.MakeCompound(v.Functor, args...)
	default:
		return t
	}
}

// Unify attempts to unify two terms, returning true if successful.
func Unify(t1, t2 types.Term, b Binding) bool {
	t1 = b.Resolve(t1)
	t2 = b.Resolve(t2)

	switch v1 := t1.(type) {
	case *types.Variable:
		if !occursCheck(v1, t2, b) {
			b[v1.Name] = t2
			return true
		}
		return false

	case *types.Atom:
		switch v2 := t2.(type) {
		case *types.Variable:
			if !occursCheck(v2, t1, b) {
				b[v2.Name] = t1
				return true
			}
			return false
		case *types.Atom:
			return v1.Name == v2.Name
		default:
			return false
		}

	case *types.Number:
		switch v2 := t2.(type) {
		case *types.Variable:
			if !occursCheck(v2, t1, b) {
				b[v2.Name] = t1
				return true
			}
			return false
		case *types.Number:
			return v1.Value == v2.Value
		default:
			return false
		}

	case *types.Float:
		switch v2 := t2.(type) {
		case *types.Variable:
			if !occursCheck(v2, t1, b) {
				b[v2.Name] = t1
				return true
			}
			return false
		case *types.Float:
			return v1.Value == v2.Value
		default:
			return false
		}

	case *types.Compound:
		switch v2 := t2.(type) {
		case *types.Variable:
			if !occursCheck(v2, t1, b) {
				b[v2.Name] = t1
				return true
			}
			return false
		case *types.Compound:
			if v1.Functor != v2.Functor || len(v1.Args) != len(v2.Args) {
				return false
			}
			for i := range v1.Args {
				if !Unify(v1.Args[i], v2.Args[i], b) {
					return false
				}
			}
			return true
		default:
			return false
		}

	default:
		return false
	}
}

// occursCheck returns true if variable v occurs in term t.
func occursCheck(v *types.Variable, t types.Term, b Binding) bool {
	t = b.Resolve(t)
	switch u := t.(type) {
	case *types.Variable:
		return v.Name == u.Name
	case *types.Compound:
		for _, arg := range u.Args {
			if occursCheck(v, arg, b) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
