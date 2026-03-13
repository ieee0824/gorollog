package types

import "testing"

func TestAtom(t *testing.T) {
	a := MakeAtom("hello")
	if a.String() != "hello" {
		t.Errorf("got %q, want %q", a.String(), "hello")
	}
	if !a.Equal(MakeAtom("hello")) {
		t.Error("same atoms should be equal")
	}
	if a.Equal(MakeAtom("world")) {
		t.Error("different atoms should not be equal")
	}
	if a.Equal(MakeNumber(0)) {
		t.Error("atom should not equal number")
	}
}

func TestNumber(t *testing.T) {
	n := MakeNumber(42)
	if n.String() != "42" {
		t.Errorf("got %q, want %q", n.String(), "42")
	}
	if !n.Equal(MakeNumber(42)) {
		t.Error("same numbers should be equal")
	}
	if n.Equal(MakeNumber(99)) {
		t.Error("different numbers should not be equal")
	}
}

func TestVariable(t *testing.T) {
	v := MakeVar("X")
	if v.String() != "X" {
		t.Errorf("got %q, want %q", v.String(), "X")
	}
	if !v.Equal(MakeVar("X")) {
		t.Error("same variables should be equal")
	}
	if v.Equal(MakeVar("Y")) {
		t.Error("different variables should not be equal")
	}
}

func TestCompound(t *testing.T) {
	c := MakeCompound("f", MakeAtom("a"), MakeNumber(1))
	if c.String() != "f(a, 1)" {
		t.Errorf("got %q, want %q", c.String(), "f(a, 1)")
	}

	c2 := MakeCompound("f", MakeAtom("a"), MakeNumber(1))
	if !c.Equal(c2) {
		t.Error("same compounds should be equal")
	}

	c3 := MakeCompound("f", MakeAtom("a"), MakeNumber(2))
	if c.Equal(c3) {
		t.Error("different compounds should not be equal")
	}

	c4 := MakeCompound("g", MakeAtom("a"), MakeNumber(1))
	if c.Equal(c4) {
		t.Error("different functors should not be equal")
	}
}

func TestCompoundZeroArity(t *testing.T) {
	c := MakeCompound("foo")
	if c.String() != "foo" {
		t.Errorf("got %q, want %q", c.String(), "foo")
	}
}

func TestMakeList(t *testing.T) {
	list := MakeList(MakeAtom("a"), MakeAtom("b"), MakeAtom("c"))
	if list.String() != "[a, b, c]" {
		t.Errorf("got %q, want %q", list.String(), "[a, b, c]")
	}
}

func TestMakeListEmpty(t *testing.T) {
	list := MakeList()
	if list.String() != "[]" {
		t.Errorf("got %q, want %q", list.String(), "[]")
	}
}

func TestListWithTail(t *testing.T) {
	// [a|X]
	list := MakeCompound(".", MakeAtom("a"), MakeVar("X"))
	if list.String() != "[a|X]" {
		t.Errorf("got %q, want %q", list.String(), "[a|X]")
	}
}

func TestClauseString(t *testing.T) {
	// fact: parent(tom, bob).
	fact := &Clause{
		Head: MakeCompound("parent", MakeAtom("tom"), MakeAtom("bob")),
	}
	if fact.String() != "parent(tom, bob)." {
		t.Errorf("got %q, want %q", fact.String(), "parent(tom, bob).")
	}

	// rule: father(X, Y) :- parent(X, Y), male(X).
	rule := &Clause{
		Head: MakeCompound("father", MakeVar("X"), MakeVar("Y")),
		Body: []Term{
			MakeCompound("parent", MakeVar("X"), MakeVar("Y")),
			MakeCompound("male", MakeVar("X")),
		},
	}
	if rule.String() != "father(X, Y) :- parent(X, Y), male(X)." {
		t.Errorf("got %q, want %q", rule.String(), "father(X, Y) :- parent(X, Y), male(X).")
	}
}
