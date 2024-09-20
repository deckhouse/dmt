package k8s

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gammazero/deque"
	"github.com/go-openapi/spec"
	"github.com/mohae/deepcopy"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/d8-lint/pkg/module"
	"github.com/deckhouse/d8-lint/pkg/valuesvalidation"
)

const (
	ExamplesKey = "x-examples"
	ArrayObject = "array"
	ObjectKey   = "object"
)

// applyDigests if ugly because values now are strongly untyped. We have to rewrite this after adding proper global schema
func applyDigests(digests map[string]any, values any) {
	if values == nil {
		return
	}
	value, ok := values.(map[string]any)["global"]
	if value == nil || !ok {
		return
	}
	value, ok = value.(map[string]any)["modulesImages"]
	if value == nil || !ok {
		return
	}
	value.(map[string]any)["digests"] = digests
}

func helmFormatModuleImages(m *module.Module, rawValues []any) ([]chartutil.Values, error) {
	caps := chartutil.DefaultCapabilities
	vers := []string(caps.APIVersions)
	vers = append(vers, "autoscaling.k8s.io/v1/VerticalPodAutoscaler")
	caps.APIVersions = vers

	digests, err := GetModulesImagesDigests(m.GetPath())
	if err != nil {
		return nil, err
	}

	values := make([]chartutil.Values, 0, len(rawValues))
	for _, singleValue := range rawValues {
		applyDigests(digests, singleValue)

		top := map[string]any{
			"Chart":        m.GetMetadata(),
			"Capabilities": caps,
			"Release": map[string]any{
				"Name":      m.GetName(),
				"Namespace": m.GetNamespace(),
				"IsUpgrade": true,
				"IsInstall": true,
				"Revision":  0,
				"Service":   "Helm",
			},
			"Values": singleValue,
		}

		values = append(values, top)
	}
	return values, nil
}

func GetModulesImagesDigests(modulePath string) (map[string]any, error) {
	var (
		modulesDigests map[string]any
		search         bool
	)

	if fi, err := os.Stat(filepath.Join(filepath.Dir(modulePath), "images_digests.json")); err != nil || fi.Size() == 0 {
		search = true
	}

	var err error
	if search {
		modulesDigests = DefaultImagesDigests
	} else {
		modulesDigests, err = getModulesImagesDigestsFromLocalPath(modulePath)
		if err != nil {
			return nil, err
		}
	}

	return modulesDigests, nil
}

func getModulesImagesDigestsFromLocalPath(modulePath string) (map[string]any, error) {
	var digests map[string]any

	imageDigestsRaw, err := os.ReadFile(filepath.Join(filepath.Dir(modulePath), "images_digests.json"))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(imageDigestsRaw, &digests)
	if err != nil {
		return nil, err
	}

	return digests, nil
}

func ComposeValuesFromSchemas(m *module.Module) ([]chartutil.Values, error) {
	valueValidator, err := valuesvalidation.NewValuesValidator(m.GetName(), m.GetPath())
	if err != nil {
		return nil, fmt.Errorf("schemas load: %w", err)
	}

	if valueValidator == nil {
		return nil, nil
	}

	camelizedModuleName := ToLowerCamel(m.GetName())

	if valueValidator.ModuleSchemaStorages[m.GetName()].Schemas == nil {
		return nil, nil
	}

	values, ok := valueValidator.ModuleSchemaStorages[m.GetName()].Schemas["values"]
	if values == nil || !ok {
		return nil, fmt.Errorf("cannot find openapi values schema for module %s", m.GetName())
	}

	moduleSchema := *values
	moduleSchema.Default = make(map[string]any)

	values, ok = valueValidator.GlobalSchemaStorage.Schemas["values"]
	var globalSchema spec.Schema
	globalSchema.Default = make(map[string]any)
	if ok && values != nil {
		globalSchema = *values
	}

	combinedSchema := spec.Schema{}
	combinedSchema.Properties = map[string]spec.Schema{camelizedModuleName: moduleSchema, "global": globalSchema}

	rawValues, err := NewOpenAPIValuesGenerator(&combinedSchema).Do()
	if err != nil {
		return nil, fmt.Errorf("generate values: %w", err)
	}

	return helmFormatModuleImages(m, rawValues)
}

type OpenAPIValuesGenerator struct {
	rootSchema *spec.Schema

	schemaQueue *deque.Deque[SchemaNode]
	resultQueue *deque.Deque[SchemaNode]
}

func NewOpenAPIValuesGenerator(schema *spec.Schema) *OpenAPIValuesGenerator {
	s := deque.Deque[SchemaNode]{}
	r := deque.Deque[SchemaNode]{}

	return &OpenAPIValuesGenerator{
		rootSchema:  schema,
		schemaQueue: &s,
		resultQueue: &r,
	}
}

type SchemaNode struct {
	Schema *spec.Schema

	Leaf *map[string]any
}

type InteractionsCounter struct {
	counter int
}

func (c *InteractionsCounter) Inc() {
	c.counter++
}

func (c *InteractionsCounter) Zero() bool {
	return c.counter == 0
}

func (g *OpenAPIValuesGenerator) Do() ([]any, error) {
	newItem := make(map[string]any)
	g.schemaQueue.PushBack(SchemaNode{Schema: g.rootSchema, Leaf: &newItem})

	for g.schemaQueue.Len() > 0 {
		tempNode := g.schemaQueue.PopFront()
		counter := InteractionsCounter{}

		err := g.parseProperties(&tempNode, &counter)
		if err != nil {
			return nil, err
		}
		if counter.Zero() {
			g.resultQueue.PushBack(tempNode)
		}
	}

	values := make([]any, 0, g.resultQueue.Len())
	for g.resultQueue.Len() > 0 {
		resultNode := g.resultQueue.PopFront()
		values = append(values, *resultNode.Leaf)
	}

	return values, nil
}

