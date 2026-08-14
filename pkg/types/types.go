package types

import "time"

type DocID uint32

type Document struct {
	ID       DocID
	Path     string
	ModTime  time.Time
	Size     int64
	Length   int
	Checksum string
}

type Token struct {
	Term     string
	Position int
}

type Posting struct {
	DocID     DocID
	Positions   []int
	TermFreq    int
}

type PostingList []*Posting

type SearchResult struct {
	Doc       *Document
	Score     float64
	Fragments []Fragment
}

type Fragment struct {
	Text      string
	Highlight []Range
}

type Range struct {
	Start int
	End   int
}

type QueryNode interface {
	queryNode()
}

type TermQuery struct {
	Term string
}

func (t *TermQuery) queryNode() {}

type PrefixQuery struct {
	Prefix string
}

func (p *PrefixQuery) queryNode() {}

type PhraseQuery struct {
	Terms []string
}

func (p *PhraseQuery) queryNode() {}

type AndQuery struct {
	Children []QueryNode
}

func (a *AndQuery) queryNode() {}

type OrQuery struct {
	Children []QueryNode
}

func (o *OrQuery) queryNode() {}

type NotQuery struct {
	Child QueryNode
}

func (n *NotQuery) queryNode() {}
