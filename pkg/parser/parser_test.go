package parser

import (
	"testing"

	"github.com/ieee0824/gorollog/pkg/lexer"
	"github.com/ieee0824/gorollog/pkg/types"
)

func parse(input string) ([]*types.Clause, error) {
	tokens, err := lexer.New(input).Tokenize()
	if err != nil {
		return nil, err
	}
	return New(tokens).ParseProgram()
}

func parseQuery(input string) ([]types.Term, error) {
	tokens, err := lexer.New(input).Tokenize()
	if err != nil {
		return nil, err
	}
	return New(tokens).ParseQuery()
}

func TestParseFact(t *testing.T) {
	clauses, err := parse("parent(tom, bob).")
	if err != nil {
		t.Fatal(err)
	}
	if len(clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(clauses))
	}
	c := clauses[0]
	if c.Head.Functor != "parent" || len(c.Head.Args) != 2 {
		t.Errorf("expected parent/2, got %s/%d", c.Head.Functor, len(c.Head.Args))
	}
	if len(c.Body) != 0 {
		t.Errorf("fact should have no body, got %d goals", len(c.Body))
	}
}

func TestParseRule(t *testing.T) {
	clauses, err := parse("father(X, Y) :- parent(X, Y), male(X).")
	if err != nil {
		t.Fatal(err)
	}
	c := clauses[0]
	if c.Head.Functor != "father" {
		t.Errorf("expected head functor 'father', got %q", c.Head.Functor)
	}
	if len(c.Body) != 2 {
		t.Errorf("expected 2 body goals, got %d", len(c.Body))
	}
}

func TestParseMultipleClauses(t *testing.T) {
	clauses, err := parse("a(1). b(2). c(3).")
	if err != nil {
		t.Fatal(err)
	}
	if len(clauses) != 3 {
		t.Fatalf("expected 3 clauses, got %d", len(clauses))
	}
}

func TestParseQuery(t *testing.T) {
	goals, err := parseQuery("parent(tom, X).")
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(goals))
	}
	comp, ok := goals[0].(*types.Compound)
	if !ok {
		t.Fatalf("expected compound, got %T", goals[0])
	}
	if comp.Functor != "parent" {
		t.Errorf("expected functor 'parent', got %q", comp.Functor)
	}
}

func TestParseConjunctionQuery(t *testing.T) {
	goals, err := parseQuery("parent(X, Y), male(X).")
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(goals))
	}
}

func TestParseArithmetic(t *testing.T) {
	goals, err := parseQuery("X is 3 + 4 * 2.")
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(goals))
	}
	// X is 3 + (4 * 2)
	is, ok := goals[0].(*types.Compound)
	if !ok || is.Functor != "is" {
		t.Fatalf("expected 'is' compound, got %v", goals[0])
	}
	plus, ok := is.Args[1].(*types.Compound)
	if !ok || plus.Functor != "+" {
		t.Fatalf("expected '+', got %v", is.Args[1])
	}
	mul, ok := plus.Args[1].(*types.Compound)
	if !ok || mul.Functor != "*" {
		t.Fatalf("expected '*', got %v", plus.Args[1])
	}
}

func TestParseList(t *testing.T) {
	goals, err := parseQuery("X = [1, 2, 3].")
	if err != nil {
		t.Fatal(err)
	}
	eq := goals[0].(*types.Compound)
	list := eq.Args[1]
	if list.String() != "[1, 2, 3]" {
		t.Errorf("expected [1, 2, 3], got %s", list.String())
	}
}

func TestParseEmptyList(t *testing.T) {
	goals, err := parseQuery("X = [].")
	if err != nil {
		t.Fatal(err)
	}
	eq := goals[0].(*types.Compound)
	if eq.Args[1].String() != "[]" {
		t.Errorf("expected [], got %s", eq.Args[1].String())
	}
}

func TestParseListWithTail(t *testing.T) {
	goals, err := parseQuery("X = [H|T].")
	if err != nil {
		t.Fatal(err)
	}
	eq := goals[0].(*types.Compound)
	list := eq.Args[1].(*types.Compound)
	if list.Functor != "." {
		t.Errorf("expected list cons, got %s", list.Functor)
	}
}

