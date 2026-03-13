package lexer

import "testing"

func tokenTypes(tokens []Token) []TokenType {
	tt := make([]TokenType, len(tokens))
	for i, t := range tokens {
		tt[i] = t.Type
	}
	return tt
}

func tokenValues(tokens []Token) []string {
	vals := make([]string, len(tokens))
	for i, t := range tokens {
		vals[i] = t.Value
	}
	return vals
}

func TestTokenizeAtom(t *testing.T) {
	tokens, err := New("hello").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 { // atom + EOF
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Type != TokenAtom || tokens[0].Value != "hello" {
		t.Errorf("expected atom 'hello', got %v", tokens[0])
	}
}

func TestTokenizeNumber(t *testing.T) {
	tokens, err := New("42").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenNumber || tokens[0].Value != "42" {
		t.Errorf("expected number 42, got %v", tokens[0])
	}
}

func TestTokenizeFloat(t *testing.T) {
	tokens, err := New("3.14").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenFloat || tokens[0].Value != "3.14" {
		t.Errorf("expected float 3.14, got %v", tokens[0])
	}
}

func TestTokenizeVariable(t *testing.T) {
	tokens, err := New("X _Y _").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenVariable || tokens[0].Value != "X" {
		t.Errorf("expected variable X, got %v", tokens[0])
	}
	if tokens[1].Type != TokenVariable || tokens[1].Value != "_Y" {
		t.Errorf("expected variable _Y, got %v", tokens[1])
	}
	if tokens[2].Type != TokenVariable || tokens[2].Value != "_" {
		t.Errorf("expected anonymous variable _, got %v", tokens[2])
	}
}

func TestTokenizeFact(t *testing.T) {
	tokens, err := New("parent(tom, bob).").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	expected := []TokenType{TokenAtom, TokenLParen, TokenAtom, TokenComma, TokenAtom, TokenRParen, TokenDot, TokenEOF}
	types := tokenTypes(tokens)
	if len(types) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(types), tokenValues(tokens))
	}
	for i := range expected {
		if types[i] != expected[i] {
			t.Errorf("token[%d]: expected type %d, got %d (%q)", i, expected[i], types[i], tokens[i].Value)
		}
	}
}

func TestTokenizeRule(t *testing.T) {
	tokens, err := New("father(X, Y) :- parent(X, Y).").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	// father ( X , Y ) :- parent ( X , Y ) . EOF
	if len(tokens) != 15 { // 13 + dot + EOF
		t.Fatalf("expected 15 tokens, got %d", len(tokens))
	}
	if tokens[6].Type != TokenOperator || tokens[6].Value != ":-" {
		t.Errorf("expected operator :-, got %v", tokens[6])
	}
}

func TestTokenizeList(t *testing.T) {
	tokens, err := New("[H|T]").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	expected := []TokenType{TokenLBracket, TokenVariable, TokenBar, TokenVariable, TokenRBracket, TokenEOF}
	types := tokenTypes(tokens)
	for i := range expected {
		if types[i] != expected[i] {
			t.Errorf("token[%d]: expected %d, got %d (%q)", i, expected[i], types[i], tokens[i].Value)
		}
	}
}

func TestTokenizeCut(t *testing.T) {
	tokens, err := New("!").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenCut {
		t.Errorf("expected cut, got %v", tokens[0])
	}
}

func TestTokenizeQuotedAtom(t *testing.T) {
	tokens, err := New("'hello world'").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenAtom || tokens[0].Value != "hello world" {
		t.Errorf("expected quoted atom 'hello world', got %v", tokens[0])
	}
}

func TestTokenizeQuotedAtomEscape(t *testing.T) {
	tokens, err := New("'it''s'").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Value != "it's" {
		t.Errorf("expected %q, got %q", "it's", tokens[0].Value)
	}
}

func TestTokenizeString(t *testing.T) {
	tokens, err := New(`"hello\n"`).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenString || tokens[0].Value != "hello\n" {
		t.Errorf("expected string with newline, got %v", tokens[0])
	}
}

func TestTokenizeNegativeNumber(t *testing.T) {
	tokens, err := New("f(-3)").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	// f ( -3 )
	if tokens[2].Type != TokenNumber || tokens[2].Value != "-3" {
		t.Errorf("expected negative number -3, got %v", tokens[2])
	}
}

func TestTokenizeSubtraction(t *testing.T) {
	tokens, err := New("X - 3").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	// X - 3
	if tokens[1].Type != TokenOperator || tokens[1].Value != "-" {
		t.Errorf("expected operator -, got %v", tokens[1])
	}
}

func TestTokenizeLineComment(t *testing.T) {
	tokens, err := New("foo. % comment\nbar.").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	vals := tokenValues(tokens)
	if vals[0] != "foo" || vals[2] != "bar" {
		t.Errorf("expected foo and bar, got %v", vals)
	}
}

func TestTokenizeBlockComment(t *testing.T) {
	tokens, err := New("foo /* comment */ bar").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Value != "foo" || tokens[1].Value != "bar" {
		t.Errorf("expected foo bar, got %v %v", tokens[0], tokens[1])
	}
}

func TestTokenizeOperators(t *testing.T) {
	tokens, err := New("X =:= Y").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[1].Type != TokenOperator || tokens[1].Value != "=:=" {
		t.Errorf("expected operator =:=, got %v", tokens[1])
	}
}

func TestTokenizeSemicolon(t *testing.T) {
	tokens, err := New(";").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenAtom || tokens[0].Value != ";" {
		t.Errorf("expected atom ;, got %v", tokens[0])
	}
}

func TestTokenizeDotInOperator(t *testing.T) {
	tokens, err := New("X =.. L.").Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	if tokens[1].Type != TokenOperator || tokens[1].Value != "=.." {
		t.Errorf("expected operator =.., got %v", tokens[1])
	}
}
