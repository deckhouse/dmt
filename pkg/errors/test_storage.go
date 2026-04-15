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

package errors

import (
	"sync"

	"github.com/deckhouse/dmt/pkg"
)

type testErrStorage struct {
	mu      sync.Mutex
	errList []pkg.TestError
}

func (s *testErrStorage) GetErrors() []pkg.TestError {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]pkg.TestError, 0, len(s.errList))
	result = append(result, s.errList...)

	return result
}

func (s *testErrStorage) add(err *pkg.TestError) {
	s.mu.Lock()
	s.errList = append(s.errList, *err)
	s.mu.Unlock()
}
