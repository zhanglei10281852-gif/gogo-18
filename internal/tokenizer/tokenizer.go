package tokenizer

import (
	"strings"
	"unicode"

	"github.com/localsearch/cli/pkg/types"
)

var DefaultStopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "but": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"have": {}, "has": {}, "had": {}, "do": {}, "does": {}, "did": {},
	"will": {}, "would": {}, "should": {}, "could": {}, "may": {}, "might": {}, "can": {},
	"of": {}, "in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "with": {}, "by": {},
	"from": {}, "about": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"it": {}, "its": {}, "i": {}, "me": {}, "my": {}, "we": {}, "our": {},
	"you": {}, "your": {}, "he": {}, "him": {}, "his": {}, "she": {}, "her": {},
	"they": {}, "them": {}, "their": {}, "what": {}, "which": {}, "who": {},
	"not": {}, "no": {}, "nor": {}, "so": {}, "as": {}, "if": {}, "then": {},
	"than": {}, "too": {}, "very": {}, "s": {}, "t": {}, "just": {},
	"的": {}, "了": {}, "和": {}, "是": {}, "在": {}, "我": {}, "有": {}, "就": {},
	"不": {}, "人": {}, "都": {}, "一": {}, "一个": {}, "上": {}, "也": {}, "很": {},
	"到": {}, "说": {}, "要": {}, "去": {}, "你": {}, "会": {}, "着": {}, "没有": {},
	"看": {}, "好": {}, "自己": {}, "这": {}, "那": {}, "他": {}, "她": {}, "它": {},
	"们": {}, "个": {}, "为": {}, "但": {}, "而": {}, "与": {}, "或": {}, "及": {},
	"等": {}, "之": {}, "其": {}, "此": {}, "该": {}, "被": {}, "把": {}, "让": {},
	"对": {}, "从": {}, "向": {}, "于": {}, "以": {}, "将": {}, "给": {}, "由": {},
}

type Tokenizer struct {
	StopWords  map[string]struct{}
	ChineseBigram bool
}

func New() *Tokenizer {
	return &Tokenizer{
		StopWords:     DefaultStopWords,
		ChineseBigram: true,
	}
}

func (t *Tokenizer) WithStopWords(words map[string]struct{}) *Tokenizer {
	t.StopWords = words
	return t
}

func (t *Tokenizer) Tokenize(text string) []types.Token {
	normalized := normalize(text)
	var tokens []types.Token
	var current strings.Builder
	pos := 0

	runes := []rune(normalized)
	i := 0

	for i < len(runes) {
		r := runes[i]

		if isChinese(r) {
			if current.Len() > 0 {
				tokens = t.addToken(tokens, current.String(), pos)
				current.Reset()
			}
			tokens = t.addToken(tokens, string(r), pos)
			pos++
			if t.ChineseBigram && i+1 < len(runes) && isChinese(runes[i+1]) {
				tokens = t.addToken(tokens, string(runes[i:i+2]), pos)
				pos++
			}
			i++
			continue
		}

		if isTokenChar(r) {
			current.WriteRune(r)
			i++
			continue
		}

		if current.Len() > 0 {
			tokens = t.addToken(tokens, current.String(), pos)
			current.Reset()
			pos++
		}
		i++
	}

	if current.Len() > 0 {
		tokens = t.addToken(tokens, current.String(), pos)
	}

	return tokens
}

func (t *Tokenizer) TokenizeQuery(text string) []types.Token {
	return t.Tokenize(text)
}

func (t *Tokenizer) addToken(tokens []types.Token, term string, pos int) []types.Token {
	term = strings.ToLower(term)
	if term == "" {
		return tokens
	}
	if _, ok := t.StopWords[term]; ok {
		return tokens
	}
	return append(tokens, types.Token{Term: term, Position: pos})
}

func normalize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		b.WriteRune(normalizeRune(r))
	}
	return b.String()
}

func normalizeRune(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	if r == 0x3000 {
		return 0x20
	}
	return unicode.ToLower(r)
}

func isTokenChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func isChinese(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}
