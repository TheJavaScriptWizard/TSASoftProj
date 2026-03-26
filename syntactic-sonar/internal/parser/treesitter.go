package parser

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// SonarResult contains the depth and the node type found at the given coordinates.
type SonarResult struct {
	Depth    int
	NodeType string
}

// Analyzer holds the tree-sitter parser state
type Analyzer struct {
	parser    *sitter.Parser
	languages map[string]*sitter.Language
}

// NewAnalyzer initializes a new AST Analyzer for multi-language code
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		parser: sitter.NewParser(),
		languages: map[string]*sitter.Language{
			"go":         golang.GetLanguage(),
			"python":     python.GetLanguage(),
			"javascript": javascript.GetLanguage(),
			"typescript": typescript.GetLanguage(),
			"java":       java.GetLanguage(),
			"cpp":        cpp.GetLanguage(),
			"c":          cpp.GetLanguage(),
		},
	}
}

// Analyze parses the source code and calculates the depth of the node at the specified line and column.
// Note: line and col should be 0-indexed points.
func (a *Analyzer) Analyze(ctx context.Context, code []byte, line, col uint32, langId string) (SonarResult, error) {
	lang, ok := a.languages[langId]
	if !ok {
		// Fallback to Go if unknown
		lang = a.languages["go"]
	}
	a.parser.SetLanguage(lang)
	
	tree, err := a.parser.ParseCtx(ctx, nil, code)
	if err != nil {
		return SonarResult{}, err
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return SonarResult{}, fmt.Errorf("failed to get root node")
	}

	// We are looking for the narrowest node that contains the given point
	// Tree-sitter Points are 0-indexed for both row and column.
	point := sitter.Point{
		Row:    line,
		Column: col,
	}

	node := root.NamedDescendantForPointRange(point, point)
	if node == nil {
		return SonarResult{Depth: 0, NodeType: "unknown"}, nil
	}

	depth := 0
	curr := node
	for curr.Parent() != nil {
		depth++
		curr = curr.Parent()
	}

	return SonarResult{
		Depth:    depth,
		NodeType: node.Type(),
	}, nil
}
