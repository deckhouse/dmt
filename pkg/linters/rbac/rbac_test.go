/*
Copyright 2025 Flant JSC

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

package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/dmt/pkg"
)

// A rule's level narrows the linter's and never lifts it (review of #479, finding 7): impact:
// ignored on the rbac linter silences the declaration rules too.
func TestLower(t *testing.T) {
	assert.Equal(t, pkg.Ignored, *lower(ptr.To(pkg.Ignored), ptr.To(pkg.Warn)))
	assert.Equal(t, pkg.Warn, *lower(ptr.To(pkg.Error), ptr.To(pkg.Warn)))
	assert.Equal(t, pkg.Warn, *lower(nil, ptr.To(pkg.Warn)))
	assert.Equal(t, pkg.Error, *lower(ptr.To(pkg.Error), nil))
}