func TestParsePower(t *testing.T) {
	goals, err := parseQuery("X is 2 ** 10.")
	if err != nil {
		t.Fatal(err)
	}
	is := goals[0].(*types.Compound)
	pow := is.Args[1].(*types.Compound)
	if pow.Functor != "**" {
		t.Errorf("expected **, got %s", pow.Functor)
	}
}

func TestParseIfThenElse(t *testing.T) {
	goals, err := parseQuery("(X > 0 -> Y = pos ; Y = neg).")
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(goals))
	}
	semi := goals[0].(*types.Compound)
	if semi.Functor != ";" {
		t.Fatalf("expected ;, got %s", semi.Functor)
	}
	arrow := semi.Args[0].(*types.Compound)
	if arrow.Functor != "->" {
		t.Fatalf("expected ->, got %s", arrow.Functor)
	}
	// Condition should be >(X, 0)
	cond := arrow.Args[0].(*types.Compound)
	if cond.Functor != ">" {
		t.Errorf("expected > in condition, got %s", cond.Functor)
	}
	// Then should be =(Y, pos)
	then := arrow.Args[1].(*types.Compound)
	if then.Functor != "=" {
		t.Errorf("expected = in then, got %s", then.Functor)
	}
}

func TestParseNestedRule(t *testing.T) {
	input := "max_list([X|Xs], Max) :- max_list(Xs, MR), (X > MR -> Max = X ; Max = MR)."
	clauses, err := parse(input)
	if err != nil {
		t.Fatal(err)
	}
	c := clauses[0]
	if c.Head.Functor != "max_list" {
		t.Errorf("expected head 'max_list', got %q", c.Head.Functor)
	}
	if len(c.Body) != 2 {
		t.Errorf("expected 2 body goals, got %d", len(c.Body))
	}
	// Second body goal should be ;(->(...), ...)
	semi, ok := c.Body[1].(*types.Compound)
	if !ok || semi.Functor != ";" {
		t.Errorf("expected ; in body[1], got %v", c.Body[1])
	}
	arrow, ok := semi.Args[0].(*types.Compound)
	if !ok || arrow.Functor != "->" {
		t.Errorf("expected -> in ;.Args[0], got %v", semi.Args[0])
	}
}

func TestParseAssertRule(t *testing.T) {
	// assert((foo(X) :- bar(X))) should parse :- inside parens as infix
	goals, err := parseQuery("assert((foo(X) :- bar(X))).")
	if err != nil {
		t.Fatal(err)
	}
	assertGoal := goals[0].(*types.Compound)
	if assertGoal.Functor != "assert" {
		t.Fatalf("expected assert, got %s", assertGoal.Functor)
	}
	inner := assertGoal.Args[0].(*types.Compound)
	if inner.Functor != ":-" {
		t.Errorf("expected :- inside assert, got %s", inner.Functor)
	}
}

func TestParseComparison(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{"X = Y.", "="},
		{"X \\= Y.", "\\="},
		{"X == Y.", "=="},
		{"X < Y.", "<"},
		{"X > Y.", ">"},
		{"X =< Y.", "=<"},
		{"X >= Y.", ">="},
		{"X =:= Y.", "=:="},
	}
	for _, tt := range tests {
		goals, err := parseQuery(tt.input)
		if err != nil {
			t.Errorf("%s: %v", tt.input, err)
			continue
		}
		comp := goals[0].(*types.Compound)
		if comp.Functor != tt.op {
			t.Errorf("%s: expected op %q, got %q", tt.input, tt.op, comp.Functor)
		}
	}
}

func TestParseAnonymousVariable(t *testing.T) {
	goals, err := parseQuery("foo(_, _).")
	if err != nil {
		t.Fatal(err)
	}
	comp := goals[0].(*types.Compound)
	v1 := comp.Args[0].(*types.Variable)
	v2 := comp.Args[1].(*types.Variable)
	if v1.Name == v2.Name {
		t.Error("anonymous variables should get unique names")
	}
}
