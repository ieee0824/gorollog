package engine

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/ieee0824/gorollog/pkg/types"
)

// Engine is the Prolog inference engine.
type Engine struct {
	Database  []*types.Clause
	varCount  int
	Output    io.Writer
	CutSignal bool
}

// New creates a new Engine.
func New() *Engine {
	return &Engine{
		Output: os.Stdout,
	}
}

// AddClause adds a clause to the database.
func (e *Engine) AddClause(c *types.Clause) {
	e.Database = append(e.Database, c)
}

// freshVar generates a fresh variable name.
func (e *Engine) freshVar() string {
	e.varCount++
	return fmt.Sprintf("_G%d", e.varCount)
}

// RenameVars creates a copy of a clause with all variables renamed to fresh names.
func (e *Engine) RenameVars(c *types.Clause) *types.Clause {
	mapping := make(map[string]string)
	head := e.renameTerm(c.Head, mapping).(*types.Compound)
	body := make([]types.Term, len(c.Body))
	for i, b := range c.Body {
		body[i] = e.renameTerm(b, mapping)
	}
	return &types.Clause{Head: head, Body: body}
}

func (e *Engine) renameTerm(t types.Term, mapping map[string]string) types.Term {
	switch v := t.(type) {
	case *types.Variable:
		if newName, ok := mapping[v.Name]; ok {
			return types.MakeVar(newName)
		}
		newName := e.freshVar()
		mapping[v.Name] = newName
		return types.MakeVar(newName)
	case *types.Compound:
		args := make([]types.Term, len(v.Args))
		for i, a := range v.Args {
			args[i] = e.renameTerm(a, mapping)
		}
		return types.MakeCompound(v.Functor, args...)
	default:
		return t
	}
}

// Solution represents a single solution to a query.
type Solution struct {
	Bindings Binding
}

// Solve finds all solutions for a list of goals.
func (e *Engine) Solve(goals []types.Term, b Binding, callback func(Binding) bool) {
	e.CutSignal = false
	e.solve(goals, b, callback)
}

func (e *Engine) solve(goals []types.Term, b Binding, callback func(Binding) bool) bool {
	if len(goals) == 0 {
		return callback(b)
	}

	goal := b.Resolve(goals[0])
	rest := goals[1:]

	// Handle built-in predicates
	if handled, stop := e.tryBuiltin(goal, rest, b, callback); handled {
		return stop
	}

	// Convert atom to compound for matching
	goalComp, ok := goalToCompound(goal)
	if !ok {
		fmt.Fprintf(e.Output, "Error: goal is not callable: %s\n", goal.String())
		return false
	}

	// Search database
	for _, clause := range e.Database {
		if e.CutSignal {
			return true
		}
		renamed := e.RenameVars(clause)
		newB := b.Clone()
		if Unify(goalComp, renamed.Head, newB) {
			newGoals := append(renamed.Body, rest...)
			if e.solve(newGoals, newB, callback) {
				return true
			}
		}
	}
	return false
}

func goalToCompound(t types.Term) (*types.Compound, bool) {
	switch v := t.(type) {
	case *types.Compound:
		return v, true
	case *types.Atom:
		return types.MakeCompound(v.Name), true
	default:
		return nil, false
	}
}

