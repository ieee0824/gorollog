package parser

import (
	"fmt"
	"strconv"

	"github.com/ieee0824/gorollog/pkg/lexer"
	"github.com/ieee0824/gorollog/pkg/types"
)

type Parser struct {
	tokens  []lexer.Token
	pos     int
	varCount int
}

func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) expect(tt lexer.TokenType) (lexer.Token, error) {
	tok := p.advance()
	if tok.Type != tt {
		return tok, fmt.Errorf("expected token type %d, got %d (%q) at line %d, col %d",
			tt, tok.Type, tok.Value, tok.Line, tok.Col)
	}
	return tok, nil
}

// ParseProgram parses multiple clauses until EOF.
func (p *Parser) ParseProgram() ([]*types.Clause, error) {
	var clauses []*types.Clause
	for p.peek().Type != lexer.TokenEOF {
		clause, err := p.ParseClause()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}

// ParseClause parses a single clause (fact or rule) ending with '.'.
func (p *Parser) ParseClause() (*types.Clause, error) {
	head, err := p.parseDisjunction()
	if err != nil {
		return nil, fmt.Errorf("parsing clause head: %w", err)
	}

	headComp, ok := termToCompound(head)
	if !ok {
		return nil, fmt.Errorf("clause head must be a callable term, got %T", head)
	}

	var body []types.Term

	if p.peek().Type == lexer.TokenOperator && p.peek().Value == ":-" {
		p.advance() // consume :-
		body, err = p.parseBody()
		if err != nil {
			return nil, fmt.Errorf("parsing clause body: %w", err)
		}
	}

	if _, err := p.expect(lexer.TokenDot); err != nil {
		return nil, fmt.Errorf("expected '.' at end of clause: %w", err)
	}

	return &types.Clause{Head: headComp, Body: body}, nil
}

// ParseQuery parses a query (goal list) ending with '.'.
func (p *Parser) ParseQuery() ([]types.Term, error) {
	goals, err := p.parseBody()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenDot); err != nil {
		return nil, fmt.Errorf("expected '.' at end of query: %w", err)
	}
	return goals, nil
}

func (p *Parser) parseBody() ([]types.Term, error) {
	// Parse the body as a single term (conjunction is handled by parseConjunction)
	// then flatten commas into a goal list
	term, err := p.parseDisjunction()
	if err != nil {
		return nil, err
	}
	var goals []types.Term
	flattenConjunction(term, &goals)
	return goals, nil
}

func flattenConjunction(t types.Term, goals *[]types.Term) {
	if c, ok := t.(*types.Compound); ok && c.Functor == "," && len(c.Args) == 2 {
		flattenConjunction(c.Args[0], goals)
		flattenConjunction(c.Args[1], goals)
	} else {
		*goals = append(*goals, t)
	}
}

func (p *Parser) parseTerm() (types.Term, error) {
	left, err := p.parseDisjunction()
	if err != nil {
		return nil, err
	}
	// Handle :- as infix operator (for assert((head :- body)))
	if p.peek().Type == lexer.TokenOperator && p.peek().Value == ":-" {
		p.advance()
		right, err := p.parseDisjunction()
		if err != nil {
			return nil, err
		}
		return types.MakeCompound(":-", left, right), nil
	}
	return left, nil
}

// Precedence chain (higher number = lower binding = parsed first):
// ;  (1100) → -> (1050) → ,  (1000) → =/>/< (700) → +/- (500) → */÷ (400)

func (p *Parser) parseDisjunction() (types.Term, error) {
	left, err := p.parseIfThen()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == lexer.TokenAtom && p.peek().Value == ";" {
		p.advance()
		right, err := p.parseIfThen()
		if err != nil {
			return nil, err
		}
		left = types.MakeCompound(";", left, right)
	}
	return left, nil
}

func (p *Parser) parseIfThen() (types.Term, error) {
	left, err := p.parseConjunction()
	if err != nil {
		return nil, err
	}

	if p.peek().Type == lexer.TokenOperator && p.peek().Value == "->" {
		p.advance()
		right, err := p.parseConjunction()
		if err != nil {
			return nil, err
		}
		return types.MakeCompound("->", left, right), nil
	}

	return left, nil
}

func (p *Parser) parseConjunction() (types.Term, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == lexer.TokenComma {
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = types.MakeCompound(",", left, right)
	}
	return left, nil
}

func (p *Parser) parseComparison() (types.Term, error) {
	left, err := p.parseArith()
	if err != nil {
		return nil, err
	}

	tok := p.peek()
	if tok.Type == lexer.TokenOperator {
		switch tok.Value {
		case "=", "\\=", "==", "\\==", "=:=", "=\\=", "<", ">", "=<", ">=", "is", "=..":
			p.advance()
			right, err := p.parseArith()
			if err != nil {
				return nil, err
			}
			return types.MakeCompound(tok.Value, left, right), nil
		}
	}
	if tok.Type == lexer.TokenAtom && tok.Value == "is" {
		p.advance()
		right, err := p.parseArith()
		if err != nil {
			return nil, err
		}
		return types.MakeCompound("is", left, right), nil
	}

	return left, nil
}

