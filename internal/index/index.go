package index

import (
	"encoding/gob"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/localsearch/cli/internal/tokenizer"
	"github.com/localsearch/cli/pkg/types"
)

type Index struct {
	mu         sync.RWMutex
	Documents  map[types.DocID]*types.Document
	pathToID   map[string]types.DocID
	postings   map[string]types.PostingList
	nextDocID  types.DocID
	AvgDocLen  float64
	TotalDocs  int
}

func New() *Index {
	return &Index{
		Documents: make(map[types.DocID]*types.Document),
		pathToID:  make(map[string]types.DocID),
		postings:   make(map[string]types.PostingList),
		nextDocID:  1,
	}
}

func (idx *Index) AddDocument(docPath string, content string, modTime int64, size int64, checksum string) (*types.Document, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tok := tokenizer.New()
	tokens := tok.Tokenize(content)

	docID, exists := idx.pathToID[docPath]
	var doc *types.Document
	if exists {
		doc = idx.Documents[docID]
		idx.removePostingsForDoc(docID)
	} else {
		docID = idx.nextDocID
		idx.nextDocID++
		doc = &types.Document{
			ID:       docID,
			Path:     docPath,
			ModTime:  unixToTime(modTime),
			Size:     size,
			Checksum: checksum,
		}
		idx.Documents[docID] = doc
		idx.pathToID[docPath] = docID
	}

	doc.Length = len(tokens)

	termFreq := make(map[string][]int)
	for _, t := range tokens {
		termFreq[t.Term] = append(termFreq[t.Term], t.Position)
	}

	for term, positions := range termFreq {
		posting := &types.Posting{
			DocID:   docID,
			Positions: positions,
			TermFreq:  len(positions),
		}
		idx.postings[term] = append(idx.postings[term], posting)
	}

	idx.recomputeAvgDocLen()
	return doc, nil
}

func (idx *Index) removePostingsForDoc(docID types.DocID) {
	for term, pl := range idx.postings {
		newPl := make(types.PostingList, 0, len(pl))
		for _, p := range pl {
			if p.DocID != docID {
				newPl = append(newPl, p)
			}
		}
		if len(newPl) == 0 {
			delete(idx.postings, term)
		} else {
			idx.postings[term] = newPl
		}
	}
}

func (idx *Index) RemoveDocument(docID types.DocID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	doc, ok := idx.Documents[docID]
	if !ok {
		return
	}
	delete(idx.Documents, docID)
	delete(idx.pathToID, doc.Path)
	idx.removePostingsForDoc(docID)
	idx.recomputeAvgDocLen()
}

func (idx *Index) GetDocumentByPath(path string) (*types.Document, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	id, ok := idx.pathToID[path]
	if !ok {
		return nil, false
	}
	doc, ok := idx.Documents[id]
	return doc, ok
}

func (idx *Index) GetPostings(term string) (types.PostingList, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	pl, ok := idx.postings[term]
	return pl, ok
}

func (idx *Index) GetTermsByPrefix(prefix string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var terms []string
	for term := range idx.postings {
		if len(term) >= len(prefix) && term[:len(prefix)] == prefix {
			terms = append(terms, term)
		}
	}
	sort.Strings(terms)
	return terms
}

func (idx *Index) Terms() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	terms := make([]string, 0, len(idx.postings))
	for term := range idx.postings {
		terms = append(terms, term)
	}
	return terms
}

func (idx *Index) UniqueTermCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.postings)
}

func (idx *Index) DocumentCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.Documents)
}

func (idx *Index) AverageDocLength() float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.AvgDocLen
}

func (idx *Index) GetDocument(docID types.DocID) (*types.Document, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	doc, ok := idx.Documents[docID]
	return doc, ok
}

func (idx *Index) AllDocIDs() []types.DocID {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	ids := make([]types.DocID, 0, len(idx.Documents))
	for id := range idx.Documents {
		ids = append(ids, id)
	}
	return ids
}

func (idx *Index) recomputeAvgDocLen() {
	total := 0
	for _, doc := range idx.Documents {
		total += doc.Length
	}
	idx.TotalDocs = len(idx.Documents)
	if idx.TotalDocs > 0 {
		idx.AvgDocLen = float64(total) / float64(idx.TotalDocs)
	} else {
		idx.AvgDocLen = 0
	}
}

func (idx *Index) Save(path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)

	data := struct {
		Documents  map[types.DocID]*types.Document
		PathToID   map[string]types.DocID
		Postings    map[string]types.PostingList
		NextDocID  types.DocID
		AvgDocLen   float64
		TotalDocs   int
	}{
		Documents:  idx.Documents,
		PathToID:   idx.pathToID,
		Postings:    idx.postings,
		NextDocID:  idx.nextDocID,
		AvgDocLen:   idx.AvgDocLen,
		TotalDocs:   idx.TotalDocs,
	}

	return enc.Encode(data)
}

func Load(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)

	var data struct {
		Documents  map[types.DocID]*types.Document
		PathToID   map[string]types.DocID
		Postings    map[string]types.PostingList
		NextDocID  types.DocID
		AvgDocLen   float64
		TotalDocs   int
	}

	if err := dec.Decode(&data); err != nil {
		return nil, err
	}

	return &Index{
		Documents: data.Documents,
		pathToID:  data.PathToID,
		postings:   data.Postings,
		nextDocID:  data.NextDocID,
		AvgDocLen:   data.AvgDocLen,
		TotalDocs:   data.TotalDocs,
	}, nil
}

func unixToTime(sec int64) time.Time {
	return time.Unix(sec, 0)
}
