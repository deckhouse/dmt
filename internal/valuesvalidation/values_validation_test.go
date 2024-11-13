package valuesvalidation

import (
	"testing"

	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/stretchr/testify/require"
)

func Test_ValidateValues_check_for_known_go_1_16_problem(t *testing.T) {
	const message = `test should not fail with error 'must be of type string: "null"'.
 There is a problem with Go 1.16 and go-openapi, see https://github.com/go-openapi/validate/issues/137.
 go-openapi should be updated in addon-operator to use Go 1.16 in deckhouse.`
	const values = `
{"global":{},
"nodeManager":{
  "internal":{
    "manualRolloutID":"null"
  }
}}`
	const schema = `
type: object
properties:
  internal:
    type: object
    properties:
      manualRolloutID:
        type: string
`

	schemaStorage, err := validation.NewSchemaStorage([]byte{}, []byte(schema))
	require.NoError(t, err, "should load schema")

	valuesValidator := &ValuesValidator{
		GlobalSchemaStorage: schemaStorage,
		ModuleSchemaStorages: map[string]*validation.SchemaStorage{
			"nodeManager": schemaStorage,
		},
	}

	// Validate empty string
	err = valuesValidator.ValidateJSONValues("nodeManager", []byte(values), false)
	require.NoError(t, err, message)
}