func (p *Parser) parseArith() (types.Term, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		if tok.Type == lexer.TokenOperator && (tok.Value == "+" || tok.Value == "-") {
			p.advance()
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			left = types.MakeCompound(tok.Value, left, right)
		} else {
			break
		}
	}
	return left, nil
}

func (p *Parser) parseMulDiv() (types.Term, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		if tok.Type == lexer.TokenOperator && (tok.Value == "*" || tok.Value == "/" || tok.Value == "//" || tok.Value == "mod") {
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = types.MakeCompound(tok.Value, left, right)
		} else if tok.Type == lexer.TokenAtom && tok.Value == "mod" {
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = types.MakeCompound("mod", left, right)
		} else {
			break
		}
	}
	return left, nil
}

func (p *Parser) parseUnary() (types.Term, error) {
	tok := p.peek()
	if tok.Type == lexer.TokenOperator && tok.Value == "-" {
		p.advance()
		inner, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		if n, ok := inner.(*types.Number); ok {
			return types.MakeNumber(-n.Value), nil
		}
		return types.MakeCompound("-", inner), nil
	}
	if tok.Type == lexer.TokenOperator && tok.Value == "\\+" {
		p.advance()
		inner, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		return types.MakeCompound("\\+", inner), nil
	}
	return p.parsePower()
}

func (p *Parser) parsePower() (types.Term, error) {
	base, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	tok := p.peek()
	if tok.Type == lexer.TokenOperator && (tok.Value == "**" || tok.Value == "^") {
		p.advance()
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return types.MakeCompound(tok.Value, base, exp), nil
	}

	return base, nil
}

func (p *Parser) parsePrimary() (types.Term, error) {
	tok := p.peek()

	switch tok.Type {
	case lexer.TokenNumber:
		p.advance()
		val, err := strconv.ParseInt(tok.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", tok.Value, err)
		}
		return types.MakeNumber(val), nil

	case lexer.TokenFloat:
		p.advance()
		val, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", tok.Value, err)
		}
		return &types.Float{Value: val}, nil

	case lexer.TokenVariable:
		p.advance()
		if tok.Value == "_" {
			p.varCount++
			return types.MakeVar(fmt.Sprintf("_%d", p.varCount)), nil
		}
		return types.MakeVar(tok.Value), nil

	case lexer.TokenCut:
		p.advance()
		return types.MakeAtom("!"), nil

	case lexer.TokenAtom:
		p.advance()
		name := tok.Value
		// Check for functor: atom followed by (
		if p.peek().Type == lexer.TokenLParen {
			p.advance() // consume (
			if p.peek().Type == lexer.TokenRParen {
				p.advance() // consume )
				return types.MakeCompound(name), nil
			}
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenRParen); err != nil {
				return nil, err
			}
			return types.MakeCompound(name, args...), nil
		}
		// Check for special atoms that are actually operators used as atoms
		if name == "not" || name == "\\+" {
			inner, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			return types.MakeCompound("\\+", inner), nil
		}
		return types.MakeAtom(name), nil

	case lexer.TokenString:
		p.advance()
		// Convert string to a list of character codes
		chars := []rune(tok.Value)
		terms := make([]types.Term, len(chars))
		for i, ch := range chars {
			terms[i] = types.MakeNumber(int64(ch))
		}
		return types.MakeList(terms...), nil

	case lexer.TokenLParen:
		p.advance() // consume (
		inner, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
		return inner, nil

	case lexer.TokenLBracket:
		return p.parseList()

	case lexer.TokenOperator:
		// Handle operators used as atoms in certain contexts, e.g., op(+, X, Y)
		if tok.Value == ":-" {
			// Directive
			p.advance()
			body, err := p.parseBody()
			if err != nil {
				return nil, err
			}
			return types.MakeCompound(":-", body[0]), nil
		}
		return nil, fmt.Errorf("unexpected operator %q at line %d, col %d", tok.Value, tok.Line, tok.Col)

	default:
		return nil, fmt.Errorf("unexpected token %q (type %d) at line %d, col %d",
			tok.Value, tok.Type, tok.Line, tok.Col)
	}
}

func (p *Parser) parseArgList() ([]types.Term, error) {
	var args []types.Term
	for {
		// Arguments stop before comma (comma is a delimiter in arg lists, not conjunction)
		arg, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.peek().Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}
	return args, nil
}

func (p *Parser) parseList() (types.Term, error) {
	p.advance() // consume [
	if p.peek().Type == lexer.TokenRBracket {
		p.advance()
		return types.MakeAtom("[]"), nil
	}

	var elems []types.Term
	for {
		elem, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)

		if p.peek().Type == lexer.TokenBar {
			p.advance() // consume |
			tail, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenRBracket); err != nil {
				return nil, err
			}
			// Build list with explicit tail
			result := tail
			for i := len(elems) - 1; i >= 0; i-- {
				result = types.MakeCompound(".", elems[i], result)
			}
			return result, nil
		}

		if p.peek().Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}

	if _, err := p.expect(lexer.TokenRBracket); err != nil {
		return nil, err
	}
	return types.MakeList(elems...), nil
}

func termToCompound(t types.Term) (*types.Compound, bool) {
	switch v := t.(type) {
	case *types.Compound:
		return v, true
	case *types.Atom:
		return types.MakeCompound(v.Name), true
	default:
		return nil, false
	}
}
