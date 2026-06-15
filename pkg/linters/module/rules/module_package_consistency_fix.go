/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// patchModuleYAML reads module.yaml, applies mutate to its root mapping node and
// writes it back if mutate reported a change. The YAML tree is edited in place,
// so comments, key order and unrelated fields are preserved. mutate must be
// idempotent: when nothing needs changing it must return false.
// The file is written atomically via temp file + rename and the original file
// permissions are preserved.
func patchModuleYAML(modulePath string, mutate func(root *yaml.Node) bool) error {
	path := filepath.Join(modulePath, ModuleConfigFilename)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", ModuleConfigFilename, err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", ModuleConfigFilename, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", ModuleConfigFilename, err)
	}

	root := documentRoot(&doc)
	if root == nil {
		return fmt.Errorf("%s is empty or not a mapping", ModuleConfigFilename)
	}

	if !mutate(root) {
		return nil
	}

	out, err := marshalYAMLNode(&doc)
	if err != nil {
		return fmt.Errorf("render %s: %w", ModuleConfigFilename, err)
	}

	if bytes.Equal(out, content) {
		return nil
	}

	tmpFile := path + ".fix.tmp"
	if err := os.WriteFile(tmpFile, out, fi.Mode()); err != nil {
		return fmt.Errorf("write temp %s: %w", ModuleConfigFilename, err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename temp %s: %w", ModuleConfigFilename, err)
	}

	return nil
}

// optionalModuleValue builds the module.yaml value for a conditional dependency.
func optionalModuleValue(constraint string) string {
	if constraint == "" {
		return "!optional"
	}

	return constraint + " !optional"
}

// setModuleEntry sets requirements.modules[name] = value, creating the
// requirements and modules mappings when they are missing.
func setModuleEntry(root *yaml.Node, name, value string) bool {
	req := ensureMapping(root, "requirements")
	modules := ensureMapping(req, "modules")

	return setScalar(modules, name, value)
}

// removeModuleEntry removes requirements.modules[name] if present.
func removeModuleEntry(root *yaml.Node, name string) bool {
	req := mappingValue(root, "requirements")
	if req == nil {
		return false
	}

	modules := mappingValue(req, "modules")
	if modules == nil || modules.Kind != yaml.MappingNode {
		return false
	}

	return removeMapKey(modules, name)
}

// setRequirementScalar sets requirements.<key> = value, creating requirements
// when missing.
func setRequirementScalar(root *yaml.Node, key, value string) bool {
	req := ensureMapping(root, "requirements")

	return setScalar(req, key, value)
}

// removeRequirementKey removes requirements.<key> if present.
func removeRequirementKey(root *yaml.Node, key string) bool {
	req := mappingValue(root, "requirements")
	if req == nil {
		return false
	}

	return removeMapKey(req, key)
}

// documentRoot returns the top-level mapping node of a parsed YAML document, or
// nil if the document is empty or not a mapping.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}

	return root
}

// mappingValue returns the value node for key in a mapping node, or nil.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

// ensureMapping returns the mapping value node for key, creating and attaching an
// empty mapping when it is missing or not a mapping.
func ensureMapping(mapping *yaml.Node, key string) *yaml.Node {
	value := mappingValue(mapping, key)
	if value != nil && value.Kind == yaml.MappingNode {
		return value
	}

	created := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setNode(mapping, key, created)

	return created
}

// setNode sets key to value in a mapping node, replacing the existing value or
// appending a new key/value pair.
func setNode(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

// setScalar sets key to a string scalar value. Returns true if the value
// changed (or the key was added).
func setScalar(mapping *yaml.Node, key, value string) bool {
	if existing := mappingValue(mapping, key); existing != nil {
		if existing.Kind == yaml.ScalarNode && existing.Value == value {
			return false
		}

		existing.Kind = yaml.ScalarNode
		existing.Tag = "!!str"
		existing.Value = value
		existing.Style = 0
		existing.Content = nil

		return true
	}

	setNode(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})

	return true
}

// removeMapKey removes key (and its value) from a mapping node.
func removeMapKey(mapping *yaml.Node, key string) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return true
		}
	}

	return false
}

// marshalYAMLNode renders a YAML node tree using a 2-space indent, matching the
// conventional module.yaml formatting.
func marshalYAMLNode(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(node); err != nil {
		_ = enc.Close()
		return nil, err
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
