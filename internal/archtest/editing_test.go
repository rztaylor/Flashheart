package archtest

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every ## Notes bullet Flashheart writes is board.NoteLine's (FH-85): the
// bullet's " · " separator is spelled once, there.
func TestNoteBulletsComeFromNoteLine(t *testing.T) {
	t.Parallel()

	var spelled []string
	for _, top := range []string{"cmd", "internal"} {
		err := goFiles(filepath.Join("..", "..", top), func(name string, file *ast.File) {
			ast.Inspect(file, func(node ast.Node) bool {
				if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && strings.Contains(literal.Value, " · ") {
					spelled = append(spelled, filepath.ToSlash(name)+": "+literal.Value)
				}
				return true
			})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(spelled) != 1 || !strings.HasSuffix(spelled[0], "internal/board/rules.go: \" · \"") {
		t.Errorf("the note bullet is spelled %v; want it once, in board.NoteLine", spelled)
	}
}

// The board's field edits and an agent's update_ticket share one
// change-set (FH-85): the API and the MCP tools decode into
// board.NewFieldChanges and write through store.EditTicket, so neither
// checks fields, sets a title or changes a workstream itself.
func TestTicketFieldEditsGoThroughEditTicket(t *testing.T) {
	t.Parallel()

	forbidden := []string{"ValidateField", "SetTitle", "SetTicketWorkstream"}
	for _, pkg := range []string{"internal/api", "internal/mcpserver"} {
		err := goFiles(filepath.Join("..", "..", pkg), func(name string, file *ast.File) {
			ast.Inspect(file, func(node ast.Node) bool {
				if selector, ok := node.(*ast.SelectorExpr); ok && slices.Contains(forbidden, selector.Sel.Name) {
					t.Errorf("%s uses %s; decode into board.NewFieldChanges and write with store.EditTicket", filepath.ToSlash(name), selector.Sel.Name)
				}
				return true
			})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
