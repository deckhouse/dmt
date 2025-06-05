package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestKeysRule(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		bannedNames []string
		wantErrors  []string
	}{
		{
			name: "valid enum without banned names",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        status:
          type: string
          enum:
            - active
            - inactive`,
			bannedNames: []string{"banned"},
			wantErrors:  nil,
		},
		{
			name: "enum with banned name",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        status:
          type: string
          enum:
            - active
            - banned
            - inactive`,
			bannedNames: []string{"banned"},
			wantErrors:  []string{"banned is invalid name for property banned"},
		},
		{
			name: "nested enum with banned name",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        nested:
          type: object
          properties:
            status:
              type: string
              enum:
                - active
                - banned
                - inactive`,
			bannedNames: []string{"banned"},
			wantErrors:  []string{"banned is invalid name for property banned"},
		},
		{
			name: "array of enums with banned name",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        statuses:
          type: array
          items:
            type: string
            enum:
              - active
              - banned
              - inactive`,
			bannedNames: []string{"banned"},
			wantErrors:  []string{"banned is invalid name for property banned"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, cleanup := createTempFile(t, tt.content)
			defer cleanup()

			cfg := &config.OpenAPISettings{
				OpenAPIExcludeRules: config.OpenAPIExcludeRules{
					KeyBannedNames: tt.bannedNames,
				},
			}
			rule := NewKeysRule(cfg, "test")
			errorList := errors.NewLintRuleErrorsList()
			rule.Run(filePath, errorList)

			errs := errorList.GetErrors()
			if tt.wantErrors == nil {
				assert.Empty(t, errs)
			} else {
				assert.Len(t, errs, len(tt.wantErrors))
				for i, err := range errs {
					assert.Contains(t, err.Text, tt.wantErrors[i])
				}
			}
		})
	}
}
