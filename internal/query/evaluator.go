package query

import (
	"sort"

	"github.com/localsearch/cli/internal/tokenizer"
	"github.com/localsearch/cli/pkg/types"
)

type MatchedDoc struct {
	DocID        types.DocID
	MatchedTerms map[string][]int
}

type Evaluator struct {
	idx IndexReader
}

type IndexReader interface {
	GetPostings(term string) (types.PostingList, bool)
	GetTermsByPrefix(prefix string) []string
	AllDocIDs() []types.DocID
}

func NewEvaluator(idx IndexReader) *Evaluator {
	return &Evaluator{idx: idx}
}

func (e *Evaluator) Evaluate(node types.QueryNode) []*MatchedDoc {
	if node == nil {
		return nil
	}
	switch n := node.(type) {
	case *types.TermQuery:
		return e.evalTerm(n)
	case *types.PrefixQuery:
		return e.evalPrefix(n)
	case *types.PhraseQuery:
		return e.evalPhrase(n)
	case *types.AndQuery:
		return e.evalAnd(n)
	case *types.OrQuery:
		return e.evalOr(n)
	case *types.NotQuery:
		return e.evalNot(n)
	}
	return nil
}

func (e *Evaluator) evalTerm(q *types.TermQuery) []*MatchedDoc {
	if containsChinese(q.Term) {
		tok := tokenizer.New()
		tokens := tok.Tokenize(q.Term)
		if len(tokens) == 0 {
			return nil
		}
		allMatched := make([]*MatchedDoc, 0)
		seen := make(map[types.DocID]*MatchedDoc)
		for _, t := range tokens {
			pl, ok := e.idx.GetPostings(t.Term)
			if !ok {
				continue
			}
			for _, p := range pl {
				if existing, ok := seen[p.DocID]; ok {
					existing.MatchedTerms[t.Term] = append(existing.MatchedTerms[t.Term], p.Positions...)
				} else {
					positions := make([]int, len(p.Positions))
					copy(positions, p.Positions)
					md := &MatchedDoc{
						DocID:        p.DocID,
						MatchedTerms: map[string][]int{t.Term: positions},
					}
					seen[p.DocID] = md
					allMatched = append(allMatched, md)
				}
			}
		}
		return allMatched
	}
	pl, ok := e.idx.GetPostings(q.Term)
	if !ok {
		return nil
	}
	result := make([]*MatchedDoc, 0, len(pl))
	for _, p := range pl {
		positions := make([]int, len(p.Positions))
		copy(positions, p.Positions)
		result = append(result, &MatchedDoc{
			DocID:        p.DocID,
			MatchedTerms: map[string][]int{q.Term: positions},
		})
	}
	return result
}

func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func (e *Evaluator) evalPrefix(q *types.PrefixQuery) []*MatchedDoc {
	terms := e.idx.GetTermsByPrefix(q.Prefix)
	if len(terms) == 0 {
		return nil
	}
	children := make([]types.QueryNode, 0, len(terms))
	for _, t := range terms {
		children = append(children, &types.TermQuery{Term: t})
	}
	return e.evalOr(&types.OrQuery{Children: children})
}

func (e *Evaluator) evalPhrase(q *types.PhraseQuery) []*MatchedDoc {
	if len(q.Terms) == 0 {
		return nil
	}
	if len(q.Terms) == 1 {
		return e.evalTerm(&types.TermQuery{Term: q.Terms[0]})
	}

	var postingsList []types.PostingList
	for _, term := range q.Terms {
		pl, ok := e.idx.GetPostings(term)
		if !ok {
			return nil
		}
		postingsList = append(postingsList, pl)
	}

	commonDocIDs := intersectDocIDs(postingsList)
	if len(commonDocIDs) == 0 {
		return nil
	}

	var result []*MatchedDoc
	for _, docID := range commonDocIDs {
		termPositions := make([][]int, len(q.Terms))
		allEmpty := true
		for i, pl := range postingsList {
			for _, p := range pl {
				if p.DocID == docID {
					pos := make([]int, len(p.Positions))
					copy(pos, p.Positions)
					termPositions[i] = pos
					allEmpty = false
					break
				}
			}
		}
		if allEmpty {
			continue
		}
		if !checkPhraseConsecutive(termPositions) {
			continue
		}
		matched := make(map[string][]int)
		for i, term := range q.Terms {
			matched[term] = termPositions[i]
		}
		result = append(result, &MatchedDoc{
			DocID:        docID,
			MatchedTerms: matched,
		})
	}
	return result
}

func intersectDocIDs(lists []types.PostingList) []types.DocID {
	if len(lists) == 0 {
		return nil
	}
	docSet := make(map[types.DocID]bool)
	for _, p := range lists[0] {
		docSet[p.DocID] = true
	}
	for _, pl := range lists[1:] {
		nextSet := make(map[types.DocID]bool)
		for _, p := range pl {
			if docSet[p.DocID] {
				nextSet[p.DocID] = true
			}
		}
		docSet = nextSet
		if len(docSet) == 0 {
			return nil
		}
	}
	result := make([]types.DocID, 0, len(docSet))
	for id := range docSet {
		result = append(result, id)
	}
	return result
}

