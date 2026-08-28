package main

// ast_dump binary: prints the tree-sitter AST for one .zig file so we
// can see exactly what node kinds and field names the parser produces.

import (
	"fmt"
	"os"

	sitter "github.com/tree-sitter/go-tree-sitter"
	zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ast_dump <file.zig>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(sitter.NewLanguage(zig.Language())); err != nil {
		fmt.Fprintln(os.Stderr, "language error:", err)
		os.Exit(1)
	}
	tree := parser.Parse(data, nil)
	if tree == nil {
		fmt.Fprintln(os.Stderr, "no tree")
		os.Exit(1)
	}
	defer tree.Close()
	printNode(tree.RootNode(), data, 0)
}

func printNode(n *sitter.Node, src []byte, depth int) {
	if n == nil {
		return
	}
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	fmt.Printf("%s%s [%d:%d-%d:%d]\n", indent, n.Kind(),
		n.StartPosition().Row, n.StartPosition().Column,
		n.EndPosition().Row, n.EndPosition().Column)

	if n.IsNamed() && n.ChildCount() == 0 {
		// Leaf named node — show the text.
		fmt.Printf("%s  text=%q\n", indent, n.Utf8Text(src))
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c != nil {
			if !c.IsNamed() {
				// Anonymous token — show its text on one line.
				indent := ""
				for j := 0; j < depth+1; j++ {
					indent += "  "
				}
				fmt.Printf("%s<anon> [%d:%d] text=%q\n", indent,
					c.StartPosition().Row, c.StartPosition().Column, c.Utf8Text(src))
				continue
			}
			printNode(c, src, depth+1)
		}
	}
}
