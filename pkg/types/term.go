package types

import (
	"fmt"
	"strings"
)

// Term represents a Prolog term.
type Term interface {
	String() string
	Equal(Term) bool
}

// Atom represents a Prolog atom (constant symbol).
type Atom struct {
	Name string
}

func (a *Atom) String() string { return a.Name }
func (a *Atom) Equal(t Term) bool {
	if b, ok := t.(*Atom); ok {
		return a.Name == b.Name
	}
	return false
}

// Number represents an integer.
type Number struct {
	Value int64
}

func (n *Number) String() string { return fmt.Sprintf("%d", n.Value) }
func (n *Number) Equal(t Term) bool {
	if m, ok := t.(*Number); ok {
		return n.Value == m.Value
	}
	return false
}

// Float represents a floating-point number.
type Float struct {
	Value float64
}

func (f *Float) String() string { return fmt.Sprintf("%g", f.Value) }
func (f *Float) Equal(t Term) bool {
	if g, ok := t.(*Float); ok {
		return f.Value == g.Value
	}
	return false
}

// Variable represents a Prolog variable.
type Variable struct {
	Name string
}

func (v *Variable) String() string { return v.Name }
func (v *Variable) Equal(t Term) bool {
	if w, ok := t.(*Variable); ok {
		return v.Name == w.Name
	}
	return false
}

// Compound represents a compound term f(a1, a2, ..., an).
type Compound struct {
	Functor string
	Args    []Term
}

func (c *Compound) String() string {
	// Special handling for lists
	if c.Functor == "." && len(c.Args) == 2 {
		return formatList(c)
	}
	if len(c.Args) == 0 {
		return c.Functor
	}
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = a.String()
	}
	return fmt.Sprintf("%s(%s)", c.Functor, strings.Join(args, ", "))
}

func (c *Compound) Equal(t Term) bool {
	if d, ok := t.(*Compound); ok {
		if c.Functor != d.Functor || len(c.Args) != len(d.Args) {
			return false
		}
		for i := range c.Args {
			if !c.Args[i].Equal(d.Args[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func formatList(c *Compound) string {
	var elems []string
	current := Term(c)
	for {
		if comp, ok := current.(*Compound); ok && comp.Functor == "." && len(comp.Args) == 2 {
			elems = append(elems, comp.Args[0].String())
			current = comp.Args[1]
		} else if atom, ok := current.(*Atom); ok && atom.Name == "[]" {
			return "[" + strings.Join(elems, ", ") + "]"
		} else {
			return "[" + strings.Join(elems, ", ") + "|" + current.String() + "]"
		}
	}
}

// Clause represents a Prolog clause (fact or rule).
// Head :- Body1, Body2, ...
// A fact has an empty Body.
type Clause struct {
	Head *Compound
	Body []Term
}

func (c *Clause) String() string {
	if len(c.Body) == 0 {
		return c.Head.String() + "."
	}
	bodyStrs := make([]string, len(c.Body))
	for i, b := range c.Body {
		bodyStrs[i] = b.String()
	}
	return c.Head.String() + " :- " + strings.Join(bodyStrs, ", ") + "."
}

// MakeAtom creates a new Atom.
func MakeAtom(name string) *Atom {
	return &Atom{Name: name}
}

// MakeNumber creates a new Number.
func MakeNumber(val int64) *Number {
	return &Number{Value: val}
}

// MakeVar creates a new Variable.
func MakeVar(name string) *Variable {
	return &Variable{Name: name}
}

// MakeCompound creates a new Compound term.
func MakeCompound(functor string, args ...Term) *Compound {
	return &Compound{Functor: functor, Args: args}
}

// MakeList creates a Prolog list from a slice of terms.
func MakeList(elems ...Term) Term {
	result := Term(MakeAtom("[]"))
	for i := len(elems) - 1; i >= 0; i-- {
		result = MakeCompound(".", elems[i], result)
	}
	return result
}
