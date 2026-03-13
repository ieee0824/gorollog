package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ieee0824/gorollog/pkg/lexer"
	"github.com/ieee0824/gorollog/pkg/parser"
	"github.com/ieee0824/gorollog/pkg/types"
)

func loadEngine(source string) *Engine {
	e := New()
	e.Output = &bytes.Buffer{}
	lex := lexer.New(source)
	tokens, _ := lex.Tokenize()
	p := parser.New(tokens)
	clauses, _ := p.ParseProgram()
	for _, c := range clauses {
		e.AddClause(c)
	}
	return e
}

func query(e *Engine, input string) []Binding {
	lex := lexer.New(input)
	tokens, _ := lex.Tokenize()
	p := parser.New(tokens)
	goals, _ := p.ParseQuery()

	var results []Binding
	e.Solve(goals, NewBinding(), func(b Binding) bool {
		results = append(results, b.Clone())
		return false
	})
	return results
}

func queryFirst(e *Engine, input string) Binding {
	results := query(e, input)
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func TestFactQuery(t *testing.T) {
	e := loadEngine("parent(tom, bob). parent(tom, liz).")
	results := query(e, "parent(tom, X).")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	x0 := results[0].Resolve(types.MakeVar("X"))
	x1 := results[1].Resolve(types.MakeVar("X"))
	if x0.String() != "bob" {
		t.Errorf("first result: expected bob, got %s", x0.String())
	}
	if x1.String() != "liz" {
		t.Errorf("second result: expected liz, got %s", x1.String())
	}
}

func TestRuleQuery(t *testing.T) {
	e := loadEngine(`
		parent(tom, bob).
		parent(tom, liz).
		male(tom).
		father(X, Y) :- parent(X, Y), male(X).
	`)
	results := query(e, "father(tom, X).")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestRecursion(t *testing.T) {
	e := loadEngine(`
		parent(tom, bob).
		parent(bob, ann).
		ancestor(X, Y) :- parent(X, Y).
		ancestor(X, Y) :- parent(X, Z), ancestor(Z, Y).
	`)
	results := query(e, "ancestor(tom, X).")
	if len(results) != 2 {
		t.Fatalf("expected 2 results (bob, ann), got %d", len(results))
	}
}

func TestArithmetic(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "X is 3 + 4 * 2.")
	if b == nil {
		t.Fatal("query failed")
	}
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "11" {
		t.Errorf("expected 11, got %s", x.String())
	}
}

func TestArithmeticMod(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "X is 10 mod 3.")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "1" {
		t.Errorf("expected 1, got %s", x.String())
	}
}

func TestArithmeticPower(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "X is 2 ** 10.")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "1024" {
		t.Errorf("expected 1024, got %s", x.String())
	}
}

func TestComparison(t *testing.T) {
	e := loadEngine("")
	if len(query(e, "3 > 2.")) != 1 {
		t.Error("3 > 2 should succeed")
	}
	if len(query(e, "2 > 3.")) != 0 {
		t.Error("2 > 3 should fail")
	}
	if len(query(e, "3 =:= 3.")) != 1 {
		t.Error("3 =:= 3 should succeed")
	}
	if len(query(e, "3 =\\= 4.")) != 1 {
		t.Error("3 =\\= 4 should succeed")
	}
}

func TestUnification(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "X = hello.")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "hello" {
		t.Errorf("expected hello, got %s", x.String())
	}
}

func TestUnificationFail(t *testing.T) {
	e := loadEngine("")
	if len(query(e, "hello = world.")) != 0 {
		t.Error("hello = world should fail")
	}
}

func TestNotEqual(t *testing.T) {
	e := loadEngine("")
	if len(query(e, "a \\= b.")) != 1 {
		t.Error("a \\= b should succeed")
	}
	if len(query(e, "a \\= a.")) != 0 {
		t.Error("a \\= a should fail")
	}
}

func TestAppend(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "append([1, 2], [3, 4], X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "[1, 2, 3, 4]" {
		t.Errorf("expected [1, 2, 3, 4], got %s", x.String())
	}
}

func TestMember(t *testing.T) {
	e := loadEngine("")
	results := query(e, "member(X, [a, b, c]).")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestLength(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "length([a, b, c], N).")
	n := b.Resolve(types.MakeVar("N"))
	if n.String() != "3" {
		t.Errorf("expected 3, got %s", n.String())
	}
}

