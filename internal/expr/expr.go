// Package expr evaluates the small arithmetic expressions the calculator app
// accepts: numbers, + - * / and parentheses.
package expr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ErrEmpty is returned when there is nothing to evaluate.
var ErrEmpty = errors.New("expr: empty expression")

// Eval parses and evaluates an arithmetic expression.
func Eval(input string) (float64, error) {
	tokens, err := lex(input)
	if err != nil {
		return 0, err
	}
	if len(tokens) == 0 {
		return 0, ErrEmpty
	}

	p := &parser{tokens: tokens}
	value, err := p.expression()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.tokens) {
		return 0, fmt.Errorf("expr: unexpected %q", string(p.tokens[p.pos].kind))
	}
	return value, nil
}

type token struct {
	kind  byte // 'n' number, or the operator itself
	value float64
}

func lex(input string) ([]token, error) {
	var tokens []token

	for i := 0; i < len(input); {
		c := rune(input[i])
		switch {
		case unicode.IsSpace(c):
			i++
		case strings.ContainsRune("+-*/()", c):
			tokens = append(tokens, token{kind: byte(c)})
			i++
		case c >= '0' && c <= '9' || c == '.':
			start := i
			for i < len(input) && (input[i] >= '0' && input[i] <= '9' || input[i] == '.') {
				i++
			}
			value, err := strconv.ParseFloat(input[start:i], 64)
			if err != nil {
				return nil, fmt.Errorf("expr: bad number %q", input[start:i])
			}
			tokens = append(tokens, token{kind: 'n', value: value})
		default:
			return nil, fmt.Errorf("expr: unexpected character %q", c)
		}
	}

	return tokens, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) expression() (float64, error) {
	value, err := p.term()
	if err != nil {
		return 0, err
	}

	for p.match('+') || p.match('-') {
		op := p.tokens[p.pos-1].kind
		right, err := p.term()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			value += right
		} else {
			value -= right
		}
	}
	return value, nil
}

func (p *parser) term() (float64, error) {
	value, err := p.factor()
	if err != nil {
		return 0, err
	}

	for p.match('*') || p.match('/') {
		op := p.tokens[p.pos-1].kind
		right, err := p.factor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			value *= right
		} else {
			if right == 0 {
				return 0, errors.New("expr: division by zero")
			}
			value /= right
		}
	}
	return value, nil
}

func (p *parser) factor() (float64, error) {
	if p.match('-') {
		value, err := p.factor()
		return -value, err
	}
	if p.match('+') {
		return p.factor()
	}
	if p.match('(') {
		value, err := p.expression()
		if err != nil {
			return 0, err
		}
		if !p.match(')') {
			return 0, errors.New("expr: missing closing parenthesis")
		}
		return value, nil
	}
	return p.number()
}

func (p *parser) number() (float64, error) {
	if p.pos >= len(p.tokens) || p.tokens[p.pos].kind != 'n' {
		return 0, errors.New("expr: expected a number")
	}
	value := p.tokens[p.pos].value
	p.pos++
	return value, nil
}

func (p *parser) match(kind byte) bool {
	if p.pos < len(p.tokens) && p.tokens[p.pos].kind == kind {
		p.pos++
		return true
	}
	return false
}
