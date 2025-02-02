package errors

import (
	"testing"

	"github.com/stretchr/testify/require"
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
	t1.Add("test1")
	require.Equal(t, 1, len(*t1.storage))
	t2.Add("test2")
	require.Equal(t, 2, len(*t1.storage))
	require.Equal(t, 2, len(*t2.storage))
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test1"},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "objectID", Text: "test2"}},
		*t1.storage)
	t1.Add("test3")
	require.Equal(t, 3, len(*t1.storage))
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test1"},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "objectID", Text: "test2"},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test3"},
		},
		*t1.storage)
	t3 := NewLinterRuleList("linterID", "moduleID2")
	require.NotNil(t, t3)
	t3.WithObjectID("objectID3").Add("test3")
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID2", ObjectID: "objectID3", Text: "test3"},
		},
		*t3.storage)
	require.Equal(t, 1, len(*t3.storage))

	t1.Merge(t3)
	require.Equal(t, 4, len(*t1.storage))
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test1"},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "objectID", Text: "test2"},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test3"},
			lintRuleError{ID: "linterid", Module: "moduleID2", ObjectID: "objectID3", Text: "test3"},
		},
		*t1.storage)
}
