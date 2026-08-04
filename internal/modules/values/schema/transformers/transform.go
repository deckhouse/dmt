package transformers

import "github.com/go-openapi/spec"

type Transformer interface {
	Transform(s *spec.Schema) *spec.Schema
}

func Transform(s *spec.Schema, transformers ...Transformer) *spec.Schema {
	for _, transformer := range transformers {
		s = transformer.Transform(s)
	}

	return s
}
