package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func TestEnumRule(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		excludeFiles []string
		wantErrors   []string
	}{
		{
			name: "valid enum values",
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
            - Active
            - Inactive
            - Pending`,
			excludeFiles: nil,
			wantErrors:   nil,
		},
		{
			name: "invalid enum values - lowercase start",
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
            - Inactive
            - Pending`,
			excludeFiles: nil,
			wantErrors:   []string{"value 'active' must start with Capital letter"},
		},
		{
			name: "invalid enum values - special characters",
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
            - Active!
            - Inactive
            - Pending`,
			excludeFiles: nil,
			wantErrors:   []string{"value: 'Active!' must be in CamelCase"},
		},
		{
			name: "valid enum values with numbers",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        version:
          type: string
          enum:
            - V1
            - V2
            - V3`,
			excludeFiles: nil,
			wantErrors:   nil,
		},
		{
			name: "valid enum values with float numbers",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        version:
          type: string
          enum:
            - V1.0
            - V2.0
            - V3.0`,
			excludeFiles: nil,
			wantErrors:   nil,
		},
		{
			name: "excluded enum path",
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
            - Active
            - Inactive`,
			excludeFiles: []string{"components.schemas.Test.properties.status.enum"},
			wantErrors:   nil,
		},
		{
			name: "excluded enum path with array wildcard",
			content: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      properties:
        items:
          type: array
          items:
            type: object
            properties:
              status:
                type: string
                enum:
                  - Active
                  - Inactive`,
			excludeFiles: []string{"components.schemas.Test.properties.items.items.properties.status.enum"},
			wantErrors:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, cleanup := createTempFile(t, tt.content)
			defer cleanup()

			cfg := &pkg.OpenAPILinterConfig{
				ExcludeRules: pkg.OpenAPIExcludeRules{
					EnumFileExcludes: tt.excludeFiles,
				},
			}
			rule := NewEnumRule(cfg, "test")
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
