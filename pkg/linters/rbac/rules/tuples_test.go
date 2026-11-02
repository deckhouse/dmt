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
	"testing"

	"github.com/stretchr/testify/assert"
)

// A grant on every object covers the declared grant on one name, never the other way round (review
// of #479, reply to finding 13d).
func TestTupleSet_UncoveredBy(t *testing.T) {
	pinned := tupleSet{}
	pinned.add(resourceTuple("deckhouse.io", "moduleconfigs", "deckhouse", "get"))

	wide := tupleSet{}
	wide.add(resourceTuple("deckhouse.io", "moduleconfigs", "", "get"))

	assert.Empty(t, pinned.uncoveredBy(wide))
	assert.Equal(t, []tuple{resourceTuple("deckhouse.io", "moduleconfigs", "", "get")}, wide.uncoveredBy(pinned))
}