func checkPhraseConsecutive(termPositions [][]int) bool {
	if len(termPositions) < 2 {
		return true
	}
	for _, firstPos := range termPositions[0] {
		if findConsecutive(firstPos, termPositions, 1) {
			return true
		}
	}
	return false
}

func findConsecutive(basePos int, termPositions [][]int, termIdx int) bool {
	if termIdx >= len(termPositions) {
		return true
	}
	expected := basePos + termIdx
	positions := termPositions[termIdx]
	idx := sort.SearchInts(positions, expected)
	if idx < len(positions) && positions[idx] == expected {
		return findConsecutive(basePos, termPositions, termIdx+1)
	}
	return false
}

func (e *Evaluator) evalAnd(q *types.AndQuery) []*MatchedDoc {
	if len(q.Children) == 0 {
		return nil
	}

	var positive []types.QueryNode
	var negative []types.QueryNode
	for _, child := range q.Children {
		if nq, ok := child.(*types.NotQuery); ok {
			negative = append(negative, nq.Child)
		} else {
			positive = append(positive, child)
		}
	}

	var result []*MatchedDoc
	if len(positive) > 0 {
		for i, child := range positive {
			childResult := e.Evaluate(child)
			if childResult == nil {
				return nil
			}
			if i == 0 {
				result = childResult
			} else {
				result = intersectResults(result, childResult)
			}
			if len(result) == 0 {
				return nil
			}
		}
	} else {
		allIDs := e.idx.AllDocIDs()
		result = make([]*MatchedDoc, 0, len(allIDs))
		for _, id := range allIDs {
			result = append(result, &MatchedDoc{
				DocID:        id,
				MatchedTerms: make(map[string][]int),
			})
		}
	}

	for _, negChild := range negative {
		negResult := e.Evaluate(negChild)
		if negResult == nil {
			continue
		}
		negSet := make(map[types.DocID]bool)
		for _, md := range negResult {
			negSet[md.DocID] = true
		}
		filtered := make([]*MatchedDoc, 0, len(result))
		for _, md := range result {
			if !negSet[md.DocID] {
				filtered = append(filtered, md)
			}
		}
		result = filtered
		if len(result) == 0 {
			return nil
		}
	}

	return result
}

func (e *Evaluator) evalOr(q *types.OrQuery) []*MatchedDoc {
	if len(q.Children) == 0 {
		return nil
	}
	resultMap := make(map[types.DocID]*MatchedDoc)
	for _, child := range q.Children {
		childResult := e.Evaluate(child)
		for _, md := range childResult {
			if existing, ok := resultMap[md.DocID]; ok {
				for term, positions := range md.MatchedTerms {
					existing.MatchedTerms[term] = append(existing.MatchedTerms[term], positions...)
				}
			} else {
				matchedCopy := make(map[string][]int)
				for t, pos := range md.MatchedTerms {
					p := make([]int, len(pos))
					copy(p, pos)
					matchedCopy[t] = p
				}
				resultMap[md.DocID] = &MatchedDoc{
					DocID:        md.DocID,
					MatchedTerms: matchedCopy,
				}
			}
		}
	}
	result := make([]*MatchedDoc, 0, len(resultMap))
	for _, md := range resultMap {
		result = append(result, md)
	}
	return result
}

func (e *Evaluator) evalNot(q *types.NotQuery) []*MatchedDoc {
	negResult := e.Evaluate(q.Child)
	negSet := make(map[types.DocID]bool)
	for _, md := range negResult {
		negSet[md.DocID] = true
	}
	allIDs := e.idx.AllDocIDs()
	result := make([]*MatchedDoc, 0)
	for _, id := range allIDs {
		if !negSet[id] {
			result = append(result, &MatchedDoc{
				DocID:        id,
				MatchedTerms: make(map[string][]int),
			})
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func intersectResults(a, b []*MatchedDoc) []*MatchedDoc {
	bMap := make(map[types.DocID]*MatchedDoc)
	for _, md := range b {
		bMap[md.DocID] = md
	}
	var result []*MatchedDoc
	for _, md := range a {
		if bmd, ok := bMap[md.DocID]; ok {
			merged := &MatchedDoc{
				DocID:        md.DocID,
				MatchedTerms: make(map[string][]int),
			}
			for t, pos := range md.MatchedTerms {
				p := make([]int, len(pos))
				copy(p, pos)
				merged.MatchedTerms[t] = p
			}
			for t, pos := range bmd.MatchedTerms {
				if existing, ok := merged.MatchedTerms[t]; ok {
					p := make([]int, len(pos))
					copy(p, pos)
					merged.MatchedTerms[t] = append(existing, p...)
				} else {
					p := make([]int, len(pos))
					copy(p, pos)
					merged.MatchedTerms[t] = p
				}
			}
			result = append(result, merged)
		}
	}
	return result
}
