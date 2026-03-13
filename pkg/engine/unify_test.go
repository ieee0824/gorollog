package engine

import (
	"testing"

	"github.com/ieee0824/gorollog/pkg/types"
)

func TestUnifyAtoms(t *testing.T) {
	b := NewBinding()
	if !Unify(types.MakeAtom("a"), types.MakeAtom("a"), b) {
		t.Error("same atoms should unify")
	}
	if Unify(types.MakeAtom("a"), types.MakeAtom("b"), NewBinding()) {
		t.Error("different atoms should not unify")
	}
}

func TestUnifyNumbers(t *testing.T) {
	if !Unify(types.MakeNumber(42), types.MakeNumber(42), NewBinding()) {
		t.Error("same numbers should unify")
	}
	if Unify(types.MakeNumber(1), types.MakeNumber(2), NewBinding()) {
		t.Error("different numbers should not unify")
	}
}

func TestUnifyVariable(t *testing.T) {
	b := NewBinding()
	if !Unify(types.MakeVar("X"), types.MakeAtom("hello"), b) {
		t.Error("variable should unify with atom")
	}
	resolved := b.Resolve(types.MakeVar("X"))
	if !resolved.Equal(types.MakeAtom("hello")) {
		t.Errorf("X should resolve to hello, got %s", resolved.String())
	}
}

func TestUnifyTwoVariables(t *testing.T) {
	b := NewBinding()
	if !Unify(types.MakeVar("X"), types.MakeVar("Y"), b) {
		t.Error("two variables should unify")
	}
	// Bind one, the other should follow
	Unify(types.MakeVar("X"), types.MakeAtom("a"), b)
	resolved := b.Resolve(types.MakeVar("Y"))
	if !resolved.Equal(types.MakeAtom("a")) {
		t.Errorf("Y should resolve to a, got %s", resolved.String())
	}
}

func TestUnifyCompound(t *testing.T) {
	c1 := types.MakeCompound("f", types.MakeVar("X"), types.MakeAtom("b"))
	c2 := types.MakeCompound("f", types.MakeAtom("a"), types.MakeVar("Y"))
	b := NewBinding()
	if !Unify(c1, c2, b) {
		t.Error("compatible compounds should unify")
	}
	x := b.Resolve(types.MakeVar("X"))
	if !x.Equal(types.MakeAtom("a")) {
		t.Errorf("X should be a, got %s", x.String())
	}
	y := b.Resolve(types.MakeVar("Y"))
	if !y.Equal(types.MakeAtom("b")) {
		t.Errorf("Y should be b, got %s", y.String())
	}
}

func TestUnifyCompoundFail(t *testing.T) {
	c1 := types.MakeCompound("f", types.MakeAtom("a"))
	c2 := types.MakeCompound("g", types.MakeAtom("a"))
	if Unify(c1, c2, NewBinding()) {
		t.Error("different functors should not unify")
	}
}

func TestUnifyCompoundArityMismatch(t *testing.T) {
	c1 := types.MakeCompound("f", types.MakeAtom("a"))
	c2 := types.MakeCompound("f", types.MakeAtom("a"), types.MakeAtom("b"))
	if Unify(c1, c2, NewBinding()) {
		t.Error("different arities should not unify")
	}
}

func TestUnifyOccursCheck(t *testing.T) {
	// X = f(X) should fail with occurs check
	b := NewBinding()
	x := types.MakeVar("X")
	fx := types.MakeCompound("f", x)
	if Unify(x, fx, b) {
		t.Error("occurs check should prevent X = f(X)")
	}
}

func TestUnifyList(t *testing.T) {
	l1 := types.MakeList(types.MakeNumber(1), types.MakeNumber(2))
	l2 := types.MakeList(types.MakeVar("X"), types.MakeVar("Y"))
	b := NewBinding()
	if !Unify(l1, l2, b) {
		t.Error("lists should unify")
	}
	x := b.Resolve(types.MakeVar("X"))
	if !x.Equal(types.MakeNumber(1)) {
		t.Errorf("X should be 1, got %s", x.String())
	}
}

func TestUnifyAtomWithNumber(t *testing.T) {
	if Unify(types.MakeAtom("a"), types.MakeNumber(1), NewBinding()) {
		t.Error("atom and number should not unify")
	}
}

func TestBindingClone(t *testing.T) {
	b := NewBinding()
	b["X"] = types.MakeAtom("a")
	c := b.Clone()
	c["X"] = types.MakeAtom("b")
	if b.Resolve(types.MakeVar("X")).String() != "a" {
		t.Error("clone should not affect original")
	}
}

func TestResolveChain(t *testing.T) {
	b := NewBinding()
	b["X"] = types.MakeVar("Y")
	b["Y"] = types.MakeVar("Z")
	b["Z"] = types.MakeAtom("hello")
	resolved := b.Resolve(types.MakeVar("X"))
	if !resolved.Equal(types.MakeAtom("hello")) {
		t.Errorf("expected hello, got %s", resolved.String())
	}
}

func TestResolveCompound(t *testing.T) {
	b := NewBinding()
	b["X"] = types.MakeAtom("a")
	term := types.MakeCompound("f", types.MakeVar("X"), types.MakeNumber(1))
	resolved := b.Resolve(term)
	if resolved.String() != "f(a, 1)" {
		t.Errorf("expected f(a, 1), got %s", resolved.String())
	}
}
