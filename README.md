# GoroLog

Go で実装された Prolog インタプリタ。CLI ツールとしても、Go ライブラリとしても利用可能。

## インストール

```bash
go install github.com/ieee0824/gorollog/cmd/gorolog@latest
```

## CLI として使う

### REPL

```bash
gorolog
```

```prolog
?- assert(parent(tom, bob)).
true.
?- assert(parent(tom, liz)).
true.
?- parent(tom, X).
X = bob ;
X = liz .
```

### ファイルのロード

```bash
gorolog examples/family.pl
```

REPL 内からもロード可能:

```prolog
?- [examples/family].
true.
?- father(tom, X).
X = bob ;
X = liz .
```

### パイプ入力

```bash
echo "[examples/math].
factorial(10, X).
halt." | gorolog
```

## ライブラリとして使う

```go
package main

import (
	"fmt"

	"github.com/ieee0824/gorollog/pkg/engine"
	"github.com/ieee0824/gorollog/pkg/lexer"
	"github.com/ieee0824/gorollog/pkg/parser"
	"github.com/ieee0824/gorollog/pkg/types"
)

func main() {
	// エンジン作成
	e := engine.New()

	// Prolog ソースをパースしてロード
	source := `
		parent(tom, bob).
		parent(tom, liz).
		parent(bob, ann).
		father(X, Y) :- parent(X, Y).
	`
	lex := lexer.New(source)
	tokens, _ := lex.Tokenize()
	p := parser.New(tokens)
	clauses, _ := p.ParseProgram()
	for _, c := range clauses {
		e.AddClause(c)
	}

	// クエリ実行
	goal := types.MakeCompound("father", types.MakeAtom("tom"), types.MakeVar("X"))
	e.Solve([]types.Term{goal}, engine.NewBinding(), func(b engine.Binding) bool {
		x := b.Resolve(types.MakeVar("X"))
		fmt.Println(x) // bob, liz
		return false    // false = 次の解を探す, true = 停止
	})
}
```

### io.Reader から Prolog を読み込む

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/ieee0824/gorollog/pkg/engine"
	"github.com/ieee0824/gorollog/pkg/lexer"
	"github.com/ieee0824/gorollog/pkg/parser"
	"github.com/ieee0824/gorollog/pkg/types"
)

// LoadFromReader は io.Reader から Prolog ソースを読み込みエンジンにロードする
func LoadFromReader(r io.Reader, e *engine.Engine) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	lex := lexer.New(string(data))
	tokens, err := lex.Tokenize()
	if err != nil {
		return err
	}
	p := parser.New(tokens)
	clauses, err := p.ParseProgram()
	if err != nil {
		return err
	}
	for _, c := range clauses {
		e.AddClause(c)
	}
	return nil
}

func main() {
	e := engine.New()

	// bytes.Buffer (io.Reader) からロード
	program := bytes.NewBufferString(`
		parent(tom, bob).
		parent(tom, liz).
		parent(bob, ann).
		ancestor(X, Y) :- parent(X, Y).
		ancestor(X, Y) :- parent(X, Z), ancestor(Z, Y).
	`)
	if err := LoadFromReader(program, e); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// os.Open で .pl ファイルからもロード可能
	// f, _ := os.Open("examples/family.pl")
	// defer f.Close()
	// LoadFromReader(f, e)

	// クエリ実行
	goal := types.MakeCompound("ancestor", types.MakeAtom("tom"), types.MakeVar("X"))
	e.Solve([]types.Term{goal}, engine.NewBinding(), func(b engine.Binding) bool {
		x := b.Resolve(types.MakeVar("X"))
		fmt.Println(x)
		return false
	})
	// Output:
	// bob
	// liz
	// ann
}
```

### パッケージ構成

| パッケージ | 役割 |
|---|---|
| `pkg/types` | 項の型定義 (`Atom`, `Number`, `Float`, `Variable`, `Compound`, `Clause`) |
| `pkg/lexer` | 字句解析器 |
| `pkg/parser` | 構文解析器 (演算子優先順位付き) |
| `pkg/engine` | 単一化 + 推論エンジン (SLD resolution, バックトラッキング) |

## 対応機能

### 基本

- ファクトとルール
- バックトラッキング
- 単一化 (occurs check 付き)
- カット (`!`)

### 制御構造

``,`` (conjunction), ``;`` (disjunction), ``->`` (if-then-else), ``\+`` (否定)

### 算術

``is``, ``+``, ``-``, ``*``, ``/``, ``//``, ``mod``, ``**``,
``abs``, ``max``, ``min``, ``sqrt``, ``sin``, ``cos``, ``log``, ``exp``,
``truncate``, ``round``, ``ceiling``, ``floor``

### 比較

``=``, ``\=``, ``==``, ``\==``, ``=:=``, ``=\=``, ``<``, ``>``, ``=<``, ``>=``

### リスト

``[H|T]`` 構文, ``append``, ``member``, ``length``, ``reverse``,
``sort``, ``msort``, ``last``, ``nth0``, ``nth1``, ``findall``

### 項操作

``functor``, ``arg``, ``copy_term``, ``=..`` (univ)

### 型検査

``atom``, ``number``, ``integer``, ``float``, ``var``, ``nonvar``, ``compound``, ``is_list``, ``ground``

### 入出力

``write``, ``writeln``, ``nl``, ``format``, ``tab``

### 高階

``call``, ``maplist``, ``between``, ``succ``, ``plus``

### データベース操作

``assert``/``assertz``, ``asserta``, ``retract``, ``listing``

## サンプル

`examples/family.pl` — 家族関係 (parent, ancestor, sibling)

```prolog
?- [examples/family].
?- ancestor(tom, X).
X = bob ;
X = liz ;
X = ann ;
X = pat ;
X = jim .
```

`examples/math.pl` — 再帰的な数学関数 (factorial, fibonacci, sum_list, max_list)

```prolog
?- [examples/math].
?- factorial(10, X).
X = 3628800 .
?- fib(10, X).
X = 55 .
```
