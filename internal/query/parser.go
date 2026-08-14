package query

import (
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
	return p.parseOr()
}

func (p *Parser) parseOr() (types.QueryNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.matchKeyword("OR") || p.matchKeyword("or") || p.matchKeyword("|") {
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
		explicitAnd := p.matchKeyword("AND") || p.matchKeyword("and") || p.matchKeyword("&&")
		if explicitAnd {
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = combineAnd(left, right)
			continue
		}
		if p.pos >= len(p.input) {
			break
		}
		ch := p.input[p.pos]
		if ch == ')' {
			break
		}
		if ch == 'O' || ch == 'o' {
			if p.lookAheadKeyword("OR") || p.lookAheadKeyword("or") {
				break
			}
		}
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		if right == nil {
			break
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
	if p.matchString("(") {
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		p.matchString(")")
		return node, nil
	}
	if p.pos < len(p.input) && p.input[p.pos] == '"' {
		return p.parsePhrase()
	}
	return p.parseTerm()
}

func (p *Parser) parsePhrase() (types.QueryNode, error) {
	p.pos++
	var sb strings.Builder
	for p.pos < len(p.input) && p.input[p.pos] != '"' {
		sb.WriteRune(p.input[p.pos])
		p.pos++
	}
	if p.pos < len(p.input) {
		p.pos++
	}
	phrase := sb.String()
	terms := splitPhrase(phrase)
	return &types.PhraseQuery{Terms: terms}, nil
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
		return nil, nil
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

func (p *Parser) matchString(s string) bool {
	runes := []rune(s)
	if p.pos+len(runes) > len(p.input) {
		return false
	}
	for i, r := range runes {
		if p.input[p.pos+i] != r {
			return false
		}
	}
	p.pos += len(runes)
	return true
}

func (p *Parser) matchKeyword(s string) bool {
	runes := []rune(s)
	savePos := p.pos
	if p.pos+len(runes) > len(p.input) {
		return false
	}
	for i, r := range runes {
		if p.input[p.pos+i] != r {
			return false
		}
	}
	nextPos := p.pos + len(runes)
	if nextPos < len(p.input) {
		next := p.input[nextPos]
		if unicode.IsLetter(next) || unicode.IsDigit(next) {
			return false
		}
	}
	p.pos = nextPos
	_ = savePos
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
	return !(unicode.IsLetter(next) || unicode.IsDigit(next))
}
