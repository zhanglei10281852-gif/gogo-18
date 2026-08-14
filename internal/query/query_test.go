package query

import (
	"testing"

	"github.com/localsearch/cli/pkg/types"
)

func TestParseSingleTerm(t *testing.T) {
	p := NewParser("hello")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	tq, ok := node.(*types.TermQuery)
	if !ok {
		t.Fatalf("expected TermQuery, got %T", node)
	}
	if tq.Term != "hello" {
		t.Errorf("expected 'hello', got %q", tq.Term)
	}
}

func TestParsePrefix(t *testing.T) {
	p := NewParser("app*")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	pq, ok := node.(*types.PrefixQuery)
	if !ok {
		t.Fatalf("expected PrefixQuery, got %T", node)
	}
	if pq.Prefix != "app" {
		t.Errorf("expected prefix 'app', got %q", pq.Prefix)
	}
}

func TestParsePhrase(t *testing.T) {
	p := NewParser(`"quick brown fox"`)
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	pq, ok := node.(*types.PhraseQuery)
	if !ok {
		t.Fatalf("expected PhraseQuery, got %T", node)
	}
	expected := []string{"quick", "brown", "fox"}
	if len(pq.Terms) != len(expected) {
		t.Fatalf("expected %d terms, got %d", len(expected), len(pq.Terms))
	}
	for i, e := range expected {
		if pq.Terms[i] != e {
			t.Errorf("term %d: expected %q, got %q", i, e, pq.Terms[i])
		}
	}
}

func TestParseImplicitAnd(t *testing.T) {
	p := NewParser("hello world")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	aq, ok := node.(*types.AndQuery)
	if !ok {
		t.Fatalf("expected AndQuery (implicit), got %T", node)
	}
	if len(aq.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(aq.Children))
	}
}

func TestParseExplicitAnd(t *testing.T) {
	p := NewParser("hello AND world")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	aq, ok := node.(*types.AndQuery)
	if !ok {
		t.Fatalf("expected AndQuery, got %T", node)
	}
	if len(aq.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(aq.Children))
	}
}

func TestParseOr(t *testing.T) {
	p := NewParser("hello OR world")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	oq, ok := node.(*types.OrQuery)
	if !ok {
		t.Fatalf("expected OrQuery, got %T", node)
	}
	if len(oq.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(oq.Children))
	}
}

func TestParseComplex(t *testing.T) {
	p := NewParser("(hello OR hi) AND world")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	aq, ok := node.(*types.AndQuery)
	if !ok {
		t.Fatalf("expected AndQuery at top, got %T", node)
	}
	if len(aq.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(aq.Children))
	}
	if _, ok := aq.Children[0].(*types.OrQuery); !ok {
		t.Errorf("expected OrQuery as first child, got %T", aq.Children[0])
	}
	if _, ok := aq.Children[1].(*types.TermQuery); !ok {
		t.Errorf("expected TermQuery as second child, got %T", aq.Children[1])
	}
}

func TestParseLowerCase(t *testing.T) {
	p := NewParser("Hello World")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	aq, ok := node.(*types.AndQuery)
	if !ok {
		t.Fatalf("expected AndQuery, got %T", node)
	}
	tq1 := aq.Children[0].(*types.TermQuery)
	tq2 := aq.Children[1].(*types.TermQuery)
	if tq1.Term != "hello" {
		t.Errorf("expected lowercase 'hello', got %q", tq1.Term)
	}
	if tq2.Term != "world" {
		t.Errorf("expected lowercase 'world', got %q", tq2.Term)
	}
}

type mockIndex struct {
	postings map[string]types.PostingList
}

func (m *mockIndex) GetPostings(term string) (types.PostingList, bool) {
	pl, ok := m.postings[term]
	return pl, ok
}

func (m *mockIndex) GetTermsByPrefix(prefix string) []string {
	var terms []string
	for t := range m.postings {
		if len(t) >= len(prefix) && t[:len(prefix)] == prefix {
			terms = append(terms, t)
		}
	}
	return terms
}

