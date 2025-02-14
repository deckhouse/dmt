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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/dmt/pkg"
)

func Test_Errors(t *testing.T) {
	t1 := NewLinterRuleList("linterID", "moduleID")
	require.NotNil(t, t1)
	require.Equal(t, "linterID", t1.linterID)
	require.Equal(t, "moduleID", t1.moduleID)
	t2 := t1.WithObjectID("objectID")
	require.NotNil(t, t2)
	require.Equal(t, "linterID", t2.linterID)
	require.Equal(t, "moduleID", t2.moduleID)
	require.Equal(t, "objectID", t2.objectID)
	require.Empty(t, t1.objectID)
	require.NotEqual(t, t1, t2)
	require.NotEqual(t, t1.objectID, t2.objectID)
	t1.Error("test1")
	require.Len(t, t1.storage.GetErrors(), 1)
	t2.Error("test2")
	require.Len(t, t1.storage.GetErrors(), 2)
	require.Len(t, t2.storage.GetErrors(), 2)
	require.Equal(t,
		[]lintRuleError{
			{LinterID: "linterid", ModuleID: "moduleID", RuleID: "", ObjectID: "", Text: "test1", Level: pkg.Error},
			{LinterID: "linterid", ModuleID: "moduleID", RuleID: "", ObjectID: "objectID", Text: "test2", Level: pkg.Error}},
		t1.storage.GetErrors())
	t1.Error("test3")
	require.Len(t, t1.storage.GetErrors(), 3)
	require.Equal(t,
		[]lintRuleError{
			{LinterID: "linterid", ModuleID: "moduleID", ObjectID: "", Text: "test1", Level: pkg.Error},
			{LinterID: "linterid", ModuleID: "moduleID", ObjectID: "objectID", Text: "test2", Level: pkg.Error},
			{LinterID: "linterid", ModuleID: "moduleID", ObjectID: "", Text: "test3", Level: pkg.Error},
		},
		t1.storage.GetErrors())
	t3 := NewLinterRuleList("linterID", "moduleID2")
	require.NotNil(t, t3)
	t3.WithObjectID("objectID3").Error("test3")
	require.Equal(t,
		[]lintRuleError{
			{LinterID: "linterid", ModuleID: "moduleID2", ObjectID: "objectID3", Text: "test3", Level: pkg.Error},
		},
		t3.storage.GetErrors())
	require.Len(t, t3.storage.GetErrors(), 1)
}