func (e *Engine) tryBuiltin(goal types.Term, rest []types.Term, b Binding, callback func(Binding) bool) (handled bool, stop bool) {
	switch g := goal.(type) {
	case *types.Atom:
		switch g.Name {
		case "true":
			return true, e.solve(rest, b, callback)
		case "fail", "false":
			return true, false
		case "!":
			stop := e.solve(rest, b, callback)
			e.CutSignal = true
			return true, stop
		case "nl":
			fmt.Fprintln(e.Output)
			return true, e.solve(rest, b, callback)
		case "halt":
			os.Exit(0)
			return true, true
		case "listing":
			for _, c := range e.Database {
				fmt.Fprintln(e.Output, c.String())
			}
			return true, e.solve(rest, b, callback)
		}

	case *types.Compound:
		switch g.Functor {
		case ",":
			if len(g.Args) == 2 {
				newGoals := append([]types.Term{g.Args[0], g.Args[1]}, rest...)
				return true, e.solve(newGoals, b, callback)
			}

		case ";":
			if len(g.Args) == 2 {
				// Check for if-then-else: (Cond -> Then ; Else)
				if cond, ok := g.Args[0].(*types.Compound); ok && cond.Functor == "->" && len(cond.Args) == 2 {
					found := false
					newGoals := append([]types.Term{cond.Args[1]}, rest...)
					e.solve([]types.Term{cond.Args[0]}, b, func(b2 Binding) bool {
						found = true
						e.solve(newGoals, b2, callback)
						return true // Only first solution of condition
					})
					if !found {
						newGoals := append([]types.Term{g.Args[1]}, rest...)
						return true, e.solve(newGoals, b, callback)
					}
					return true, false
				}
				// Regular disjunction
				newGoals1 := append([]types.Term{g.Args[0]}, rest...)
				if e.solve(newGoals1, b.Clone(), callback) {
					return true, true
				}
				newGoals2 := append([]types.Term{g.Args[1]}, rest...)
				return true, e.solve(newGoals2, b.Clone(), callback)
			}

		case "->":
			if len(g.Args) == 2 {
				newGoals := append([]types.Term{g.Args[1]}, rest...)
				e.solve([]types.Term{g.Args[0]}, b, func(b2 Binding) bool {
					e.solve(newGoals, b2, callback)
					return true // Only first solution
				})
				return true, false
			}

		case "\\+":
			if len(g.Args) == 1 {
				found := false
				inner := b.Resolve(g.Args[0])
				e.solve([]types.Term{inner}, b.Clone(), func(_ Binding) bool {
					found = true
					return true
				})
				if !found {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "=":
			if len(g.Args) == 2 {
				newB := b.Clone()
				if Unify(g.Args[0], g.Args[1], newB) {
					return true, e.solve(rest, newB, callback)
				}
				return true, false
			}

		case "\\=":
			if len(g.Args) == 2 {
				testB := b.Clone()
				if !Unify(g.Args[0], g.Args[1], testB) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "==":
			if len(g.Args) == 2 {
				t1 := b.Resolve(g.Args[0])
				t2 := b.Resolve(g.Args[1])
				if t1.Equal(t2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "\\==":
			if len(g.Args) == 2 {
				t1 := b.Resolve(g.Args[0])
				t2 := b.Resolve(g.Args[1])
				if !t1.Equal(t2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "is":
			if len(g.Args) == 2 {
				val, err := e.evalArith(g.Args[1], b)
				if err != nil {
					fmt.Fprintf(e.Output, "Error: %v\n", err)
					return true, false
				}
				newB := b.Clone()
				if Unify(g.Args[0], val, newB) {
					return true, e.solve(rest, newB, callback)
				}
				return true, false
			}

		case "=:=":
			if len(g.Args) == 2 {
				v1, err1 := e.evalArith(g.Args[0], b)
				v2, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				if toFloat(v1) == toFloat(v2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "=\\=":
			if len(g.Args) == 2 {
				v1, err1 := e.evalArith(g.Args[0], b)
				v2, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				if toFloat(v1) != toFloat(v2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "<":
			if len(g.Args) == 2 {
				v1, err1 := e.evalArith(g.Args[0], b)
				v2, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				if toFloat(v1) < toFloat(v2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case ">":
			if len(g.Args) == 2 {
				v1, err1 := e.evalArith(g.Args[0], b)
				v2, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				if toFloat(v1) > toFloat(v2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "=<":
			if len(g.Args) == 2 {
				v1, err1 := e.evalArith(g.Args[0], b)
				v2, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				if toFloat(v1) <= toFloat(v2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case ">=":
			if len(g.Args) == 2 {
				v1, err1 := e.evalArith(g.Args[0], b)
				v2, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				if toFloat(v1) >= toFloat(v2) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "write":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				fmt.Fprint(e.Output, termToString(resolved))
				return true, e.solve(rest, b, callback)
			}

		case "writeln":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				fmt.Fprintln(e.Output, termToString(resolved))
				return true, e.solve(rest, b, callback)
			}

		case "write_canonical":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				fmt.Fprint(e.Output, resolved.String())
				return true, e.solve(rest, b, callback)
			}

		case "atom":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if _, ok := resolved.(*types.Atom); ok {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "number":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				switch resolved.(type) {
				case *types.Number, *types.Float:
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "integer":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if _, ok := resolved.(*types.Number); ok {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "float":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if _, ok := resolved.(*types.Float); ok {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "var":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if _, ok := resolved.(*types.Variable); ok {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "nonvar":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if _, ok := resolved.(*types.Variable); !ok {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "compound":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if c, ok := resolved.(*types.Compound); ok && len(c.Args) > 0 {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "is_list":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if isList(resolved) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "assert", "assertz":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				clause := termToClause(resolved)
				if clause != nil {
					e.Database = append(e.Database, clause)
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "asserta":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				clause := termToClause(resolved)
				if clause != nil {
					e.Database = append([]*types.Clause{clause}, e.Database...)
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "retract":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				target := termToClause(resolved)
				if target == nil {
					return true, false
				}
				for i, c := range e.Database {
					newB := b.Clone()
					renamed := e.RenameVars(c)
					if Unify(target.Head, renamed.Head, newB) && matchBody(target.Body, renamed.Body, newB) {
						e.Database = append(e.Database[:i], e.Database[i+1:]...)
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "functor":
			if len(g.Args) == 3 {
				resolved := b.Resolve(g.Args[0])
				newB := b.Clone()
				switch v := resolved.(type) {
				case *types.Atom:
					if Unify(g.Args[1], v, newB) && Unify(g.Args[2], types.MakeNumber(0), newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				case *types.Number:
					if Unify(g.Args[1], v, newB) && Unify(g.Args[2], types.MakeNumber(0), newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				case *types.Compound:
					if Unify(g.Args[1], types.MakeAtom(v.Functor), newB) && Unify(g.Args[2], types.MakeNumber(int64(len(v.Args))), newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				case *types.Variable:
					// functor(X, f, N) — construct term
					fAtom := b.Resolve(g.Args[1])
					nNum := b.Resolve(g.Args[2])
					if a, ok := fAtom.(*types.Atom); ok {
						if n, ok := nNum.(*types.Number); ok {
							if n.Value == 0 {
								if Unify(g.Args[0], a, newB) {
									return true, e.solve(rest, newB, callback)
								}
							} else {
								args := make([]types.Term, n.Value)
								for i := range args {
									args[i] = types.MakeVar(e.freshVar())
								}
								comp := types.MakeCompound(a.Name, args...)
								if Unify(g.Args[0], comp, newB) {
									return true, e.solve(rest, newB, callback)
								}
							}
						}
					}
					return true, false
				}
			}

		case "arg":
			if len(g.Args) == 3 {
				n := b.Resolve(g.Args[0])
				term := b.Resolve(g.Args[1])
				if num, ok := n.(*types.Number); ok {
					if comp, ok := term.(*types.Compound); ok {
						idx := int(num.Value)
						if idx >= 1 && idx <= len(comp.Args) {
							newB := b.Clone()
							if Unify(g.Args[2], comp.Args[idx-1], newB) {
								return true, e.solve(rest, newB, callback)
							}
						}
					}
				}
				return true, false
			}

		case "copy_term":
			if len(g.Args) == 2 {
				original := b.Resolve(g.Args[0])
				mapping := make(map[string]string)
				copied := e.copyTerm(original, mapping)
				newB := b.Clone()
				if Unify(g.Args[1], copied, newB) {
					return true, e.solve(rest, newB, callback)
				}
				return true, false
			}

		case "=..":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				newB := b.Clone()
				switch v := resolved.(type) {
				case *types.Compound:
					elems := []types.Term{types.MakeAtom(v.Functor)}
					elems = append(elems, v.Args...)
					list := types.MakeList(elems...)
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				case *types.Atom:
					list := types.MakeList(v)
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				case *types.Number:
					list := types.MakeList(v)
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				case *types.Variable:
					// Construct from list
					listTerm := b.Resolve(g.Args[1])
					elems := listToSlice(listTerm)
					if elems == nil {
						return true, false
					}
					if len(elems) == 0 {
						return true, false
					}
					functor, ok := elems[0].(*types.Atom)
					if !ok {
						return true, false
					}
					if len(elems) == 1 {
						if Unify(g.Args[0], functor, newB) {
							return true, e.solve(rest, newB, callback)
						}
					} else {
						comp := types.MakeCompound(functor.Name, elems[1:]...)
						if Unify(g.Args[0], comp, newB) {
							return true, e.solve(rest, newB, callback)
						}
					}
					return true, false
				}
			}

		case "length":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				elems := listToSlice(resolved)
				if elems != nil {
					newB := b.Clone()
					if Unify(g.Args[1], types.MakeNumber(int64(len(elems))), newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "append":
			if len(g.Args) == 3 {
				// append([], L, L).
				// append([H|T], L, [H|R]) :- append(T, L, R).
				l1 := b.Resolve(g.Args[0])
				l2 := b.Resolve(g.Args[1])
				l3 := b.Resolve(g.Args[2])
				return true, e.solveAppend(l1, l2, l3, rest, b, callback)
			}

		case "member":
			if len(g.Args) == 2 {
				elem := g.Args[0]
				list := b.Resolve(g.Args[1])
				return true, e.solveMember(elem, list, rest, b, callback)
			}

		case "between":
			if len(g.Args) == 3 {
				low, err1 := e.evalArith(g.Args[0], b)
				high, err2 := e.evalArith(g.Args[1], b)
				if err1 != nil || err2 != nil {
					return true, false
				}
				lowN, ok1 := low.(*types.Number)
				highN, ok2 := high.(*types.Number)
				if !ok1 || !ok2 {
					return true, false
				}
				for i := lowN.Value; i <= highN.Value; i++ {
					newB := b.Clone()
					if Unify(g.Args[2], types.MakeNumber(i), newB) {
						if e.solve(rest, newB, callback) {
							return true, true
						}
					}
				}
				return true, false
			}

		case "succ":
			if len(g.Args) == 2 {
				x := b.Resolve(g.Args[0])
				y := b.Resolve(g.Args[1])
				newB := b.Clone()
				if n, ok := x.(*types.Number); ok {
					if n.Value >= 0 {
						if Unify(g.Args[1], types.MakeNumber(n.Value+1), newB) {
							return true, e.solve(rest, newB, callback)
						}
					}
					return true, false
				}
				if n, ok := y.(*types.Number); ok {
					if n.Value > 0 {
						if Unify(g.Args[0], types.MakeNumber(n.Value-1), newB) {
							return true, e.solve(rest, newB, callback)
						}
					}
					return true, false
				}
				return true, false
			}

		case "plus":
			if len(g.Args) == 3 {
				x := b.Resolve(g.Args[0])
				y := b.Resolve(g.Args[1])
				z := b.Resolve(g.Args[2])
				newB := b.Clone()
				if xn, ok := x.(*types.Number); ok {
					if yn, ok := y.(*types.Number); ok {
						if Unify(g.Args[2], types.MakeNumber(xn.Value+yn.Value), newB) {
							return true, e.solve(rest, newB, callback)
						}
						return true, false
					}
					if zn, ok := z.(*types.Number); ok {
						if Unify(g.Args[1], types.MakeNumber(zn.Value-xn.Value), newB) {
							return true, e.solve(rest, newB, callback)
						}
						return true, false
					}
				}
				if yn, ok := y.(*types.Number); ok {
					if zn, ok := z.(*types.Number); ok {
						if Unify(g.Args[0], types.MakeNumber(zn.Value-yn.Value), newB) {
							return true, e.solve(rest, newB, callback)
						}
						return true, false
					}
				}
				return true, false
			}

		case "atom_chars":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				newB := b.Clone()
				if a, ok := resolved.(*types.Atom); ok {
					chars := []rune(a.Name)
					terms := make([]types.Term, len(chars))
					for i, ch := range chars {
						terms[i] = types.MakeAtom(string(ch))
					}
					list := types.MakeList(terms...)
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				}
				// Reverse: chars -> atom
				listTerm := b.Resolve(g.Args[1])
				elems := listToSlice(listTerm)
				if elems != nil {
					var sb strings.Builder
					for _, elem := range elems {
						if a, ok := elem.(*types.Atom); ok {
							sb.WriteString(a.Name)
						} else {
							return true, false
						}
					}
					if Unify(g.Args[0], types.MakeAtom(sb.String()), newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "atom_length":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				if a, ok := resolved.(*types.Atom); ok {
					newB := b.Clone()
					if Unify(g.Args[1], types.MakeNumber(int64(len([]rune(a.Name)))), newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "atom_concat":
			if len(g.Args) == 3 {
				a1 := b.Resolve(g.Args[0])
				a2 := b.Resolve(g.Args[1])
				if at1, ok := a1.(*types.Atom); ok {
					if at2, ok := a2.(*types.Atom); ok {
						newB := b.Clone()
						if Unify(g.Args[2], types.MakeAtom(at1.Name+at2.Name), newB) {
							return true, e.solve(rest, newB, callback)
						}
					}
				}
				return true, false
			}

		case "number_chars":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				if n, ok := resolved.(*types.Number); ok {
					s := fmt.Sprintf("%d", n.Value)
					chars := []rune(s)
					terms := make([]types.Term, len(chars))
					for i, ch := range chars {
						terms[i] = types.MakeAtom(string(ch))
					}
					list := types.MakeList(terms...)
					newB := b.Clone()
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "char_code":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				newB := b.Clone()
				if a, ok := resolved.(*types.Atom); ok && len([]rune(a.Name)) == 1 {
					code := int64([]rune(a.Name)[0])
					if Unify(g.Args[1], types.MakeNumber(code), newB) {
						return true, e.solve(rest, newB, callback)
					}
					return true, false
				}
				code := b.Resolve(g.Args[1])
				if n, ok := code.(*types.Number); ok {
					ch := string(rune(n.Value))
					if Unify(g.Args[0], types.MakeAtom(ch), newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "findall":
			if len(g.Args) == 3 {
				template := g.Args[0]
				goalArg := g.Args[1]
				resultList := g.Args[2]
				var results []types.Term
				e.solve([]types.Term{goalArg}, b.Clone(), func(solnB Binding) bool {
					resolved := solnB.Resolve(template)
					results = append(results, resolved)
					return false // collect all
				})
				list := types.MakeList(results...)
				newB := b.Clone()
				if Unify(resultList, list, newB) {
					return true, e.solve(rest, newB, callback)
				}
				return true, false
			}

		case "msort":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				elems := listToSlice(resolved)
				if elems != nil {
					sorted := msort(elems)
					list := types.MakeList(sorted...)
					newB := b.Clone()
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "sort":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				elems := listToSlice(resolved)
				if elems != nil {
					sorted := msort(elems)
					// Remove duplicates
					unique := removeDuplicates(sorted)
					list := types.MakeList(unique...)
					newB := b.Clone()
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "last":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				elems := listToSlice(resolved)
				if elems != nil && len(elems) > 0 {
					newB := b.Clone()
					if Unify(g.Args[1], elems[len(elems)-1], newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "reverse":
			if len(g.Args) == 2 {
				resolved := b.Resolve(g.Args[0])
				elems := listToSlice(resolved)
				if elems != nil {
					reversed := make([]types.Term, len(elems))
					for i, e := range elems {
						reversed[len(elems)-1-i] = e
					}
					list := types.MakeList(reversed...)
					newB := b.Clone()
					if Unify(g.Args[1], list, newB) {
						return true, e.solve(rest, newB, callback)
					}
				}
				return true, false
			}

		case "nth0":
			if len(g.Args) == 3 {
				resolved := b.Resolve(g.Args[1])
				elems := listToSlice(resolved)
				if elems == nil {
					return true, false
				}
				idxTerm := b.Resolve(g.Args[0])
				if n, ok := idxTerm.(*types.Number); ok {
					idx := int(n.Value)
					if idx >= 0 && idx < len(elems) {
						newB := b.Clone()
						if Unify(g.Args[2], elems[idx], newB) {
							return true, e.solve(rest, newB, callback)
						}
					}
					return true, false
				}
				// Enumerate
				for i, elem := range elems {
					newB := b.Clone()
					if Unify(g.Args[0], types.MakeNumber(int64(i)), newB) &&
						Unify(g.Args[2], elem, newB) {
						if e.solve(rest, newB, callback) {
							return true, true
						}
					}
				}
				return true, false
			}

		case "nth1":
			if len(g.Args) == 3 {
				resolved := b.Resolve(g.Args[1])
				elems := listToSlice(resolved)
				if elems == nil {
					return true, false
				}
				idxTerm := b.Resolve(g.Args[0])
				if n, ok := idxTerm.(*types.Number); ok {
					idx := int(n.Value) - 1
					if idx >= 0 && idx < len(elems) {
						newB := b.Clone()
						if Unify(g.Args[2], elems[idx], newB) {
							return true, e.solve(rest, newB, callback)
						}
					}
					return true, false
				}
				for i, elem := range elems {
					newB := b.Clone()
					if Unify(g.Args[0], types.MakeNumber(int64(i+1)), newB) &&
						Unify(g.Args[2], elem, newB) {
						if e.solve(rest, newB, callback) {
							return true, true
						}
					}
				}
				return true, false
			}

		case "maplist":
			if len(g.Args) >= 2 {
				return true, e.solveMaplist(g, rest, b, callback)
			}

		case "format":
			if len(g.Args) >= 1 {
				fmtStr := b.Resolve(g.Args[0])
				if a, ok := fmtStr.(*types.Atom); ok {
					var fmtArgs []types.Term
					if len(g.Args) >= 2 {
						argList := b.Resolve(g.Args[1])
						fmtArgs = listToSlice(argList)
					}
					formatted := formatString(a.Name, fmtArgs)
					fmt.Fprint(e.Output, formatted)
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "ground":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if isGround(resolved) {
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "call":
			if len(g.Args) >= 1 {
				inner := b.Resolve(g.Args[0])
				if len(g.Args) > 1 {
					// call(Goal, Arg1, Arg2, ...)
					if comp, ok := inner.(*types.Compound); ok {
						args := make([]types.Term, len(comp.Args)+len(g.Args)-1)
						copy(args, comp.Args)
						for i := 1; i < len(g.Args); i++ {
							args[len(comp.Args)+i-1] = g.Args[i]
						}
						newGoal := types.MakeCompound(comp.Functor, args...)
						newGoals := append([]types.Term{newGoal}, rest...)
						return true, e.solve(newGoals, b, callback)
					}
					if a, ok := inner.(*types.Atom); ok {
						extraArgs := make([]types.Term, len(g.Args)-1)
						for i := 1; i < len(g.Args); i++ {
							extraArgs[i-1] = g.Args[i]
						}
						newGoal := types.MakeCompound(a.Name, extraArgs...)
						newGoals := append([]types.Term{newGoal}, rest...)
						return true, e.solve(newGoals, b, callback)
					}
				} else {
					newGoals := append([]types.Term{inner}, rest...)
					return true, e.solve(newGoals, b, callback)
				}
				return true, false
			}

		case "listing":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if a, ok := resolved.(*types.Atom); ok {
					for _, c := range e.Database {
						if c.Head.Functor == a.Name {
							fmt.Fprintln(e.Output, c.String())
						}
					}
					return true, e.solve(rest, b, callback)
				}
				// functor/arity
				if c, ok := resolved.(*types.Compound); ok && c.Functor == "/" && len(c.Args) == 2 {
					if a, ok := c.Args[0].(*types.Atom); ok {
						if n, ok := c.Args[1].(*types.Number); ok {
							for _, cl := range e.Database {
								if cl.Head.Functor == a.Name && int64(len(cl.Head.Args)) == n.Value {
									fmt.Fprintln(e.Output, cl.String())
								}
							}
							return true, e.solve(rest, b, callback)
						}
					}
				}
				return true, false
			}

		case "tab":
			if len(g.Args) == 1 {
				resolved := b.Resolve(g.Args[0])
				if n, ok := resolved.(*types.Number); ok {
					for i := int64(0); i < n.Value; i++ {
						fmt.Fprint(e.Output, " ")
					}
					return true, e.solve(rest, b, callback)
				}
				return true, false
			}

		case "succ_or_zero":
			// Not standard, skip
		}
	}
	return false, false
}

func (e *Engine) evalArith(t types.Term, b Binding) (types.Term, error) {
	t = b.Resolve(t)

	switch v := t.(type) {
	case *types.Number:
		return v, nil
	case *types.Float:
		return v, nil
	case *types.Atom:
		switch v.Name {
		case "pi":
			return &types.Float{Value: math.Pi}, nil
		case "e":
			return &types.Float{Value: math.E}, nil
		case "inf", "infinity":
			return &types.Float{Value: math.Inf(1)}, nil
		case "random_float":
			return &types.Float{Value: 0.5}, nil // Simplified
		default:
			return nil, fmt.Errorf("unknown arithmetic atom: %s", v.Name)
		}
	case *types.Compound:
		switch v.Functor {
		case "+":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				return numericOp(a, bv, func(x, y int64) int64 { return x + y },
					func(x, y float64) float64 { return x + y })
			}
		case "-":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				return numericOp(a, bv, func(x, y int64) int64 { return x - y },
					func(x, y float64) float64 { return x - y })
			}
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				if n, ok := a.(*types.Number); ok {
					return types.MakeNumber(-n.Value), nil
				}
				if f, ok := a.(*types.Float); ok {
					return &types.Float{Value: -f.Value}, nil
				}
			}
		case "*":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				return numericOp(a, bv, func(x, y int64) int64 { return x * y },
					func(x, y float64) float64 { return x * y })
			}
		case "/":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				// Float division
				af := toFloat(a)
				bf := toFloat(bv)
				if bf == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return &types.Float{Value: af / bf}, nil
			}
		case "//":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				an, ok1 := a.(*types.Number)
				bn, ok2 := bv.(*types.Number)
				if !ok1 || !ok2 {
					return nil, fmt.Errorf("integer division requires integers")
				}
				if bn.Value == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return types.MakeNumber(an.Value / bn.Value), nil
			}
		case "mod":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				an, ok1 := a.(*types.Number)
				bn, ok2 := bv.(*types.Number)
				if !ok1 || !ok2 {
					return nil, fmt.Errorf("mod requires integers")
				}
				if bn.Value == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return types.MakeNumber(an.Value % bn.Value), nil
			}
		case "abs":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				if n, ok := a.(*types.Number); ok {
					if n.Value < 0 {
						return types.MakeNumber(-n.Value), nil
					}
					return n, nil
				}
				if f, ok := a.(*types.Float); ok {
					return &types.Float{Value: math.Abs(f.Value)}, nil
				}
			}
		case "max":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				if toFloat(a) >= toFloat(bv) {
					return a, nil
				}
				return bv, nil
			}
		case "min":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				if toFloat(a) <= toFloat(bv) {
					return a, nil
				}
				return bv, nil
			}
		case "sqrt":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return &types.Float{Value: math.Sqrt(toFloat(a))}, nil
			}
		case "**", "^":
			if len(v.Args) == 2 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				bv, err := e.evalArith(v.Args[1], b)
				if err != nil {
					return nil, err
				}
				result := math.Pow(toFloat(a), toFloat(bv))
				// If both inputs were integers and result is integral, return integer
				if _, ok := a.(*types.Number); ok {
					if _, ok := bv.(*types.Number); ok {
						if result == math.Floor(result) && !math.IsInf(result, 0) {
							return types.MakeNumber(int64(result)), nil
						}
					}
				}
				return &types.Float{Value: result}, nil
			}
		case "truncate":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return types.MakeNumber(int64(toFloat(a))), nil
			}
		case "round":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return types.MakeNumber(int64(math.Round(toFloat(a)))), nil
			}
		case "ceiling":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return types.MakeNumber(int64(math.Ceil(toFloat(a)))), nil
			}
		case "floor":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return types.MakeNumber(int64(math.Floor(toFloat(a)))), nil
			}
		case "sin":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return &types.Float{Value: math.Sin(toFloat(a))}, nil
			}
		case "cos":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return &types.Float{Value: math.Cos(toFloat(a))}, nil
			}
		case "log":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return &types.Float{Value: math.Log(toFloat(a))}, nil
			}
		case "exp":
			if len(v.Args) == 1 {
				a, err := e.evalArith(v.Args[0], b)
				if err != nil {
					return nil, err
				}
				return &types.Float{Value: math.Exp(toFloat(a))}, nil
			}
		}
		return nil, fmt.Errorf("unknown arithmetic expression: %s", v.String())
	default:
		return nil, fmt.Errorf("cannot evaluate %s as arithmetic", t.String())
	}
}

func toFloat(t types.Term) float64 {
	switch v := t.(type) {
	case *types.Number:
		return float64(v.Value)
	case *types.Float:
		return v.Value
	default:
		return 0
	}
}

func numericOp(a, bv types.Term, intOp func(int64, int64) int64, floatOp func(float64, float64) float64) (types.Term, error) {
	an, aIsInt := a.(*types.Number)
	bn, bIsInt := bv.(*types.Number)
	if aIsInt && bIsInt {
		return types.MakeNumber(intOp(an.Value, bn.Value)), nil
	}
	return &types.Float{Value: floatOp(toFloat(a), toFloat(bv))}, nil
}

func termToString(t types.Term) string {
	switch v := t.(type) {
	case *types.Atom:
		return v.Name
	default:
		return t.String()
	}
}

func termToClause(t types.Term) *types.Clause {
	switch v := t.(type) {
	case *types.Compound:
		if v.Functor == ":-" && len(v.Args) == 2 {
			head, ok := goalToCompound(v.Args[0])
			if !ok {
				return nil
			}
			var body []types.Term
			collectBody(v.Args[1], &body)
			return &types.Clause{Head: head, Body: body}
		}
		return &types.Clause{Head: v}
	case *types.Atom:
		return &types.Clause{Head: types.MakeCompound(v.Name)}
	default:
		return nil
	}
}

func collectBody(t types.Term, body *[]types.Term) {
	if c, ok := t.(*types.Compound); ok && c.Functor == "," && len(c.Args) == 2 {
		collectBody(c.Args[0], body)
		collectBody(c.Args[1], body)
	} else {
		*body = append(*body, t)
	}
}

func matchBody(a, b []types.Term, binding Binding) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !Unify(a[i], b[i], binding.Clone()) {
			return false
		}
	}
	return true
}

func isList(t types.Term) bool {
	switch v := t.(type) {
	case *types.Atom:
		return v.Name == "[]"
	case *types.Compound:
		return v.Functor == "." && len(v.Args) == 2 && isList(v.Args[1])
	default:
		return false
	}
}

func listToSlice(t types.Term) []types.Term {
	var result []types.Term
	current := t
	for {
		switch v := current.(type) {
		case *types.Atom:
			if v.Name == "[]" {
				return result
			}
			return nil
		case *types.Compound:
			if v.Functor == "." && len(v.Args) == 2 {
				result = append(result, v.Args[0])
				current = v.Args[1]
				continue
			}
			return nil
		default:
			return nil
		}
	}
}

func (e *Engine) solveAppend(l1, l2, l3 types.Term, rest []types.Term, b Binding, callback func(Binding) bool) bool {
	// Try: append([], L, L)
	newB := b.Clone()
	if Unify(l1, types.MakeAtom("[]"), newB) && Unify(l2, l3, newB) {
		if e.solve(rest, newB, callback) {
			return true
		}
	}

	// Try: append([H|T], L, [H|R]) :- append(T, L, R)
	h := types.MakeVar(e.freshVar())
	t := types.MakeVar(e.freshVar())
	r := types.MakeVar(e.freshVar())
	newB = b.Clone()
	cons1 := types.MakeCompound(".", h, t)
	cons2 := types.MakeCompound(".", h, r)
	if Unify(l1, cons1, newB) && Unify(l3, cons2, newB) {
		tResolved := newB.Resolve(t)
		rResolved := newB.Resolve(r)
		return e.solveAppend(tResolved, l2, rResolved, rest, newB, callback)
	}
	return false
}

func (e *Engine) solveMember(elem, list types.Term, rest []types.Term, b Binding, callback func(Binding) bool) bool {
	if comp, ok := list.(*types.Compound); ok && comp.Functor == "." && len(comp.Args) == 2 {
		// member(X, [X|_])
		newB := b.Clone()
		if Unify(elem, comp.Args[0], newB) {
			if e.solve(rest, newB, callback) {
				return true
			}
		}
		// member(X, [_|T]) :- member(X, T)
		return e.solveMember(elem, comp.Args[1], rest, b, callback)
	}
	return false
}

func (e *Engine) solveMaplist(g *types.Compound, rest []types.Term, b Binding, callback func(Binding) bool) bool {
	pred := b.Resolve(g.Args[0])
	if len(g.Args) == 2 {
		// maplist(Pred, List)
		list := b.Resolve(g.Args[1])
		elems := listToSlice(list)
		if elems == nil {
			if a, ok := list.(*types.Atom); ok && a.Name == "[]" {
				return e.solve(rest, b, callback)
			}
			return false
		}
		return e.solveMaplistHelper(pred, elems, 0, rest, b, callback)
	}
	if len(g.Args) == 3 {
		// maplist(Pred, List1, List2)
		list1 := b.Resolve(g.Args[1])
		elems1 := listToSlice(list1)
		if elems1 == nil {
			if a, ok := list1.(*types.Atom); ok && a.Name == "[]" {
				newB := b.Clone()
				if Unify(g.Args[2], types.MakeAtom("[]"), newB) {
					return e.solve(rest, newB, callback)
				}
			}
			return false
		}
		results := make([]types.Term, len(elems1))
		for i := range results {
			results[i] = types.MakeVar(e.freshVar())
		}
		return e.solveMaplist2Helper(pred, elems1, results, 0, g.Args[2], rest, b, callback)
	}
	return false
}

func (e *Engine) solveMaplistHelper(pred types.Term, elems []types.Term, idx int, rest []types.Term, b Binding, callback func(Binding) bool) bool {
	if idx >= len(elems) {
		return e.solve(rest, b, callback)
	}
	var goal types.Term
	switch p := pred.(type) {
	case *types.Atom:
		goal = types.MakeCompound(p.Name, elems[idx])
	case *types.Compound:
		args := make([]types.Term, len(p.Args)+1)
		copy(args, p.Args)
		args[len(p.Args)] = elems[idx]
		goal = types.MakeCompound(p.Functor, args...)
	default:
		return false
	}
	return e.solve([]types.Term{goal}, b, func(newB Binding) bool {
		return e.solveMaplistHelper(pred, elems, idx+1, rest, newB, callback)
	})
}

func (e *Engine) solveMaplist2Helper(pred types.Term, inputs, outputs []types.Term, idx int, resultList types.Term, rest []types.Term, b Binding, callback func(Binding) bool) bool {
	if idx >= len(inputs) {
		list := types.MakeList(outputs...)
		newB := b.Clone()
		for i, o := range outputs {
			resolved := b.Resolve(o)
			outputs[i] = resolved
			_ = resolved
		}
		resolvedOutputs := make([]types.Term, len(outputs))
		for i, o := range outputs {
			resolvedOutputs[i] = b.Resolve(o)
		}
		list = types.MakeList(resolvedOutputs...)
		if Unify(resultList, list, newB) {
			return e.solve(rest, newB, callback)
		}
		return false
	}
	var goal types.Term
	switch p := pred.(type) {
	case *types.Atom:
		goal = types.MakeCompound(p.Name, inputs[idx], outputs[idx])
	case *types.Compound:
		args := make([]types.Term, len(p.Args)+2)
		copy(args, p.Args)
		args[len(p.Args)] = inputs[idx]
		args[len(p.Args)+1] = outputs[idx]
		goal = types.MakeCompound(p.Functor, args...)
	default:
		return false
	}
	return e.solve([]types.Term{goal}, b, func(newB Binding) bool {
		return e.solveMaplist2Helper(pred, inputs, outputs, idx+1, resultList, rest, newB, callback)
	})
}

func (e *Engine) copyTerm(t types.Term, mapping map[string]string) types.Term {
	switch v := t.(type) {
	case *types.Variable:
		if newName, ok := mapping[v.Name]; ok {
			return types.MakeVar(newName)
		}
		newName := e.freshVar()
		mapping[v.Name] = newName
		return types.MakeVar(newName)
	case *types.Compound:
		args := make([]types.Term, len(v.Args))
		for i, a := range v.Args {
			args[i] = e.copyTerm(a, mapping)
		}
		return types.MakeCompound(v.Functor, args...)
	default:
		return t
	}
}

func isGround(t types.Term) bool {
	switch v := t.(type) {
	case *types.Variable:
		return false
	case *types.Compound:
		for _, a := range v.Args {
			if !isGround(a) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func formatString(format string, args []types.Term) string {
	var result strings.Builder
	argIdx := 0
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '~' && i+1 < len(runes) {
			i++
			switch runes[i] {
			case 'w':
				if argIdx < len(args) {
					result.WriteString(args[argIdx].String())
					argIdx++
				}
			case 'a':
				if argIdx < len(args) {
					result.WriteString(termToString(args[argIdx]))
					argIdx++
				}
			case 'd':
				if argIdx < len(args) {
					result.WriteString(args[argIdx].String())
					argIdx++
				}
			case 'n':
				result.WriteRune('\n')
			case 't':
				result.WriteRune('\t')
			case '~':
				result.WriteRune('~')
			default:
				result.WriteRune('~')
				result.WriteRune(runes[i])
			}
		} else {
			result.WriteRune(runes[i])
		}
	}
	return result.String()
}

// Simple merge sort for terms
func msort(terms []types.Term) []types.Term {
	if len(terms) <= 1 {
		return terms
	}
	mid := len(terms) / 2
	left := msort(terms[:mid])
	right := msort(terms[mid:])
	return merge(left, right)
}

func merge(a, b []types.Term) []types.Term {
	result := make([]types.Term, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if compareTerm(a[i], b[j]) <= 0 {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func compareTerm(a, b types.Term) int {
	// Standard order: var < number < atom < compound
	orderA := termOrder(a)
	orderB := termOrder(b)
	if orderA != orderB {
		if orderA < orderB {
			return -1
		}
		return 1
	}
	switch va := a.(type) {
	case *types.Number:
		vb := b.(*types.Number)
		if va.Value < vb.Value {
			return -1
		}
		if va.Value > vb.Value {
			return 1
		}
		return 0
	case *types.Float:
		vb := b.(*types.Float)
		if va.Value < vb.Value {
			return -1
		}
		if va.Value > vb.Value {
			return 1
		}
		return 0
	case *types.Atom:
		vb := b.(*types.Atom)
		if va.Name < vb.Name {
			return -1
		}
		if va.Name > vb.Name {
			return 1
		}
		return 0
	case *types.Compound:
		vb := b.(*types.Compound)
		if len(va.Args) != len(vb.Args) {
			if len(va.Args) < len(vb.Args) {
				return -1
			}
			return 1
		}
		if va.Functor != vb.Functor {
			if va.Functor < vb.Functor {
				return -1
			}
			return 1
		}
		for i := range va.Args {
			c := compareTerm(va.Args[i], vb.Args[i])
			if c != 0 {
				return c
			}
		}
		return 0
	}
	return 0
}

func termOrder(t types.Term) int {
	switch t.(type) {
	case *types.Variable:
		return 0
	case *types.Number:
		return 1
	case *types.Float:
		return 1
	case *types.Atom:
		return 2
	case *types.Compound:
		return 3
	default:
		return 4
	}
}

func removeDuplicates(terms []types.Term) []types.Term {
	if len(terms) == 0 {
		return terms
	}
	result := []types.Term{terms[0]}
	for i := 1; i < len(terms); i++ {
		if !terms[i].Equal(terms[i-1]) {
			result = append(result, terms[i])
		}
	}
	return result
}

// CollectVars collects all unique user-visible variable names from goals.
func CollectVars(goals []types.Term) []string {
	seen := make(map[string]bool)
	var vars []string
	for _, g := range goals {
		collectVarsFromTerm(g, seen, &vars)
	}
	return vars
}

func collectVarsFromTerm(t types.Term, seen map[string]bool, vars *[]string) {
	switch v := t.(type) {
	case *types.Variable:
		if !seen[v.Name] && !strings.HasPrefix(v.Name, "_") {
			seen[v.Name] = true
			*vars = append(*vars, v.Name)
		}
	case *types.Compound:
		for _, a := range v.Args {
			collectVarsFromTerm(a, seen, vars)
		}
	}
}
