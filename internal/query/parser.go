package query

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/localsearch/cli/pkg/types"
)

type Parser struct {
	input []rune
	pos   int
}

func NewParser(input string) *Parser {
	return &Parser{input: []rune(input), pos: 0}
}

func (p *Parser) Parse() (types.QueryNode, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return nil, p.parseError("expected expression")
	}

	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if p.pos != len(p.input) {
		return nil, p.parseError("unexpected trailing token %q", p.input[p.pos])
	}
	return node, nil
}

func (p *Parser) parseOr() (types.QueryNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.matchOrOperator() {
			right, err := p.parseAnd()
			if err != nil {
				return nil, err
			}
			if oq, ok := left.(*types.OrQuery); ok {
				oq.Children = append(oq.Children, right)
			} else {
				left = &types.OrQuery{Children: []types.QueryNode{left, right}}
			}
		} else {
			break
		}
	}
	return left, nil
}

func (p *Parser) parseAnd() (types.QueryNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.matchAndOperator() {
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = combineAnd(left, right)
			continue
		}
		if p.pos >= len(p.input) || p.input[p.pos] == ')' || p.atOrOperator() {
			break
		}

		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = combineAnd(left, right)
	}
	return left, nil
}

func combineAnd(left, right types.QueryNode) types.QueryNode {
	if aq, ok := left.(*types.AndQuery); ok {
		aq.Children = append(aq.Children, right)
		return aq
	}
	return &types.AndQuery{Children: []types.QueryNode{left, right}}
}

func (p *Parser) parseNot() (types.QueryNode, error) {
	p.skipWhitespace()
	if p.matchKeyword("NOT") || p.matchKeyword("not") || p.matchString("-") || p.matchString("!") {
		child, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &types.NotQuery{Child: child}, nil
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (types.QueryNode, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return nil, p.parseError("expected expression")
	}
	if p.input[p.pos] == ')' {
		return nil, p.parseError("expected expression before ')'")
	}
	if p.atBinaryOperator() {
		return nil, p.parseError("expected expression, found operator")
	}

	if p.matchString("(") {
		openingPos := p.pos - 1
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if !p.matchString(")") {
			return nil, p.parseError("expected ')' to close '(' at position %d", openingPos)
		}
		return node, nil
	}
	if p.input[p.pos] == '"' {
		return p.parsePhrase()
	}
	return p.parseTerm()
}

func (p *Parser) parsePhrase() (types.QueryNode, error) {
	openingPos := p.pos
	p.pos++
	var sb strings.Builder
	for p.pos < len(p.input) && p.input[p.pos] != '"' {
		sb.WriteRune(p.input[p.pos])
		p.pos++
	}
	if p.pos >= len(p.input) {
		return nil, p.parseError("unterminated phrase starting at position %d", openingPos)
	}
	p.pos++
	return &types.PhraseQuery{Terms: splitPhrase(sb.String())}, nil
}

func splitPhrase(s string) []string {
	var terms []string
	var current strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) || (unicode.IsPunct(r) && r != '_' && r != '-') {
			if current.Len() > 0 {
				terms = append(terms, strings.ToLower(current.String()))
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		terms = append(terms, strings.ToLower(current.String()))
	}
	return terms
}

func (p *Parser) parseTerm() (types.QueryNode, error) {
	var sb strings.Builder
	for p.pos < len(p.input) {
		r := p.input[p.pos]
		if unicode.IsSpace(r) || r == '(' || r == ')' || r == '"' {
			break
		}
		sb.WriteRune(r)
		p.pos++
	}
	term := strings.ToLower(sb.String())
	if term == "" {
		return nil, p.parseError("expected term")
	}
	if strings.HasSuffix(term, "*") {
		return &types.PrefixQuery{Prefix: term[:len(term)-1]}, nil
	}
	return &types.TermQuery{Term: term}, nil
}

func (p *Parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func (p *Parser) matchAndOperator() bool {
	return p.matchKeyword("AND") || p.matchKeyword("and") || p.matchString("&&")
}

func (p *Parser) matchOrOperator() bool {
	return p.matchKeyword("OR") || p.matchKeyword("or") || p.matchString("|")
}

func (p *Parser) atBinaryOperator() bool {
	return p.lookAheadKeyword("AND") || p.lookAheadKeyword("and") ||
		p.lookAheadKeyword("OR") || p.lookAheadKeyword("or") ||
		p.lookAheadString("&&") || p.lookAheadString("|")
}

func (p *Parser) atOrOperator() bool {
	return p.lookAheadKeyword("OR") || p.lookAheadKeyword("or") || p.lookAheadString("|")
}

func (p *Parser) matchString(s string) bool {
	if !p.lookAheadString(s) {
		return false
	}
	p.pos += len([]rune(s))
	return true
}

func (p *Parser) lookAheadString(s string) bool {
	runes := []rune(s)
	if p.pos+len(runes) > len(p.input) {
		return false
	}
	for i, r := range runes {
		if p.input[p.pos+i] != r {
			return false
		}
	}
	return true
}

func (p *Parser) matchKeyword(s string) bool {
	if !p.lookAheadKeyword(s) {
		return false
	}
	p.pos += len([]rune(s))
	return true
}

func (p *Parser) lookAheadKeyword(s string) bool {
	runes := []rune(s)
	if p.pos+len(runes) > len(p.input) {
		return false
	}
	for i, r := range runes {
		if p.input[p.pos+i] != r {
			return false
		}
	}
	nextPos := p.pos + len(runes)
	if nextPos >= len(p.input) {
		return true
	}
	next := p.input[nextPos]
	return !(unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_')
}

func (p *Parser) parseError(format string, args ...any) error {
	return fmt.Errorf("parse error at position %d: %s", p.pos, fmt.Sprintf(format, args...))
}