func TestReverse(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "reverse([1, 2, 3], X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "[3, 2, 1]" {
		t.Errorf("expected [3, 2, 1], got %s", x.String())
	}
}

func TestFindall(t *testing.T) {
	e := loadEngine("color(red). color(green). color(blue).")
	b := queryFirst(e, "findall(X, color(X), L).")
	l := b.Resolve(types.MakeVar("L"))
	if l.String() != "[red, green, blue]" {
		t.Errorf("expected [red, green, blue], got %s", l.String())
	}
}

func TestAssertRetract(t *testing.T) {
	e := loadEngine("")
	query(e, "assert(fact(1)).")
	query(e, "assert(fact(2)).")
	results := query(e, "fact(X).")
	if len(results) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(results))
	}
	query(e, "retract(fact(1)).")
	results = query(e, "fact(X).")
	if len(results) != 1 {
		t.Fatalf("expected 1 fact after retract, got %d", len(results))
	}
}

func TestNegation(t *testing.T) {
	e := loadEngine("likes(tom, cats).")
	if len(query(e, "\\+ likes(tom, dogs).")) != 1 {
		t.Error("\\+ likes(tom, dogs) should succeed")
	}
	if len(query(e, "\\+ likes(tom, cats).")) != 0 {
		t.Error("\\+ likes(tom, cats) should fail")
	}
}

func TestCut(t *testing.T) {
	e := loadEngine(`
		f(X, 0) :- X < 3, !.
		f(X, 2) :- X < 6, !.
		f(X, 4).
	`)
	b := queryFirst(e, "f(1, Y).")
	y := b.Resolve(types.MakeVar("Y"))
	if y.String() != "0" {
		t.Errorf("expected 0, got %s", y.String())
	}
	results := query(e, "f(1, Y).")
	if len(results) != 1 {
		t.Errorf("cut should prevent backtracking, got %d results", len(results))
	}
}

func TestBetween(t *testing.T) {
	e := loadEngine("")
	results := query(e, "between(1, 5, X).")
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestWrite(t *testing.T) {
	e := loadEngine("")
	buf := &bytes.Buffer{}
	e.Output = buf
	query(e, "write(hello), write(' '), write(world), nl.")
	if buf.String() != "hello world\n" {
		t.Errorf("expected %q, got %q", "hello world\n", buf.String())
	}
}

func TestFactorial(t *testing.T) {
	e := loadEngine(`
		factorial(0, 1).
		factorial(N, F) :- N > 0, N1 is N - 1, factorial(N1, F1), F is N * F1.
	`)
	b := queryFirst(e, "factorial(10, X).")
	if b == nil {
		t.Fatal("factorial(10, X) failed")
	}
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "3628800" {
		t.Errorf("expected 3628800, got %s", x.String())
	}
}

func TestFibonacci(t *testing.T) {
	e := loadEngine(`
		fib(0, 0).
		fib(1, 1).
		fib(N, F) :- N > 1, N1 is N - 1, N2 is N - 2, fib(N1, F1), fib(N2, F2), F is F1 + F2.
	`)
	b := queryFirst(e, "fib(10, X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "55" {
		t.Errorf("expected 55, got %s", x.String())
	}
}

func TestMaxList(t *testing.T) {
	e := loadEngine(`
		max_list([X], X).
		max_list([X|Xs], Max) :- max_list(Xs, MR), (X > MR -> Max = X ; Max = MR).
	`)
	b := queryFirst(e, "max_list([3, 1, 4, 1, 5, 9, 2, 6], X).")
	if b == nil {
		t.Fatal("max_list failed")
	}
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "9" {
		t.Errorf("expected 9, got %s", x.String())
	}
}

func TestSumList(t *testing.T) {
	e := loadEngine(`
		sum_list([], 0).
		sum_list([H|T], S) :- sum_list(T, S1), S is S1 + H.
	`)
	b := queryFirst(e, "sum_list([1, 2, 3, 4, 5], X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "15" {
		t.Errorf("expected 15, got %s", x.String())
	}
}

func TestTypeChecks(t *testing.T) {
	e := loadEngine("")
	if len(query(e, "atom(hello).")) != 1 {
		t.Error("atom(hello) should succeed")
	}
	if len(query(e, "number(42).")) != 1 {
		t.Error("number(42) should succeed")
	}
	if len(query(e, "integer(42).")) != 1 {
		t.Error("integer(42) should succeed")
	}
	if len(query(e, "atom(42).")) != 0 {
		t.Error("atom(42) should fail")
	}
}

func TestAtomChars(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "atom_chars(hello, X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "[h, e, l, l, o]" {
		t.Errorf("expected [h, e, l, l, o], got %s", x.String())
	}
}

