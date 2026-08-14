package tokenizer

import (
	"testing"
)

func TestEnglishTokenization(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("The quick brown fox jumps over the lazy dog.")

	terms := make([]string, len(tokens))
	for i, t := range tokens {
		terms[i] = t.Term
	}

	expected := []string{"quick", "brown", "fox", "jumps", "over", "lazy", "dog"}

	if len(terms) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(terms), terms)
	}
	for i, e := range expected {
		if terms[i] != e {
			t.Errorf("token %d: expected %q, got %q", i, e, terms[i])
		}
	}
}

func TestCaseNormalization(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("Hello WORLD Foo-Bar")

	terms := make([]string, len(tokens))
	for i, t := range tokens {
		terms[i] = t.Term
	}

	expected := []string{"hello", "world", "foo-bar"}
	if len(terms) != len(expected) {
		t.Fatalf("expected %d terms, got %d: %v", len(expected), len(terms), terms)
	}
	for i, e := range expected {
		if terms[i] != e {
			t.Errorf("token %d: expected %q, got %q", i, e, terms[i])
		}
	}
}

func TestFullWidthNormalization(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("Ｈｅｌｌｏ　Ｗｏｒｌｄ！")

	terms := make([]string, len(tokens))
	for i, t := range tokens {
		terms[i] = t.Term
	}

	expected := []string{"hello", "world"}
	if len(terms) != len(expected) {
		t.Fatalf("expected %d terms, got %d: %v", len(expected), len(terms), terms)
	}
	for i, e := range expected {
		if terms[i] != e {
			t.Errorf("token %d: expected %q, got %q", i, e, terms[i])
		}
	}
}

func TestChineseTokenization(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("机器学习")

	termSet := make(map[string]bool)
	for _, tok := range tokens {
		termSet[tok.Term] = true
	}

	singleChars := []string{"机", "器", "学", "习"}
	for _, c := range singleChars {
		if !termSet[c] {
			t.Errorf("expected single char token %q not found", c)
		}
	}

	bigrams := []string{"机器", "器学", "学习"}
	for _, bg := range bigrams {
		if !termSet[bg] {
			t.Errorf("expected bigram token %q not found", bg)
		}
	}
}

func TestChineseStopWords(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("机器学习的算法")

	for _, tok := range tokens {
		if tok.Term == "的" {
			t.Error("stop word '的' should be filtered out")
		}
	}
}

func TestMixedChineseEnglish(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("学习 Python 编程")

	hasPython := false
	hasXuexi := false
	for _, tok := range tokens {
		if tok.Term == "python" {
			hasPython = true
		}
		if tok.Term == "学习" {
			hasXuexi = true
		}
	}
	if !hasPython {
		t.Error("expected 'python' token")
	}
	if !hasXuexi {
		t.Error("expected '学习' bigram token")
	}
}

func TestCustomStopWords(t *testing.T) {
	tok := New()
	custom := map[string]struct{}{
		"quick": {},
		"brown": {},
	}
	tok.WithStopWords(custom)

	tokens := tok.Tokenize("The quick brown fox")
	for _, tok := range tokens {
		if tok.Term == "quick" || tok.Term == "brown" {
			t.Errorf("custom stop word %q should be filtered", tok.Term)
		}
	}
}

func TestTokenPositions(t *testing.T) {
	tok := New()
	tokens := tok.Tokenize("hello world foo bar")

	positions := make([]int, len(tokens))
	for i, tok := range tokens {
		positions[i] = tok.Position
	}

	for i, p := range positions {
		if p != i {
			t.Errorf("expected position %d, got %d", i, p)
		}
	}
}
