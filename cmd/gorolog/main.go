package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ieee0824/gorollog/pkg/engine"
	"github.com/ieee0824/gorollog/pkg/lexer"
	"github.com/ieee0824/gorollog/pkg/parser"
	"github.com/ieee0824/gorollog/pkg/types"
)

func main() {
	if len(os.Args) > 1 {
		// Load file
		for _, filename := range os.Args[1:] {
			if err := loadFile(filename, eng); err != nil {
				fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", filename, err)
				os.Exit(1)
			}
		}
	}

	repl(eng)
}

var eng = engine.New()

func loadFile(filename string, e *engine.Engine) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return loadSource(string(data), e)
}

func loadSource(source string, e *engine.Engine) error {
	lex := lexer.New(source)
	tokens, err := lex.Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}

	p := parser.New(tokens)
	clauses, err := p.ParseProgram()
	if err != nil {
		return fmt.Errorf("parser error: %w", err)
	}

	for _, c := range clauses {
		e.AddClause(c)
	}
	return nil
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func repl(e *engine.Engine) {
	scanner := bufio.NewScanner(os.Stdin)
	interactive := isInteractive()

	if interactive {
		fmt.Println("GoroLog - Prolog Interpreter in Go")
		fmt.Println("Enter queries at the ?- prompt.")
		fmt.Println("Load files: [filename].  Add facts: assert(fact).")
		fmt.Println("Commands: halt. / quit.")
		fmt.Println()
	}

	for {
		if interactive {
			fmt.Print("?- ")
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Multi-line input: keep reading until we see a '.'
		for !strings.HasSuffix(line, ".") {
			if interactive {
				fmt.Print("   ")
			}
			if !scanner.Scan() {
				break
			}
			line += " " + strings.TrimSpace(scanner.Text())
		}

		// Special commands
		if line == "halt." || line == "quit." {
			fmt.Println("Bye!")
			return
		}

		// Load file: [filename].
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "].") {
			filename := line[1 : len(line)-2]
			filename = strings.Trim(filename, "'\"")
			if !strings.HasSuffix(filename, ".pl") && !strings.HasSuffix(filename, ".pro") {
				filename += ".pl"
			}
			if err := loadFile(filename, e); err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("true.")
			}
			continue
		}

		// Determine if input is a clause (fact/rule) or a query
		if isClauseInput(line) {
			if err := loadSource(line, e); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			continue
		}

		// Parse as query
		lex := lexer.New(line)
		tokens, err := lex.Tokenize()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		p := parser.New(tokens)
		goals, err := p.ParseQuery()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Collect user-visible variables
		vars := engine.CollectVars(goals)

		found := false
		binding := engine.NewBinding()

		e.Solve(goals, binding, func(b engine.Binding) bool {
			found = true
			if len(vars) == 0 {
				fmt.Println("true.")
				return true
			}
			// Print variable bindings
			for i, v := range vars {
				resolved := b.Resolve(types.MakeVar(v))
				fmt.Printf("%s = %s", v, resolved.String())
				if i < len(vars)-1 {
					fmt.Println(",")
				}
			}

			if !interactive {
				// Non-interactive: print all solutions
				fmt.Println(" ;")
				return false
			}

			// Interactive: ask for more solutions
			fmt.Print(" ")
			reader := bufio.NewReader(os.Stdin)
			ch, _, err := reader.ReadRune()
			if err != nil || ch == '\n' || ch == '.' {
				fmt.Println()
				return true // stop
			}
			if ch == ';' || ch == ' ' || ch == 'n' {
				fmt.Println()
				return false // continue
			}
			fmt.Println()
			return true
		})

		if !found {
			fmt.Println("false.")
		}
	}
}

func isClauseInput(line string) bool {
	lex := lexer.New(line)
	tokens, err := lex.Tokenize()
	if err != nil || len(tokens) < 2 {
		return false
	}

	// If it contains :- at top level (not inside parens/brackets), it's a rule
	depth := 0
	for _, t := range tokens {
		switch t.Type {
		case lexer.TokenLParen, lexer.TokenLBracket:
			depth++
		case lexer.TokenRParen, lexer.TokenRBracket:
			depth--
		case lexer.TokenOperator:
			if t.Value == ":-" && depth == 0 {
				return true
			}
		}
	}

	// If the input contains any variables, it's a query
	for _, t := range tokens {
		if t.Type == lexer.TokenVariable {
			return false
		}
	}

	// Ground term starting with a lowercase atom (not a builtin) → fact
	if tokens[0].Type == lexer.TokenAtom {
		builtins := map[string]bool{
			"write": true, "writeln": true, "nl": true,
			"halt": true, "fail": true, "true": true, "false": true,
			"assert": true, "assertz": true, "asserta": true, "retract": true,
			"functor": true, "arg": true, "copy_term": true,
			"findall": true, "bagof": true, "setof": true,
			"atom": true, "number": true, "integer": true, "float": true,
			"var": true, "nonvar": true, "compound": true,
			"is_list": true, "append": true, "member": true, "length": true,
			"sort": true, "msort": true, "last": true, "reverse": true,
			"between": true, "succ": true, "plus": true,
			"atom_chars": true, "atom_length": true, "atom_concat": true,
			"number_chars": true, "char_code": true,
			"format": true, "listing": true, "tab": true,
			"call": true, "not": true, "maplist": true, "ground": true,
			"nth0": true, "nth1": true,
		}
		if builtins[tokens[0].Value] {
			return false
		}
		if tokens[1].Type == lexer.TokenLParen || tokens[1].Type == lexer.TokenDot {
			return true
		}
	}
	return false
}
