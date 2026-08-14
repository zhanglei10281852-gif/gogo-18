package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/localsearch/cli/internal/index"
	"github.com/localsearch/cli/internal/query"
	"github.com/localsearch/cli/internal/ranker"
	"github.com/localsearch/cli/internal/scanner"
	"github.com/localsearch/cli/pkg/types"
)

const (
	defaultIndexDir = ".localsearch"
	indexFileName   = "index.gob"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "index":
		runIndex(os.Args[2:])
	case "search":
		runSearch(os.Args[2:])
	case "stats":
		runStats(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`localsearch - offline full-text search for notes and code

Usage:
  localsearch <command> [arguments]

Commands:
  index <dir>       Recursively index files in directory (incremental)
  search <query>    Search the index (supports AND/OR/NOT, "phrase", prefix*)
  stats             Show index statistics

Examples:
  localsearch index ~/notes
  localsearch search "machine learning" AND python
  localsearch search "quick brown fox"
  localsearch search algo*
  localsearch search golang NOT rust`)
}

func getIndexPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, defaultIndexDir, indexFileName)
}

func ensureIndexDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, defaultIndexDir)
	return os.MkdirAll(dir, 0755)
}

func loadIndex() (*index.Index, error) {
	path := getIndexPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return index.New(), nil
	}
	return index.Load(path)
}

func saveIndex(idx *index.Index) error {
	if err := ensureIndexDir(); err != nil {
		return err
	}
	return idx.Save(getIndexPath())
}

func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	force := fs.Bool("force", false, "Force full reindex")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: localsearch index <directory>")
		os.Exit(1)
	}

	rootDir := fs.Arg(0)
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", absRoot)
		os.Exit(1)
	}

	idx, err := loadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scanning %s...\n", absRoot)
	scan := scanner.New(absRoot)
	files, err := scan.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d candidate files\n", len(files))

	added := 0
	updated := 0
	unchanged := 0

	for _, fi := range files {
		existing, exists := idx.GetDocumentByPath(fi.Path)
		if exists && !*force {
			if existing.Checksum == fi.Checksum {
				unchanged++
				continue
			}
		}

		content, err := os.ReadFile(fi.Path)
		if err != nil {
			continue
		}

		_, err = idx.AddDocument(fi.Path, string(content), fi.ModTime.Unix(), fi.Size, fi.Checksum)
		if err != nil {
			continue
		}

		if exists {
			updated++
		} else {
			added++
		}
	}

	fmt.Printf("Added: %d, Updated: %d, Unchanged: %d\n", added, updated, unchanged)

	if err := saveIndex(idx); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Index saved to %s\n", getIndexPath())
}

func runSearch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: localsearch search <query>")
		os.Exit(1)
	}

	queryStr := joinArgs(args)

	idx, err := loadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		os.Exit(1)
	}

	if idx.DocumentCount() == 0 {
		fmt.Fprintln(os.Stderr, "Index is empty. Run 'localsearch index <dir>' first.")
		os.Exit(1)
	}

	parser := query.NewParser(queryStr)
	ast, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	evaluator := query.NewEvaluator(idx)
	matched := evaluator.Evaluate(ast)

	if len(matched) == 0 {
		fmt.Println("No results found.")
		return
	}

	bm25 := ranker.NewBM25()
	results := bm25.Score(idx, matched)

	if len(results) > 20 {
		results = results[:20]
	}

	fmt.Printf("Found %d result(s):\n\n", len(results))

	for i, res := range results {
		fmt.Printf("%d. %s  [score: %.4f]\n", i+1, res.Doc.Path, res.Score)

		content, err := os.ReadFile(res.Doc.Path)
		if err == nil {
			for _, md := range matched {
				if md.DocID == res.Doc.ID {
					frags := ranker.BuildFragments(string(content), md.MatchedTerms, 3)
					for _, f := range frags {
						fmt.Printf("   ... %s ...\n", renderFragment(f))
					}
					break
				}
			}
		}
		fmt.Println()
	}
}

func renderFragment(f types.Fragment) string {
	runes := []rune(f.Text)
	result := ""
	lastEnd := 0
	sort.Slice(f.Highlight, func(i, j int) bool {
		return f.Highlight[i].Start < f.Highlight[j].Start
	})
	for _, hl := range f.Highlight {
		if hl.Start >= len(runes) {
			continue
		}
		if hl.End > len(runes) {
			hl.End = len(runes)
		}
		if hl.Start > lastEnd {
			result += string(runes[lastEnd:hl.Start])
		}
		result += "\x1b[1;31m" + string(runes[hl.Start:hl.End]) + "\x1b[0m"
		lastEnd = hl.End
	}
	if lastEnd < len(runes) {
		result += string(runes[lastEnd:])
	}
	return result
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func runStats(args []string) {
	idx, err := loadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Index Statistics ===")
	fmt.Printf("Index path:       %s\n", getIndexPath())
	fmt.Printf("Documents:        %d\n", idx.DocumentCount())
	fmt.Printf("Unique terms:     %d\n", idx.UniqueTermCount())
	fmt.Printf("Avg doc length:   %.1f tokens\n", idx.AvgDocLen)

	totalSize := int64(0)
	oldest := time.Now()
	newest := time.Time{}
	for _, doc := range idx.Documents {
		totalSize += doc.Size
		if doc.ModTime.Before(oldest) {
			oldest = doc.ModTime
		}
		if doc.ModTime.After(newest) {
			newest = doc.ModTime
		}
	}
	fmt.Printf("Total file size:  %s\n", humanSize(totalSize))
	if idx.DocumentCount() > 0 {
		fmt.Printf("Oldest doc:       %s\n", oldest.Format("2006-01-02 15:04:05"))
		fmt.Printf("Newest doc:       %s\n", newest.Format("2006-01-02 15:04:05"))
	}
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
