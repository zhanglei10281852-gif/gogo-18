package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/localsearch/cli/pkg/types"
)

func TestAddDocumentAndPostings(t *testing.T) {
	idx := New()

	_, err := idx.AddDocument("/doc1", "hello world hello", 0, 100, "cs1")
	if err != nil {
		t.Fatal(err)
	}

	pl, ok := idx.GetPostings("hello")
	if !ok {
		t.Fatal("expected postings for 'hello'")
	}
	if len(pl) != 1 {
		t.Fatalf("expected 1 posting, got %d", len(pl))
	}
	if pl[0].TermFreq != 2 {
		t.Errorf("expected term freq 2, got %d", pl[0].TermFreq)
	}

	pl, ok = idx.GetPostings("world")
	if !ok {
		t.Fatal("expected postings for 'world'")
	}
	if pl[0].TermFreq != 1 {
		t.Errorf("expected term freq 1, got %d", pl[0].TermFreq)
	}
}

func TestDocumentCount(t *testing.T) {
	idx := New()

	if idx.DocumentCount() != 0 {
		t.Error("expected 0 docs")
	}

	idx.AddDocument("/doc1", "hello", 0, 10, "cs1")
	if idx.DocumentCount() != 1 {
		t.Error("expected 1 doc")
	}

	idx.AddDocument("/doc2", "world", 0, 10, "cs2")
	if idx.DocumentCount() != 2 {
		t.Error("expected 2 docs")
	}
}

func TestUniqueTermCount(t *testing.T) {
	idx := New()

	idx.AddDocument("/doc1", "hello world", 0, 10, "cs1")
	if idx.UniqueTermCount() != 2 {
		t.Errorf("expected 2 unique terms, got %d", idx.UniqueTermCount())
	}

	idx.AddDocument("/doc2", "hello foo", 0, 10, "cs2")
	if idx.UniqueTermCount() != 3 {
		t.Errorf("expected 3 unique terms, got %d", idx.UniqueTermCount())
	}
}

func TestGetTermsByPrefix(t *testing.T) {
	idx := New()

	idx.AddDocument("/doc1", "apple application app", 0, 10, "cs1")
	idx.AddDocument("/doc2", "banana", 0, 10, "cs2")

	terms := idx.GetTermsByPrefix("app")
	if len(terms) != 3 {
		t.Fatalf("expected 3 terms with prefix 'app', got %d: %v", len(terms), terms)
	}

	expected := map[string]bool{"app": true, "apple": true, "application": true}
	for _, tm := range terms {
		if !expected[tm] {
			t.Errorf("unexpected term %q", tm)
		}
	}
}

func TestIncrementalIndexUpdate(t *testing.T) {
	idx := New()

	doc1, err := idx.AddDocument("/doc1", "hello world", 0, 10, "cs1")
	if err != nil {
		t.Fatal(err)
	}
	firstID := doc1.ID

	_, err = idx.AddDocument("/doc1", "hello world foo", 0, 10, "cs2")
	if err != nil {
		t.Fatal(err)
	}

	if idx.DocumentCount() != 1 {
		t.Errorf("expected 1 doc after re-index same path, got %d", idx.DocumentCount())
	}

	doc, ok := idx.GetDocumentByPath("/doc1")
	if !ok {
		t.Fatal("expected doc1 to exist")
	}
	if doc.ID != firstID {
		t.Errorf("expected same DocID %d, got %d", firstID, doc.ID)
	}

	pl, ok := idx.GetPostings("foo")
	if !ok {
		t.Error("expected 'foo' to be indexed after update")
	} else if len(pl) != 1 {
		t.Errorf("expected 1 posting for 'foo', got %d", len(pl))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	idxPath := filepath.Join(dir, "test.gob")

	idx := New()
	idx.AddDocument("/doc1", "hello world", 0, 10, "cs1")
	idx.AddDocument("/doc2", "foo bar baz", 0, 10, "cs2")

	if err := idx.Save(idxPath); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if _, err := os.Stat(idxPath); os.IsNotExist(err) {
		t.Fatal("index file not created")
	}

	loaded, err := Load(idxPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.DocumentCount() != 2 {
		t.Errorf("expected 2 docs, got %d", loaded.DocumentCount())
	}

	if loaded.UniqueTermCount() != 5 {
		t.Errorf("expected 5 terms, got %d", loaded.UniqueTermCount())
	}

	_, ok := loaded.GetDocumentByPath("/doc1")
	if !ok {
		t.Error("doc1 not found after load")
	}

	pl, ok := loaded.GetPostings("hello")
	if !ok || len(pl) != 1 {
		t.Error("'hello' postings missing after load")
	}
}

func TestRemoveDocument(t *testing.T) {
	idx := New()

	idx.AddDocument("/doc1", "hello world", 0, 10, "cs1")
	doc, _ := idx.AddDocument("/doc2", "hello foo", 0, 10, "cs2")

	idx.RemoveDocument(doc.ID)

	if idx.DocumentCount() != 1 {
		t.Errorf("expected 1 doc, got %d", idx.DocumentCount())
	}

	pl, _ := idx.GetPostings("foo")
	if len(pl) != 0 {
		t.Error("'foo' should have no postings after removing doc2")
	}

	pl, _ = idx.GetPostings("hello")
	if len(pl) != 1 {
		t.Errorf("'hello' should have 1 posting, got %d", len(pl))
	}
}

func TestAvgDocLength(t *testing.T) {
	idx := New()

	idx.AddDocument("/doc1", "one two three", 0, 10, "cs1")
	idx.AddDocument("/doc2", "four five", 0, 10, "cs2")

	expected := (3.0 + 2.0) / 2.0
	if idx.AverageDocLength() != expected {
		t.Errorf("expected avg doc len %.1f, got %.1f", expected, idx.AverageDocLength())
	}
}

func TestPositionIndex(t *testing.T) {
	idx := New()

	idx.AddDocument("/doc1", "a b c d b", 0, 10, "cs1")

	pl, ok := idx.GetPostings("b")
	if !ok {
		t.Fatal("expected postings for 'b'")
	}

	if len(pl[0].Positions) != 2 {
		t.Fatalf("expected 2 positions for 'b', got %d", len(pl[0].Positions))
	}

	expected := []int{1, 4}
	for i, p := range expected {
		if pl[0].Positions[i] != p {
			t.Errorf("position %d: expected %d, got %d", i, p, pl[0].Positions[i])
		}
	}
}

func TestGetDocument(t *testing.T) {
	idx := New()
	doc, _ := idx.AddDocument("/doc1", "hello", 0, 42, "cs1")

	retrieved, ok := idx.GetDocument(doc.ID)
	if !ok {
		t.Fatal("expected to find document")
	}
	if retrieved.Size != 42 {
		t.Errorf("expected size 42, got %d", retrieved.Size)
	}

	_, ok = idx.GetDocument(types.DocID(9999))
	if ok {
		t.Error("should not find non-existent doc")
	}
}
