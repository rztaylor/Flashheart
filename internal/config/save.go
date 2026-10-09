package config

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SetUI returns config.yaml content with the UI preferences replaced (CFG-2).
// Other settings, unknown keys and comments are kept; the caller writes the
// result (store owns the root).
func SetUI(data []byte, ui UI) ([]byte, error) {
	if problems := ui.problems(); len(problems) > 0 {
		return nil, errors.New("invalid preferences: " + strings.Join(problems, "; "))
	}
	var document yaml.Node
	if len(bytes.TrimSpace(data)) > 0 {
		if err := yaml.Unmarshal(data, &document); err != nil {
			return nil, fmt.Errorf("parse config.yaml: %w", err)
		}
	}
	if len(document.Content) == 0 {
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
		set(document.Content[0], "version", scalarNode(fmt.Sprint(FormatVersion), "!!int"))
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("config.yaml is not a mapping of settings")
	}
	uiNode := lookup(root, "ui")
	if uiNode == nil || uiNode.Kind != yaml.MappingNode {
		uiNode = &yaml.Node{Kind: yaml.MappingNode}
		set(root, "ui", uiNode)
	}
	set(uiNode, "theme", scalarNode(ui.Theme, "!!str"))
	set(uiNode, "density", scalarNode(ui.Density, "!!str"))
	set(uiNode, "colour_by", scalarNode(ui.ColourBy, "!!str"))
	// Needs you and Agent working became filters (FH-42, FH-44).
	remove(uiNode, "virtual_columns")
	set(uiNode, "hidden_columns", flowList(ui.HiddenColumns))
	if len(ui.Scopes) == 0 {
		remove(uiNode, "scopes")
	} else {
		var scopes yaml.Node
		if err := scopes.Encode(ui.Scopes); err != nil {
			return nil, err
		}
		set(uiNode, "scopes", &scopes)
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// flowList is a list of strings written on one line, [a, b].
func flowList(values []string) *yaml.Node {
	list := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, value := range values {
		list.Content = append(list.Content, scalarNode(value, "!!str"))
	}
	return list
}

func scalarNode(value, tag string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
}

func lookup(mapping *yaml.Node, key string) *yaml.Node {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

// set replaces a key's value in place, keeping its comments, or appends it.
func set(mapping *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			old := mapping.Content[index+1]
			value.LineComment, value.HeadComment, value.FootComment = old.LineComment, old.HeadComment, old.FootComment
			mapping.Content[index+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, scalarNode(key, "!!str"), value)
}

func remove(mapping *yaml.Node, key string) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
			return
		}
	}
}
