package autosearch

import (
	"fmt"
	"strings"
	"unicode"
)

// Filter expression AST node types

type filterNode interface {
	match(text string) bool
}

type termNode struct {
	term string
}

func (n *termNode) match(text string) bool {
	return strings.Contains(text, n.term)
}

type notNode struct {
	child filterNode
}

func (n *notNode) match(text string) bool {
	return !n.child.match(text)
}

type andNode struct {
	left, right filterNode
}

func (n *andNode) match(text string) bool {
	return n.left.match(text) && n.right.match(text)
}

type orNode struct {
	left, right filterNode
}

func (n *orNode) match(text string) bool {
	return n.left.match(text) || n.right.match(text)
}

// ParseFilterExpression parses a boolean filter expression into an AST.
// Supports: AND, OR, NOT (!), parentheses, bare terms.
// Bare terms without operators are combined with AND.
// All matching is case-insensitive.
func ParseFilterExpression(expr string) (filterNode, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}

	tokens, err := tokenize(expr)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens}
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("unexpected token: %s", p.tokens[p.pos])
	}

	return node, nil
}

// MatchFilter evaluates a filter expression against a text string.
func MatchFilter(node filterNode, text string) bool {
	if node == nil {
		return true
	}
	return node.match(strings.ToLower(text))
}

// Token types
const (
	tokTerm   = "TERM"
	tokAnd    = "AND"
	tokOr     = "OR"
	tokNot    = "NOT"
	tokLParen = "("
	tokRParen = ")"
)

type token struct {
	typ   string
	value string
}

func (t token) String() string {
	if t.typ == tokTerm {
		return fmt.Sprintf("%q", t.value)
	}
	return t.typ
}

func tokenize(expr string) ([]token, error) {
	var tokens []token
	i := 0
	runes := []rune(expr)

	for i < len(runes) {
		ch := runes[i]

		if unicode.IsSpace(ch) {
			i++
			continue
		}

		if ch == '(' {
			tokens = append(tokens, token{typ: tokLParen})
			i++
			continue
		}

		if ch == ')' {
			tokens = append(tokens, token{typ: tokRParen})
			i++
			continue
		}

		if ch == '!' {
			tokens = append(tokens, token{typ: tokNot})
			i++
			continue
		}

		// Quoted string: "multi word term"
		if ch == '"' {
			i++
			start := i
			for i < len(runes) && runes[i] != '"' {
				i++
			}
			if i >= len(runes) {
				return nil, fmt.Errorf("unclosed quote")
			}
			word := strings.ToLower(string(runes[start:i]))
			tokens = append(tokens, token{typ: tokTerm, value: word})
			i++ // skip closing quote
			continue
		}

		// Read a word
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) && runes[i] != '(' && runes[i] != ')' && runes[i] != '!' {
			i++
		}
		word := string(runes[start:i])
		upper := strings.ToUpper(word)

		switch upper {
		case "AND":
			tokens = append(tokens, token{typ: tokAnd})
		case "OR":
			tokens = append(tokens, token{typ: tokOr})
		case "NOT":
			tokens = append(tokens, token{typ: tokNot})
		default:
			tokens = append(tokens, token{typ: tokTerm, value: strings.ToLower(word)})
		}
	}

	return tokens, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() *token {
	if p.pos >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.pos]
}

func (p *parser) next() *token {
	t := p.peek()
	if t != nil {
		p.pos++
	}
	return t
}

// parseOr handles OR (lowest precedence)
func (p *parser) parseOr() (filterNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		if t == nil || t.typ != tokOr {
			break
		}
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &orNode{left: left, right: right}
	}

	return left, nil
}

// parseAnd handles AND and implicit AND (higher precedence than OR)
func (p *parser) parseAnd() (filterNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		if t == nil || t.typ == tokRParen || t.typ == tokOr {
			break
		}

		if t.typ == tokAnd {
			p.next()
		}

		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &andNode{left: left, right: right}
	}

	return left, nil
}

// parseUnary handles NOT/! (highest precedence)
func (p *parser) parseUnary() (filterNode, error) {
	t := p.peek()
	if t != nil && t.typ == tokNot {
		p.next()
		child, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &notNode{child: child}, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles terms and parenthesized expressions
func (p *parser) parsePrimary() (filterNode, error) {
	t := p.peek()
	if t == nil {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	if t.typ == tokLParen {
		p.next()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		closing := p.next()
		if closing == nil || closing.typ != tokRParen {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		return node, nil
	}

	if t.typ == tokTerm {
		p.next()
		return &termNode{term: t.value}, nil
	}

	return nil, fmt.Errorf("unexpected token: %s", t)
}
