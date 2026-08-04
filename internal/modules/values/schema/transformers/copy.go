package transformers

import "github.com/go-openapi/spec"

type Copy struct{}

func (*Copy) Transform(s *spec.Schema) *spec.Schema {
	tmpBytes, _ := s.MarshalJSON()
	res := new(spec.Schema)
	_ = res.UnmarshalJSON(tmpBytes)

	return res
}
