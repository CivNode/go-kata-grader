package dsl

import (
	"fmt"
	"strings"
	"unicode"
)

// Node is the parsed AST-shape. Every DSL expression produces a Node tree.
type Node struct {
	// Kind is the shape kind. One of: "call", "literal", "assign", "ident",
	// "wildcard".
	Kind string
	// Name is the string argument for call (qualified name, e.g.
	// "context.WithTimeout") and ident. Empty for other kinds.
	Name string
	// Args holds positional arguments: the args=[...] list for call and
	// literal's positional args.
	Args []*Node
	// LHS and RHS hold the two sides of an assign.
	LHS []*Node
	// RHS is the right-hand side list of an assign.
	RHS []*Node
	// Unordered is set when the DSL marks this node's argument list as
	// order-insensitive (currently only meaningful for call).
	Unordered bool
	// StringLit holds a literal string argument when this node represents a
	// string literal (inside literal(...)).
	StringLit string
	// HasString is true when StringLit is set.
	HasString bool
}

// Parse turns a DSL string into a Node tree.
func Parse(src string) (*Node, error) {
	p := &parser{src: src}
	p.advance()
	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tkEOF {
		return nil, p.errf("unexpected trailing token %q", p.tok.text)
	}
	return node, nil
}

// --- tokenizer -------------------------------------------------------------

type tokKind int

const (
	tkEOF tokKind = iota
	tkIdent
	tkString
	tkNumber
	tkLParen
	tkRParen
	tkLBrack
	tkRBrack
	tkComma
	tkEquals
	tkUnderscore
)

type token struct {
	kind tokKind
	text string
	pos  int
}

type parser struct {
	src string
	pos int
	tok token
}

func (p *parser) errf(format string, args ...any) error {
	return fmt.Errorf("dsl: at pos %d: %s", p.tok.pos, fmt.Sprintf(format, args...))
}

func (p *parser) advance() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
	if p.pos >= len(p.src) {
		p.tok = token{kind: tkEOF, pos: p.pos}
		return
	}
	start := p.pos
	c := p.src[p.pos]
	switch {
	case c == '(':
		p.pos++
		p.tok = token{kind: tkLParen, text: "(", pos: start}
	case c == ')':
		p.pos++
		p.tok = token{kind: tkRParen, text: ")", pos: start}
	case c == '[':
		p.pos++
		p.tok = token{kind: tkLBrack, text: "[", pos: start}
	case c == ']':
		p.pos++
		p.tok = token{kind: tkRBrack, text: "]", pos: start}
	case c == ',':
		p.pos++
		p.tok = token{kind: tkComma, text: ",", pos: start}
	case c == '=':
		p.pos++
		p.tok = token{kind: tkEquals, text: "=", pos: start}
	case c == '_':
		// Treat _ as identifier only when it stands alone; otherwise it is
		// part of an identifier (e.g. my_var). For our DSL _ is always the
		// wildcard, never part of a bigger name, so we handle it explicitly.
		if p.pos+1 < len(p.src) && isIdentPart(p.src[p.pos+1]) {
			p.readIdent()
		} else {
			p.pos++
			p.tok = token{kind: tkUnderscore, text: "_", pos: start}
		}
	case c == '"':
		p.readString()
	case isIdentStart(c):
		p.readIdent()
	case isDigit(c):
		p.readNumber()
	default:
		// Unknown character — record as ident to produce a clean error later.
		p.pos++
		p.tok = token{kind: tkIdent, text: string(c), pos: start}
	}
}

func (p *parser) readIdent() {
	start := p.pos
	for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
		p.pos++
	}
	p.tok = token{kind: tkIdent, text: p.src[start:p.pos], pos: start}
}

func (p *parser) readNumber() {
	start := p.pos
	for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		p.pos++
	}
	p.tok = token{kind: tkNumber, text: p.src[start:p.pos], pos: start}
}

func (p *parser) readString() {
	start := p.pos
	p.pos++ // opening quote
	var sb strings.Builder
	for p.pos < len(p.src) && p.src[p.pos] != '"' {
		if p.src[p.pos] == '\\' && p.pos+1 < len(p.src) {
			switch p.src[p.pos+1] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte(p.src[p.pos+1])
			}
			p.pos += 2
			continue
		}
		sb.WriteByte(p.src[p.pos])
		p.pos++
	}
	if p.pos < len(p.src) {
		p.pos++ // closing quote
	}
	p.tok = token{kind: tkString, text: sb.String(), pos: start}
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '.'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// --- parser ----------------------------------------------------------------

func (p *parser) expect(kind tokKind, what string) error {
	if p.tok.kind != kind {
		return p.errf("expected %s, got %q", what, p.tok.text)
	}
	p.advance()
	return nil
}

