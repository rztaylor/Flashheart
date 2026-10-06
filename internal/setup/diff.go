package setup

import (
	"fmt"
	"strings"
)

// unifiedDiff renders a line diff of before and after with three lines of
// context, in the familiar unified format. Settings files are small, so a
// plain longest-common-subsequence table is enough.
func unifiedDiff(name string, before, after []byte) string {
	a, b := lines(before), lines(after)
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type op struct {
		kind byte // ' ', '-', '+'
		text string
		ai   int
		bi   int
	}
	var ops []op
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			ops = append(ops, op{' ', a[i], i, j})
			i, j = i+1, j+1
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			ops = append(ops, op{'+', b[j], i, j})
			j++
		default:
			ops = append(ops, op{'-', a[i], i, j})
			i++
		}
	}
	const context = 3
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", name, name)
	for start := 0; start < len(ops); {
		if ops[start].kind == ' ' {
			start++
			continue
		}
		first := max(0, start-context)
		end := start
		for k := start; k < len(ops); k++ {
			if ops[k].kind != ' ' {
				end = k
			} else if k-end > 2*context {
				break
			}
		}
		last := min(len(ops)-1, end+context)
		removed, added := 0, 0
		for _, o := range ops[first : last+1] {
			if o.kind != '+' {
				removed++
			}
			if o.kind != '-' {
				added++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", ops[first].ai+1, removed, ops[first].bi+1, added)
		for _, o := range ops[first : last+1] {
			out.WriteByte(o.kind)
			out.WriteString(o.text)
			out.WriteByte('\n')
		}
		start = last + 1
	}
	return out.String()
}

func lines(data []byte) []string {
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
