package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenAtom        TokenType = iota // lowercase identifier or quoted atom
	TokenVariable                     // uppercase identifier or _
	TokenNumber                       // integer
	TokenFloat                        // floating point
	TokenLParen                       // (
	TokenRParen                       // )
	TokenLBracket                     // [
	TokenRBracket                     // ]
	TokenComma                        // ,
	TokenDot                          // . (end of clause)
	TokenBar                          // |
	TokenOperator                     // :- , + - * / etc.
	TokenCut                          // !
	TokenString                       // "..."
	TokenEOF                          // end of input
)

type Token struct {
	Type    TokenType
	Value   string
	Line    int
	Col     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q)", t.Type, t.Value)
}

type Lexer struct {
	input []rune
	pos   int
	line  int
	col   int
}

func New(input string) *Lexer {
	return &Lexer{
		input: []rune(input),
		pos:   0,
		line:  1,
		col:   1,
	}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '%' {
			// Line comment
			for l.pos < len(l.input) && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		if ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '*' {
			// Block comment
			l.advance() // /
			l.advance() // *
			for l.pos < len(l.input) {
				if l.peek() == '*' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '/' {
					l.advance() // *
					l.advance() // /
					break
				}
				l.advance()
			}
			continue
		}
		if unicode.IsSpace(ch) {
			l.advance()
			continue
		}
		break
	}
}

var operatorChars = "+-*/\\^<>=~:.?@#&"

func isOperatorChar(ch rune) bool {
	return strings.ContainsRune(operatorChars, ch)
}

func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			tokens = append(tokens, Token{Type: TokenEOF, Line: l.line, Col: l.col})
			return tokens, nil
		}

		line, col := l.line, l.col
		ch := l.peek()

		switch {
		case ch == '!':
			l.advance()
			tokens = append(tokens, Token{Type: TokenCut, Value: "!", Line: line, Col: col})

		case ch == '(':
			l.advance()
			tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Line: line, Col: col})

		case ch == ')':
			l.advance()
			tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Line: line, Col: col})

		case ch == '[':
			l.advance()
			tokens = append(tokens, Token{Type: TokenLBracket, Value: "[", Line: line, Col: col})

		case ch == ']':
			l.advance()
			tokens = append(tokens, Token{Type: TokenRBracket, Value: "]", Line: line, Col: col})

		case ch == ',':
			l.advance()
			tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: line, Col: col})

		case ch == '|':
			l.advance()
			tokens = append(tokens, Token{Type: TokenBar, Value: "|", Line: line, Col: col})

		case ch == ';':
			l.advance()
			tokens = append(tokens, Token{Type: TokenAtom, Value: ";", Line: line, Col: col})

		case ch == '\'':
			tok, err := l.readQuotedAtom()
			if err != nil {
				return nil, err
			}
			tok.Line = line
			tok.Col = col
			tokens = append(tokens, tok)

		case ch == '"':
			tok, err := l.readString()
			if err != nil {
				return nil, err
			}
			tok.Line = line
			tok.Col = col
			tokens = append(tokens, tok)

		case unicode.IsUpper(ch) || ch == '_':
			name := l.readIdentifier()
			if name == "_" {
				tokens = append(tokens, Token{Type: TokenVariable, Value: "_", Line: line, Col: col})
			} else {
				tokens = append(tokens, Token{Type: TokenVariable, Value: name, Line: line, Col: col})
			}

		case unicode.IsLower(ch):
			name := l.readIdentifier()
			// Check if next char is ( without space — then it's a functor
			tokens = append(tokens, Token{Type: TokenAtom, Value: name, Line: line, Col: col})

		case unicode.IsDigit(ch):
			tok := l.readNumber()
			tok.Line = line
			tok.Col = col
			tokens = append(tokens, tok)

		case ch == '-' && l.pos+1 < len(l.input) && unicode.IsDigit(l.input[l.pos+1]):
			// Negative number — but only if previous token suggests it's not subtraction
			if len(tokens) == 0 || tokens[len(tokens)-1].Type == TokenLParen ||
				tokens[len(tokens)-1].Type == TokenComma ||
				tokens[len(tokens)-1].Type == TokenOperator ||
				tokens[len(tokens)-1].Type == TokenLBracket {
				l.advance() // consume -
				tok := l.readNumber()
				tok.Value = "-" + tok.Value
				tok.Line = line
				tok.Col = col
				tokens = append(tokens, tok)
			} else {
				tok := l.readOperator()
				tok.Line = line
				tok.Col = col
				tokens = append(tokens, tok)
			}

		case isOperatorChar(ch):
			tok := l.readOperator()
			tok.Line = line
			tok.Col = col
			// Special case: standalone dot followed by whitespace or EOF = end of clause
			if tok.Value == "." {
				if l.pos >= len(l.input) || unicode.IsSpace(l.peek()) || l.peek() == '%' {
					tok.Type = TokenDot
				} else {
					tok.Type = TokenOperator
				}
			}
			tokens = append(tokens, tok)

		default:
			return nil, fmt.Errorf("unexpected character %q at line %d, col %d", ch, line, col)
		}
	}
}

func (l *Lexer) readIdentifier() string {
	var buf strings.Builder
	for l.pos < len(l.input) {
		ch := l.peek()
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			buf.WriteRune(l.advance())
		} else {
			break
		}
	}
	return buf.String()
}

func (l *Lexer) readNumber() Token {
	var buf strings.Builder
	isFloat := false
	for l.pos < len(l.input) {
		ch := l.peek()
		if unicode.IsDigit(ch) {
			buf.WriteRune(l.advance())
		} else if ch == '.' && l.pos+1 < len(l.input) && unicode.IsDigit(l.input[l.pos+1]) {
			isFloat = true
			buf.WriteRune(l.advance()) // .
		} else {
			break
		}
	}
	if isFloat {
		return Token{Type: TokenFloat, Value: buf.String()}
	}
	return Token{Type: TokenNumber, Value: buf.String()}
}

func (l *Lexer) readQuotedAtom() (Token, error) {
	l.advance() // opening '
	var buf strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if ch == '\'' {
			if l.peek() == '\'' {
				buf.WriteRune(l.advance()) // escaped '
			} else {
				return Token{Type: TokenAtom, Value: buf.String()}, nil
			}
		} else {
			buf.WriteRune(ch)
		}
	}
	return Token{}, fmt.Errorf("unterminated quoted atom at line %d", l.line)
}

func (l *Lexer) readString() (Token, error) {
	l.advance() // opening "
	var buf strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if ch == '"' {
			return Token{Type: TokenString, Value: buf.String()}, nil
		}
		if ch == '\\' && l.pos < len(l.input) {
			next := l.advance()
			switch next {
			case 'n':
				buf.WriteRune('\n')
			case 't':
				buf.WriteRune('\t')
			case '\\':
				buf.WriteRune('\\')
			case '"':
				buf.WriteRune('"')
			default:
				buf.WriteRune('\\')
				buf.WriteRune(next)
			}
		} else {
			buf.WriteRune(ch)
		}
	}
	return Token{}, fmt.Errorf("unterminated string at line %d", l.line)
}

func (l *Lexer) readOperator() Token {
	var buf strings.Builder
	for l.pos < len(l.input) && isOperatorChar(l.peek()) {
		buf.WriteRune(l.advance())
	}
	return Token{Type: TokenOperator, Value: buf.String()}
}