func TestAtomLength(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "atom_length(hello, N).")
	n := b.Resolve(types.MakeVar("N"))
	if n.String() != "5" {
		t.Errorf("expected 5, got %s", n.String())
	}
}

func TestAtomConcat(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "atom_concat(hello, world, X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "helloworld" {
		t.Errorf("expected helloworld, got %s", x.String())
	}
}

func TestFunctor(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "functor(f(a, b), F, A).")
	f := b.Resolve(types.MakeVar("F"))
	a := b.Resolve(types.MakeVar("A"))
	if f.String() != "f" {
		t.Errorf("expected f, got %s", f.String())
	}
	if a.String() != "2" {
		t.Errorf("expected 2, got %s", a.String())
	}
}

func TestUniv(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "f(a, b) =.. X.")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "[f, a, b]" {
		t.Errorf("expected [f, a, b], got %s", x.String())
	}
}

func TestDisjunction(t *testing.T) {
	e := loadEngine("a(1). b(2).")
	results := query(e, "(a(X) ; b(X)).")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestTrueFalse(t *testing.T) {
	e := loadEngine("")
	if len(query(e, "true.")) != 1 {
		t.Error("true should succeed")
	}
	if len(query(e, "fail.")) != 0 {
		t.Error("fail should fail")
	}
}

func TestCollectVars(t *testing.T) {
	goals := []types.Term{
		types.MakeCompound("f", types.MakeVar("X"), types.MakeVar("Y")),
		types.MakeCompound("g", types.MakeVar("X"), types.MakeVar("Z")),
	}
	vars := CollectVars(goals)
	if len(vars) != 3 {
		t.Fatalf("expected 3 vars, got %d: %v", len(vars), vars)
	}
	if !contains(vars, "X") || !contains(vars, "Y") || !contains(vars, "Z") {
		t.Errorf("expected X, Y, Z in vars, got %v", vars)
	}
}

func TestCollectVarsSkipsInternal(t *testing.T) {
	goals := []types.Term{
		types.MakeCompound("f", types.MakeVar("_G1"), types.MakeVar("X")),
	}
	vars := CollectVars(goals)
	if len(vars) != 1 || vars[0] != "X" {
		t.Errorf("expected [X], got %v", vars)
	}
}

func TestSort(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "sort([3, 1, 2, 1], X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "[1, 2, 3]" {
		t.Errorf("expected [1, 2, 3], got %s", x.String())
	}
}

func TestMsort(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "msort([3, 1, 2, 1], X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "[1, 1, 2, 3]" {
		t.Errorf("expected [1, 1, 2, 3], got %s", x.String())
	}
}

func TestLast(t *testing.T) {
	e := loadEngine("")
	b := queryFirst(e, "last([a, b, c], X).")
	x := b.Resolve(types.MakeVar("X"))
	if x.String() != "c" {
		t.Errorf("expected c, got %s", x.String())
	}
}

func TestAssertRule(t *testing.T) {
	e := loadEngine("")
	buf := &bytes.Buffer{}
	e.Output = buf
	query(e, "assert(bar(hello)).")
	query(e, "assert((foo(X) :- bar(X))).")
	b := queryFirst(e, "foo(Y).")
	if b == nil {
		t.Fatal("foo(Y) should succeed")
	}
	y := b.Resolve(types.MakeVar("Y"))
	if y.String() != "hello" {
		t.Errorf("expected hello, got %s", y.String())
	}
}

func TestFormat(t *testing.T) {
	e := loadEngine("")
	buf := &bytes.Buffer{}
	e.Output = buf
	query(e, "format('hello ~w~n', [world]).")
	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected 'hello world' in output, got %q", out)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
