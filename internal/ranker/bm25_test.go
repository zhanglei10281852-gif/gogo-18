package ranker

import (
	"testing"

	"github.com/localsearch/cli/internal/query"
	"github.com/localsearch/cli/pkg/types"
)

type mockIndex struct {
	docs     map[types.DocID]*types.Document
	postings map[string]types.PostingList
	avgDL    float64
}

func (m *mockIndex) GetPostings(term string) (types.PostingList, bool) {
	pl, ok := m.postings[term]
	return pl, ok
}

func (m *mockIndex) DocumentCount() int {
	return len(m.docs)
}

func (m *mockIndex) AverageDocLength() float64 {
	return m.avgDL
}

func (m *mockIndex) GetDocument(docID types.DocID) (*types.Document, bool) {
	d, ok := m.docs[docID]
	return d, ok
}

func buildRankingTestIndex() *mockIndex {
	return &mockIndex{
		docs: map[types.DocID]*types.Document{
			1: {ID: 1, Path: "/doc1", Length: 100},
			2: {ID: 2, Path: "/doc2", Length: 20},
			3: {ID: 3, Path: "/doc3", Length: 50},
		},
		postings: map[string]types.PostingList{
			"python": {
				{DocID: 1, Positions: []int{0}, TermFreq: 1},
				{DocID: 2, Positions: []int{0, 1, 2, 3, 4}, TermFreq: 5},
				{DocID: 3, Positions: []int{0, 1}, TermFreq: 2},
			},
			"machine": {
				{DocID: 1, Positions: []int{1}, TermFreq: 1},
				{DocID: 3, Positions: []int{3}, TermFreq: 1},
			},
			"learning": {
				{DocID: 1, Positions: []int{2}, TermFreq: 1},
				{DocID: 3, Positions: []int{4}, TermFreq: 1},
			},
			"rust": {
				{DocID: 2, Positions: []int{10}, TermFreq: 1},
			},
		},
		avgDL: (100.0 + 20.0 + 50.0) / 3.0,
	}
}

func TestBM25SingleTermRanking(t *testing.T) {
	idx := buildRankingTestIndex()
	r := NewBM25()

	matched := []*query.MatchedDoc{
		{DocID: 1, MatchedTerms: map[string][]int{"python": {0}}},
		{DocID: 2, MatchedTerms: map[string][]int{"python": {0, 1, 2, 3, 4}}},
		{DocID: 3, MatchedTerms: map[string][]int{"python": {0, 1}}},
	}

	results := r.Score(idx, matched)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Doc.ID != 2 {
		t.Errorf("expected doc 2 (highest freq) to rank first, got doc %d", results[0].Doc.ID)
	}

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted descending: result[%d]=%.4f > result[%d]=%.4f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestBM25MultiTermScoring(t *testing.T) {
	idx := buildRankingTestIndex()
	r := NewBM25()

	matched := []*query.MatchedDoc{
		{DocID: 1, MatchedTerms: map[string][]int{
			"python":  {0},
			"machine": {1},
			"learning": {2},
		}},
		{DocID: 3, MatchedTerms: map[string][]int{
			"python":   {0, 1},
			"machine":  {3},
			"learning": {4},
		}},
	}

	results := r.Score(idx, matched)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Score <= 0 {
		t.Error("expected positive score")
	}

	if results[0].Score <= results[1].Score {
		t.Errorf("expected descending scores, got %.4f and %.4f", results[0].Score, results[1].Score)
	}
}

func TestBM25EmptyInput(t *testing.T) {
	idx := buildRankingTestIndex()
	r := NewBM25()

	results := r.Score(idx, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil input, got %d", len(results))
	}

	results = r.Score(idx, []*query.MatchedDoc{})
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestBM25ScoreProperties(t *testing.T) {
	idx := buildRankingTestIndex()
	r := NewBM25()

	matched := []*query.MatchedDoc{
		{DocID: 2, MatchedTerms: map[string][]int{"python": {0, 1, 2, 3, 4}}},
	}

	results := r.Score(idx, matched)
	if len(results) != 1 {
		t.Fatalf("expected 1 result")
	}

	if results[0].Score <= 0 {
		t.Errorf("expected positive BM25 score, got %.4f", results[0].Score)
	}
}

func TestBM25DocLengthNormalization(t *testing.T) {
	docs := map[types.DocID]*types.Document{
		1: {ID: 1, Path: "/short", Length: 10},
		2: {ID: 2, Path: "/long", Length: 200},
	}
	postings := map[string]types.PostingList{
		"term": {
			{DocID: 1, Positions: []int{0}, TermFreq: 3},
			{DocID: 2, Positions: []int{0, 1, 2}, TermFreq: 3},
		},
	}
	idx := &mockIndex{
		docs:     docs,
		postings: postings,
		avgDL:    105.0,
	}
	r := NewBM25()
	matched := []*query.MatchedDoc{
		{DocID: 1, MatchedTerms: map[string][]int{"term": {0, 1, 2}}},
		{DocID: 2, MatchedTerms: map[string][]int{"term": {0, 1, 2}}},
	}
	results := r.Score(idx, matched)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Doc.ID != 1 {
		t.Errorf("expected shorter doc (1) to rank higher with same TF, got doc %d", results[0].Doc.ID)
	}
}