func (m *mockIndex) AllDocIDs() []types.DocID {
	seen := make(map[types.DocID]bool)
	for _, pl := range m.postings {
		for _, p := range pl {
			seen[p.DocID] = true
		}
	}
	ids := make([]types.DocID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func buildTestIndex() *mockIndex {
	return &mockIndex{
		postings: map[string]types.PostingList{
			"hello": {
				{DocID: 1, Positions: []int{0}, TermFreq: 1},
				{DocID: 2, Positions: []int{0}, TermFreq: 1},
			},
			"world": {
				{DocID: 1, Positions: []int{1}, TermFreq: 1},
				{DocID: 3, Positions: []int{0}, TermFreq: 1},
			},
			"foo": {
				{DocID: 2, Positions: []int{1}, TermFreq: 1},
				{DocID: 3, Positions: []int{1}, TermFreq: 1},
			},
			"apple": {
				{DocID: 4, Positions: []int{0}, TermFreq: 1},
			},
			"application": {
				{DocID: 4, Positions: []int{1}, TermFreq: 1},
			},
			"quick": {
				{DocID: 5, Positions: []int{0}, TermFreq: 1},
			},
			"brown": {
				{DocID: 5, Positions: []int{1}, TermFreq: 1},
			},
			"fox": {
				{DocID: 5, Positions: []int{2}, TermFreq: 1},
				{DocID: 6, Positions: []int{5}, TermFreq: 1},
			},
			"lazy": {
				{DocID: 6, Positions: []int{0}, TermFreq: 1},
			},
			"dog": {
				{DocID: 6, Positions: []int{1}, TermFreq: 1},
			},
		},
	}
}

func TestEvalTerm(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)
	results := ev.Evaluate(&types.TermQuery{Term: "hello"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestEvalAnd(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)
	q := &types.AndQuery{
		Children: []types.QueryNode{
			&types.TermQuery{Term: "hello"},
			&types.TermQuery{Term: "world"},
		},
	}
	results := ev.Evaluate(q)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (doc 1), got %d", len(results))
	}
	if results[0].DocID != 1 {
		t.Errorf("expected docID 1, got %d", results[0].DocID)
	}
}

func TestEvalOr(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)
	q := &types.OrQuery{
		Children: []types.QueryNode{
			&types.TermQuery{Term: "hello"},
			&types.TermQuery{Term: "world"},
		},
	}
	results := ev.Evaluate(q)
	if len(results) != 3 {
		t.Fatalf("expected 3 results (docs 1,2,3), got %d", len(results))
	}
}

func TestEvalPrefix(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)
	q := &types.PrefixQuery{Prefix: "app"}
	results := ev.Evaluate(q)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for app*, got %d", len(results))
	}
}

func TestEvalPhraseConsecutive(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)

	q := &types.PhraseQuery{Terms: []string{"quick", "brown", "fox"}}
	results := ev.Evaluate(q)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for phrase 'quick brown fox', got %d", len(results))
	}
	if results[0].DocID != 5 {
		t.Errorf("expected docID 5, got %d", results[0].DocID)
	}
}

func TestEvalPhraseNonConsecutive(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)

	q := &types.PhraseQuery{Terms: []string{"lazy", "fox"}}
	results := ev.Evaluate(q)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for non-consecutive phrase, got %d", len(results))
	}
}

func TestEvalPhraseMissingTerm(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)

	q := &types.PhraseQuery{Terms: []string{"hello", "nonexistent"}}
	results := ev.Evaluate(q)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for phrase with missing term, got %d", len(results))
	}
}

func TestParseNot(t *testing.T) {
	p := NewParser("NOT rust")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	nq, ok := node.(*types.NotQuery)
	if !ok {
		t.Fatalf("expected NotQuery, got %T", node)
	}
	tq, ok := nq.Child.(*types.TermQuery)
	if !ok {
		t.Fatalf("expected TermQuery as child, got %T", nq.Child)
	}
	if tq.Term != "rust" {
		t.Errorf("expected 'rust', got %q", tq.Term)
	}
}

func TestParseNotWithDash(t *testing.T) {
	p := NewParser("hello -world")
	node, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	aq, ok := node.(*types.AndQuery)
	if !ok {
		t.Fatalf("expected AndQuery, got %T", node)
	}
	if len(aq.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(aq.Children))
	}
	if _, ok := aq.Children[0].(*types.TermQuery); !ok {
		t.Errorf("first child should be TermQuery")
	}
	if _, ok := aq.Children[1].(*types.NotQuery); !ok {
		t.Errorf("second child should be NotQuery, got %T", aq.Children[1])
	}
}

func TestEvalNotAlone(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)

	totalDocs := len(idx.AllDocIDs())

	q := &types.NotQuery{Child: &types.TermQuery{Term: "rust"}}
	results := ev.Evaluate(q)
	if results == nil {
		t.Fatalf("expected results for NOT rust")
	}

	hasRust := false
	for _, md := range results {
		if md.DocID == 7 {
			hasRust = true
		}
	}
	_ = hasRust
	_ = totalDocs
}

func TestEvalAndWithNotMissingTerm(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)

	q := &types.AndQuery{
		Children: []types.QueryNode{
			&types.TermQuery{Term: "hello"},
			&types.NotQuery{Child: &types.TermQuery{Term: "xyzzy"}},
		},
	}
	results := ev.Evaluate(q)
	if results == nil {
		t.Fatal("NOT xyzzy (non-existent) should not exclude any docs")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (hello matches), got %d", len(results))
	}
}

func TestEvalAndWithNotExisting(t *testing.T) {
	idx := buildTestIndex()
	ev := NewEvaluator(idx)

	idx.postings["rust"] = types.PostingList{
		{DocID: 7, Positions: []int{0}, TermFreq: 1},
	}

	q := &types.AndQuery{
		Children: []types.QueryNode{
			&types.TermQuery{Term: "hello"},
			&types.NotQuery{Child: &types.TermQuery{Term: "rust"}},
		},
	}
	results := ev.Evaluate(q)
	if results == nil {
		t.Fatal("expected results")
	}
	hasDoc7 := false
	for _, md := range results {
		if md.DocID == 7 {
			hasDoc7 = true
		}
	}
	if hasDoc7 {
		t.Error("doc 7 (with rust) should be excluded by NOT rust")
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (hello docs minus none of them have rust), got %d", len(results))
	}
}
