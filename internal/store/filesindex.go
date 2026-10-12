package store

import (
	"errors"

	"go.yaml.in/yaml/v3"
)

// attachmentEntry is one file of a ticket's files/index.yaml.
type attachmentEntry struct {
	File    string `yaml:"file"`
	Caption string `yaml:"caption"`
	Kind    string `yaml:"kind"`
	Source  string `yaml:"source"`
	Run     string `yaml:"run"`
	Added   string `yaml:"added"`
	SHA256  string `yaml:"sha256,omitempty"`
}

// filesIndex is a ticket's files/index.yaml: its entries, and the document
// they came from, so adding a file keeps what Flashheart does not write
// (unknown keys, comments). It is the one reader and writer of the index.
type filesIndex struct {
	doc     *yaml.Node
	entries []attachmentEntry
}

var errNotAFileList = errors.New("is not a list of files")

// parseFilesIndex reads an index; empty content is an empty index.
func parseFilesIndex(data []byte) (filesIndex, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return filesIndex{}, errors.Join(errNotAFileList, err)
	}
	if doc.Kind == 0 || len(doc.Content) == 1 && doc.Content[0].Tag == "!!null" {
		return filesIndex{}, nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.SequenceNode {
		return filesIndex{}, errNotAFileList
	}
	var entries []attachmentEntry
	if err := doc.Content[0].Decode(&entries); err != nil {
		return filesIndex{}, errors.Join(errNotAFileList, err)
	}
	return filesIndex{doc: &doc, entries: entries}, nil
}

// add appends an entry.
func (x *filesIndex) add(entry attachmentEntry) error {
	var node yaml.Node
	if err := node.Encode(entry); err != nil {
		return err
	}
	if x.doc == nil {
		x.doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.SequenceNode}}}
	}
	list := x.doc.Content[0]
	list.Content = append(list.Content, &node)
	x.entries = append(x.entries, entry)
	return nil
}

// encode is the index's content.
func (x filesIndex) encode() ([]byte, error) {
	if x.doc == nil {
		return yaml.Marshal([]attachmentEntry{})
	}
	return yaml.Marshal(x.doc)
}
