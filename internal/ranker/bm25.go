package ranker

import (
	"math"
	"sort"
	"strings"

	"github.com/localsearch/cli/internal/query"
	"github.com/localsearch/cli/pkg/types"
)

type BM25Ranker struct {
	K1 float64
	B  float64
}

type IndexReader interface {
	GetPostings(term string) (types.PostingList, bool)
	DocumentCount() int
	AverageDocLength() float64
	GetDocument(docID types.DocID) (*types.Document, bool)
}

func NewBM25() *BM25Ranker {
	return &BM25Ranker{
		K1: 1.2,
		B:  0.75,
	}
}

func (r *BM25Ranker) Score(idx IndexReader, matched []*query.MatchedDoc) []types.SearchResult {
	totalDocs := idx.DocumentCount()
	avgDL := idx.AverageDocLength()
	if avgDL == 0 {
		avgDL = 1
	}

	results := make([]types.SearchResult, 0, len(matched))

	for _, md := range matched {
		doc, ok := idx.GetDocument(md.DocID)
		if !ok {
			continue
		}
		docLen := float64(doc.Length)
		if docLen == 0 {
			docLen = 1
		}

		var score float64
		uniqueTerms := make(map[string]bool)
		for term := range md.MatchedTerms {
			uniqueTerms[term] = true
		}

		for term := range uniqueTerms {
			pl, ok := idx.GetPostings(term)
			if !ok {
				continue
			}
			docFreq := len(pl)
			idf := math.Log(1 + (float64(totalDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5))

			var termFreq int
			for _, p := range pl {
				if p.DocID == md.DocID {
					termFreq = p.TermFreq
					break
				}
			}
			if termFreq == 0 {
				continue
			}

			tf := float64(termFreq)
			numerator := tf * (r.K1 + 1)
			denominator := tf + r.K1*(1-r.B+r.B*(docLen/avgDL))
			score += idf * (numerator / denominator)
		}

		results = append(results, types.SearchResult{
			Doc:   doc,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func BuildFragments(content string, matchedTerms map[string][]int, contextSize int) []types.Fragment {
	if len(matchedTerms) == 0 {
		return nil
	}

	runes := []rune(content)
	lowContent := strings.ToLower(content)
	lowRunes := []rune(lowContent)

	type charRange struct {
		start int
		end   int
	}
	var allHits []charRange

	for term := range matchedTerms {
		if term == "_all" {
			continue
		}
		termRunes := []rune(strings.ToLower(term))
		if len(termRunes) == 0 {
			continue
		}
		positions := findAllOccurrences(lowRunes, termRunes)
		for _, start := range positions {
			allHits = append(allHits, charRange{start: start, end: start + len(termRunes)})
		}
	}

	if len(allHits) == 0 {
		return nil
	}

	sort.Slice(allHits, func(i, j int) bool {
		return allHits[i].start < allHits[j].start
	})

	var fragments []types.Fragment
	lastEnd := -1
	contextChars := contextSize * 6

	for _, hit := range allHits {
		charStart := hit.start - contextChars
		if charStart < 0 {
			charStart = 0
		}
		charEnd := hit.end + contextChars
		if charEnd > len(runes) {
			charEnd = len(runes)
		}

		if charStart <= lastEnd && len(fragments) > 0 {
			lastFrag := &fragments[len(fragments)-1]
			if charEnd > lastEnd {
				if charEnd > len(runes) {
					charEnd = len(runes)
				}
				extra := string(runes[lastEnd:charEnd])
				lastFrag.Text = lastFrag.Text + extra
				lastEnd = charEnd
			}
			continue
		}

		fragText := string(runes[charStart:charEnd])
		var highlights []types.Range
		for _, h := range allHits {
			if h.start >= charStart && h.end <= charEnd {
				highlights = append(highlights, types.Range{
					Start: h.start - charStart,
					End:   h.end - charStart,
				})
			}
		}

		fragments = append(fragments, types.Fragment{
			Text:      fragText,
			Highlight: highlights,
		})
		lastEnd = charEnd

		if len(fragments) >= 5 {
			break
		}
	}

	return fragments
}

func findAllOccurrences(text, pattern []rune) []int {
	var positions []int
	if len(pattern) == 0 {
		return positions
	}
	for i := 0; i <= len(text)-len(pattern)+1; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if text[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			positions = append(positions, i)
		}
	}
	return positions
}

type tokenPos struct {
	term  string
	start int
	end   int
}

func tokenizePositions(text string) []tokenPos {
	var tokens []tokenPos
	var current strings.Builder
	start := -1

	runes := []rune(text)
	i := 0

	for i < len(runes) {
		r := runes[i]

		if isTokenChar(r) {
			if current.Len() == 0 {
				start = i
			}
			current.WriteRune(r)
			i++
			continue
		}

		if current.Len() > 0 {
			tokens = append(tokens, tokenPos{
				term:  strings.ToLower(current.String()),
				start: start,
				end:   i,
			})
			current.Reset()
			start = -1
		}
		i++
	}

	if current.Len() > 0 {
		tokens = append(tokens, tokenPos{
			term:  strings.ToLower(current.String()),
			start: start,
			end:   len(runes),
		})
	}

	return tokens
}

func isTokenChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-' ||
		(r >= 0x4E00 && r <= 0x9FFF)
}
