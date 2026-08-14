package query

import (
	"strings"
	"testing"

	"github.com/localsearch/cli/pkg/types"
)

func TestParserParseRejectsMalformedQueries(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "unterminated phrase", input: `"alpha beta`},
		{name: "unterminated group", input: `alpha AND (beta OR gamma`},
		{name: "trailing AND", input: `alpha AND`},
		{name: "trailing OR", input: `alpha OR`},
		{name: "trailing NOT", input: `alpha NOT`},
		{name: "lone NOT", input: `NOT`},
		{name: "empty group", input: `()`},
		{name: "whitespace-only group", input: `(   )`},
		{name: "extra closing parenthesis", input: `alpha)`},
		{name: "garbage after query", input: `(alpha)) garbage`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("Parser.Parse panicked for %q: %v", tt.input, recovered)
				}
			}()

			_, err := NewParser(tt.input).Parse()
			if err == nil {
				t.Fatalf("Parser.Parse(%q) returned no error", tt.input)
			}
			if !strings.Contains(err.Error(), "position") {
				t.Fatalf("Parser.Parse(%q) error lacks position context: %v", tt.input, err)
			}
		})
	}
}

func TestParserParseValidSyntaxRegression(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(*testing.T, types.QueryNode)
	}{
		{
			name:  "nested boolean expression",
			input: `((alpha OR beta) AND NOT gamma)`,
			check: func(t *testing.T, node types.QueryNode) {
				and, ok := node.(*types.AndQuery)
				if !ok || len(and.Children) != 2 {
					t.Fatalf("expected two-child AndQuery, got %#v", node)
				}
				if _, ok := and.Children[0].(*types.OrQuery); !ok {
					t.Fatalf("expected nested OrQuery, got %T", and.Children[0])
				}
				if _, ok := and.Children[1].(*types.NotQuery); !ok {
					t.Fatalf("expected nested NotQuery, got %T", and.Children[1])
				}
			},
		},
		{
			name:  "implicit AND with phrase and prefix",
			input: `alpha "beta gamma" delta*`,
			check: func(t *testing.T, node types.QueryNode) {
				and, ok := node.(*types.AndQuery)
				if !ok || len(and.Children) != 3 {
					t.Fatalf("expected three-child implicit AndQuery, got %#v", node)
				}
				phrase, ok := and.Children[1].(*types.PhraseQuery)
				if !ok || len(phrase.Terms) != 2 || phrase.Terms[0] != "beta" || phrase.Terms[1] != "gamma" {
					t.Fatalf("unexpected phrase node: %#v", and.Children[1])
				}
				prefix, ok := and.Children[2].(*types.PrefixQuery)
				if !ok || prefix.Prefix != "delta" {
					t.Fatalf("unexpected prefix node: %#v", and.Children[2])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := NewParser(tt.input).Parse()
			if err != nil {
				t.Fatalf("Parser.Parse(%q) returned error: %v", tt.input, err)
			}
			tt.check(t, node)
		})
	}
}