func (g *OpenAPIValuesGenerator) pushBackNodesFromValues(tempNode *SchemaNode, key string, items []any, counter *InteractionsCounter) {
	for _, item := range items {
		headNode := copyNode(tempNode, key, item)
		g.deleteNodeAndPushBack(&headNode, key, counter)
	}
}

func (g *OpenAPIValuesGenerator) generateAndPushBackNodes(
	tempNode *SchemaNode, key string, prop *spec.Schema, counter *InteractionsCounter) error {
	downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
	// Recursive call, consider switching to a better solution.
	values, err := NewOpenAPIValuesGenerator(downwardSchema).Do()
	if err != nil {
		return err
	}

	g.pushBackNodesFromValues(tempNode, key, values, counter)
	return nil
}

func (g *OpenAPIValuesGenerator) parseProperties(tempNode *SchemaNode, counter *InteractionsCounter) error {
	for key := range tempNode.Schema.Properties {
		prop := tempNode.Schema.Properties[key]
		switch {
		case prop.Extensions[ExamplesKey] != nil:
			examples := prop.Extensions[ExamplesKey].([]any)
			g.pushBackNodesFromValues(tempNode, key, examples, counter)
			return nil

		case len(prop.Enum) > 0:
			g.pushBackNodesFromValues(tempNode, key, prop.Enum, counter)
			return nil

		case prop.Type.Contains(ObjectKey):
			if prop.Default == nil {
				g.deleteNodeAndPushBack(tempNode, key, counter)
				return nil
			}
			return g.generateAndPushBackNodes(tempNode, key, &prop, counter)

		case prop.Default != nil:
			g.schemaQueue.PushBack(copyNode(tempNode, key, prop.Default))
			counter.Inc()
			return nil

		case prop.Type.Contains(ArrayObject) && prop.Items.Schema != nil:
			switch {
			case prop.Items.Schema.Default != nil:
				var wrapped []any
				wrapped = append(wrapped, prop.Items.Schema.Default)

				g.schemaQueue.PushBack(copyNode(tempNode, key, wrapped))
				counter.Inc()
				return nil
			case prop.Items.Schema.Type.Contains(ObjectKey):
				if prop.Items.Schema.Default == nil {
					g.deleteNodeAndPushBack(tempNode, key, counter)
					return nil
				}

				downwardSchema := deepcopy.Copy(prop.Items.Schema).(spec.Schema)
				// Recursive call, consider switching to a better solution.
				values, err := NewOpenAPIValuesGenerator(&downwardSchema).Do()
				if err != nil {
					return err
				}

				for index, value := range values {
					var wrapped []any
					wrapped = append(wrapped, value)

					values[index] = wrapped
				}
				g.pushBackNodesFromValues(tempNode, key, values, counter)
				return nil
			default:
				g.deleteNodeAndPushBack(tempNode, key, counter)
				return nil
			}
		case prop.AllOf != nil:
			// not implemented
			continue
		case prop.OneOf != nil:
			for i := range prop.OneOf {
				schema := prop.OneOf[i]
				downwardSchema := deepcopy.Copy(prop).(*spec.Schema)

				mergedSchema := mergeSchemas(downwardSchema, schema)
				return g.generateAndPushBackNodes(tempNode, key, &mergedSchema, counter)
			}
			return nil

		case prop.AnyOf != nil:
			for i := range prop.AnyOf {
				schema := prop.AnyOf[i]
				downwardSchema := deepcopy.Copy(prop).(*spec.Schema)
				mergedSchema := mergeSchemas(downwardSchema, schema)

				if err := g.generateAndPushBackNodes(tempNode, key, &mergedSchema, counter); err != nil {
					return err
				}
			}
			return g.generateAndPushBackNodes(tempNode, key, &prop, counter)
		default:
			g.deleteNodeAndPushBack(tempNode, key, counter)
			return nil
		}
	}
	return nil
}

func (g *OpenAPIValuesGenerator) deleteNodeAndPushBack(tempNode *SchemaNode, key string, counter *InteractionsCounter) {
	delete(tempNode.Schema.Properties, key)

	g.schemaQueue.PushBack(*tempNode)
	counter.Inc()
}

func copyNode(previousNode *SchemaNode, key string, value any) SchemaNode {
	tempNode := *previousNode

	newSchema := deepcopy.Copy(*previousNode.Schema).(spec.Schema)
	delete(newSchema.Properties, key)

	leaf := *tempNode.Leaf
	leaf[key] = value

	newItem := deepcopy.Copy(leaf).(map[string]any)
	return SchemaNode{Leaf: &newItem, Schema: &newSchema}
}

func mergeSchemas(rootSchema *spec.Schema, schemas ...spec.Schema) spec.Schema {
	rootSchema.OneOf = nil
	rootSchema.AllOf = nil
	rootSchema.AnyOf = nil

	for i := range schemas {
		schema := schemas[i]
		for key := range schema.Properties {
			rootSchema.Properties[key] = schema.Properties[key]
		}
		rootSchema.OneOf = schema.OneOf
		rootSchema.AllOf = schema.AllOf
		rootSchema.AnyOf = schema.AnyOf
	}

	return *rootSchema
}