func (p *parser) parseExpr() (*Node, error) {
	switch p.tok.kind {
	case tkUnderscore:
		p.advance()
		return &Node{Kind: "wildcard"}, nil
	case tkIdent:
		name := p.tok.text
		switch name {
		case "call":
			return p.parseCall()
		case "literal":
			return p.parseLiteral()
		case "assign":
			return p.parseAssign()
		case "ident":
			return p.parseIdent()
		case "named":
			// "named" matches any identifier that is not the blank _.
			p.advance()
			return &Node{Kind: "named"}, nil
		default:
			return nil, p.errf("unknown shape %q", name)
		}
	case tkString:
		s := p.tok.text
		p.advance()
		return &Node{Kind: "literal", StringLit: s, HasString: true}, nil
	case tkNumber:
		n := p.tok.text
		p.advance()
		return &Node{Kind: "literal", StringLit: n, HasString: true}, nil
	default:
		return nil, p.errf("unexpected token %q", p.tok.text)
	}
}

func (p *parser) parseCall() (*Node, error) {
	// Already on "call"
	p.advance()
	if err := p.expect(tkLParen, "("); err != nil {
		return nil, err
	}
	if p.tok.kind != tkString {
		return nil, p.errf(`call requires a quoted function name, got %q`, p.tok.text)
	}
	name := p.tok.text
	p.advance()
	node := &Node{Kind: "call", Name: name}
	argsSeen := false
	for p.tok.kind == tkComma {
		p.advance()
		if p.tok.kind == tkIdent && p.tok.text == "args" {
			p.advance()
			if err := p.expect(tkEquals, "="); err != nil {
				return nil, err
			}
			args, err := p.parseList()
			if err != nil {
				return nil, err
			}
			node.Args = args
			argsSeen = true
			continue
		}
		if p.tok.kind == tkIdent && p.tok.text == "unordered" {
			p.advance()
			node.Unordered = true
			continue
		}
		// Shorthand:
		//   call("x", _)            -- match callee, don't constrain args.
		//   call("x", <otherExpr>)  -- positional args list.
		if argsSeen {
			return nil, p.errf("mixed positional args after args=[]: %q", p.tok.text)
		}
		if p.tok.kind == tkUnderscore {
			p.advance()
			// Args stays nil: "don't constrain args."
			continue
		}
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		node.Args = append(node.Args, arg)
	}
	if err := p.expect(tkRParen, ")"); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *parser) parseLiteral() (*Node, error) {
	// Already on "literal"
	p.advance()
	if err := p.expect(tkLParen, "("); err != nil {
		return nil, err
	}
	node := &Node{Kind: "literal"}
	if p.tok.kind != tkRParen {
		for {
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			node.Args = append(node.Args, arg)
			if p.tok.kind != tkComma {
				break
			}
			p.advance()
		}
	}
	if err := p.expect(tkRParen, ")"); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *parser) parseAssign() (*Node, error) {
	// Already on "assign"
	p.advance()
	if err := p.expect(tkLParen, "("); err != nil {
		return nil, err
	}
	node := &Node{Kind: "assign"}
	first := true
	for p.tok.kind != tkRParen {
		if !first {
			if err := p.expect(tkComma, ","); err != nil {
				return nil, err
			}
		}
		first = false
		if p.tok.kind != tkIdent {
			return nil, p.errf("assign option must be an identifier, got %q", p.tok.text)
		}
		opt := p.tok.text
		p.advance()
		if err := p.expect(tkEquals, "="); err != nil {
			return nil, err
		}
		list, err := p.parseList()
		if err != nil {
			return nil, err
		}
		switch opt {
		case "lhs":
			node.LHS = list
		case "rhs":
			node.RHS = list
		default:
			return nil, p.errf("unknown assign option %q", opt)
		}
	}
	if err := p.expect(tkRParen, ")"); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *parser) parseIdent() (*Node, error) {
	// Already on "ident"
	p.advance()
	if err := p.expect(tkLParen, "("); err != nil {
		return nil, err
	}
	if p.tok.kind != tkString {
		return nil, p.errf("ident requires a quoted name, got %q", p.tok.text)
	}
	name := p.tok.text
	p.advance()
	if err := p.expect(tkRParen, ")"); err != nil {
		return nil, err
	}
	return &Node{Kind: "ident", Name: name}, nil
}

func (p *parser) parseList() ([]*Node, error) {
	if err := p.expect(tkLBrack, "["); err != nil {
		return nil, err
	}
	var out []*Node
	if p.tok.kind == tkRBrack {
		p.advance()
		return out, nil
	}
	for {
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		out = append(out, n)
		if p.tok.kind != tkComma {
			break
		}
		p.advance()
	}
	if err := p.expect(tkRBrack, "]"); err != nil {
		return nil, err
	}
	return out, nil
}
