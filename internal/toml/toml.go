// Package toml provides format-preserving TOML edits and value queries.
package toml

import (
	"fmt"
	"sort"
	"strings"

	gotoml "github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/dayrion/upvert/internal/fileio"
)

// edit is one byte replacement inside a TOML document.
type edit struct {
	start       int
	end         int
	replacement string
}

// keyPath returns the decoded key parts of a table or key/value node.
func keyPath(node *unstable.Node) []string {
	var path []string
	keys := node.Key()
	for keys.Next() {
		path = append(path, string(keys.Node().Data))
	}
	return path
}

// pathIn reports whether path is one of the target key paths.
func pathIn(path []string, targets [][]string) bool {
	for _, target := range targets {
		if len(path) != len(target) {
			continue
		}
		matches := true
		for i := range target {
			if path[i] != target[i] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// Update replaces the value of version inside one of the given sections while
// preserving the rest of the document byte-for-byte.
func Update(data []byte, sections [][]string, version string) ([]byte, error) {
	targets := make([][]string, len(sections))
	for i, section := range sections {
		target := append([]string(nil), section...)
		targets[i] = append(target, "version")
	}

	var (
		parser  unstable.Parser
		current []string
		edits   []edit
	)
	parser.Reset(data)
	for parser.NextExpression() {
		expression := parser.Expression()
		switch expression.Kind {
		case unstable.Table:
			current = keyPath(expression)
		case unstable.ArrayTable:
			current = nil
		case unstable.KeyValue:
			key := append(append([]string(nil), current...), keyPath(expression)...)
			if !pathIn(key, targets) {
				continue
			}
			value := expression.Value()
			if value == nil || value.Kind != unstable.String {
				continue
			}
			raw := parser.Raw(value.Raw)
			quote := byte('"')
			if len(raw) > 0 && raw[0] == '\'' {
				quote = '\''
			}
			edits = append(edits, edit{
				start:       int(value.Raw.Offset),
				end:         int(value.Raw.Offset + value.Raw.Length),
				replacement: string(quote) + version + string(quote),
			})
		}
	}
	if err := parser.Error(); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}
	if len(edits) == 0 {
		return data, nil
	}

	// Replace from the end so earlier byte offsets stay valid.
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), data...)
	for _, edit := range edits {
		out = append(out[:edit.start], append([]byte(edit.replacement), out[edit.end:]...)...)
	}
	return out, nil
}

// Write updates a TOML file without reformatting it.
func Write(path string, sections [][]string, version string) (bool, error) {
	return fileio.Update(path, func(data []byte) ([]byte, error) {
		return Update(data, sections, version)
	})
}

// Decode parses a TOML document into a generic map.
func Decode(content string) (map[string]any, error) {
	var document map[string]any
	if err := gotoml.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}
	return document, nil
}

// String walks nested tables and returns a string value, or "" when the path is
// absent or not a string.
func String(document map[string]any, path ...string) string {
	var current any = document
	for _, key := range path {
		table, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = table[key]
		if !ok {
			return ""
		}
	}
	value, _ := current.(string)
	return strings.TrimSpace(value)
}

// TableExists reports whether path names a table.
func TableExists(document map[string]any, path ...string) bool {
	var current any = document
	for _, key := range path {
		table, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = table[key]
		if !ok {
			return false
		}
	}
	_, ok := current.(map[string]any)
	return ok
}
