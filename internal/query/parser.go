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

// ParseError describes a parse failure together with the 0-based rune position
// at which it occurred, so callers can report location context to users.
type ParseError struct {
	Position int
	Message  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}

func (p *Parser) errorAt(pos int, format string, args ...any) error {
	return &ParseError{Position: pos, Message: fmt.Sprintf(format, args...)}
}

func NewParser(input string) *Parser {
	return &Parser{input: []rune(input), pos: 0}
}

func (p *Parser) Parse() (types.QueryNode, error) {
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	// A well-formed query consumes the entire input. Any leftover token (an
	// unmatched ')', trailing garbage, etc.) is a syntax error.
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, p.errorAt(p.pos, "unexpected token %q", string(p.input[p.pos]))
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
		if p.matchKeyword("OR") || p.matchKeyword("or") || p.matchString("|") {
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
			// An explicit AND must be followed by an operand; parseNot errors
			// if none is present, so trailing AND is rejected.
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = combineAnd(left, right)
			continue
		}
		// Stop the implicit-AND chain at the boundaries owned by an enclosing
		// scope (end of input, closing paren, or an OR that belongs to the
		// caller). Probing these first means parseNot is only reached when an
		// operand genuinely follows.
		if p.pos >= len(p.input) {
			break
		}
		ch := p.input[p.pos]
		if ch == ')' {
			break
		}
		if (ch == 'O' || ch == 'o') && (p.lookAheadKeyword("OR") || p.lookAheadKeyword("or")) {
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
		// NOT is a prefix operator and must be followed by an operand. parseNot
		// never returns a nil node without an error, so a lone or trailing NOT
		// surfaces here as an "expected a term" error rather than building a
		// NotQuery with a nil child.
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
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		openPos := p.pos
		p.pos++ // consume '('
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return nil, p.errorAt(openPos, "unterminated group (missing closing ')')")
		}
		p.pos++ // consume ')'
		// parseOr never returns a nil node without an error, so an empty group
		// is already rejected above (parseTerm reports "expected a term").
		return node, nil
	}
	if p.pos < len(p.input) && p.input[p.pos] == '"' {
		return p.parsePhrase()
	}
	return p.parseTerm()
}

func (p *Parser) parsePhrase() (types.QueryNode, error) {
	startPos := p.pos
	p.pos++ // consume opening '"'
	var sb strings.Builder
	for p.pos < len(p.input) && p.input[p.pos] != '"' {
		sb.WriteRune(p.input[p.pos])
		p.pos++
	}
	if p.pos >= len(p.input) {
		return nil, p.errorAt(startPos, "unterminated phrase (missing closing quote)")
	}
	p.pos++ // consume closing '"'
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
	startPos := p.pos
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
		// Reaching here means a term was expected (an operand after AND/OR/NOT
		// or the contents of a group) but none followed. Reporting an error
		// rather than a nil node is what lets malformed queries fail loudly.
		return nil, p.errorAt(startPos, "expected a term")
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
