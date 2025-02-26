package schema

import "github.com/go-openapi/spec"

type AdditionalPropertiesTransformer struct {
	Parent *spec.Schema
}

// Transform sets undefined AdditionalProperties to false recursively.
func (t *AdditionalPropertiesTransformer) Transform(s *spec.Schema) *spec.Schema {
	if s == nil {
		return nil
	}

	if s.AdditionalProperties == nil {
		s.AdditionalProperties = &spec.SchemaOrBool{
			Allows: false,
		}
	}

	for k := range s.Properties {
		prop := s.Properties[k]
		if prop.AdditionalProperties == nil {
			prop.AdditionalProperties = &spec.SchemaOrBool{
				Allows: false,
			}
			s.Properties[k] = *t.Transform(&prop)
		}
	}

	if s.Items != nil {
		if s.Items.Schema != nil {
			s.Items.Schema = t.Transform(s.Items.Schema)
		}
		for i := range s.Items.Schemas {
			s.Items.Schemas[i] = *t.Transform(&s.Items.Schemas[i])
		}
	}

	return s
}
