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

package linters

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

// BaseLinter provides common functionality for all linters
type BaseLinter struct {
	name, desc string
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

// NewBaseLinter creates a new base linter with common initialization
func NewBaseLinter(id, desc string, _ *config.ModuleConfig, impact *pkg.Level, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *BaseLinter {
	return &BaseLinter{
		name:      id,
		desc:      desc,
		ErrorList: errorList.WithLinterID(id).WithMaxLevel(impact),
		tracker:   tracker,
	}
}

// Name returns the linter name
func (l *BaseLinter) Name() string {
	return l.name
}

// Desc returns the linter description
func (l *BaseLinter) Desc() string {
	return l.desc
}

// HasTracker returns true if the linter has exclusion tracking enabled
func (l *BaseLinter) HasTracker() bool {
	return l.tracker != nil
}

// GetTracker returns the exclusion tracker
func (l *BaseLinter) GetTracker() *exclusions.ExclusionTracker {
	return l.tracker
}

// GetErrorList returns the error list
func (l *BaseLinter) GetErrorList() *errors.LintRuleErrorsList {
	return l.ErrorList
}

// Linter defines the interface that all linters must implement
type Linter interface {
	Run(m *module.Module)
	Name() string
	Desc() string
}
