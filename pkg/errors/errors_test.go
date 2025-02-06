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
	require.Len(t, *t1.storage, 1)
	t2.Add("test2")
	require.Len(t, *t1.storage, 2)
	require.Len(t, *t2.storage, 2)
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test1", Level: 1},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "objectID", Text: "test2", Level: 1}},
		*t1.storage)
	t1.Add("test3")
	require.Len(t, *t1.storage, 3)
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test1", Level: 1},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "objectID", Text: "test2", Level: 1},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test3", Level: 1},
		},
		*t1.storage)
	t3 := NewLinterRuleList("linterID", "moduleID2")
	require.NotNil(t, t3)
	t3.WithObjectID("objectID3").Add("test3")
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID2", ObjectID: "objectID3", Text: "test3", Level: 1},
		},
		*t3.storage)
	require.Len(t, *t3.storage, 1)

	t1.Merge(t3)
	require.Len(t, *t1.storage, 4)
	require.Equal(t,
		errStorage{
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test1", Level: 1},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "objectID", Text: "test2", Level: 1},
			lintRuleError{ID: "linterid", Module: "moduleID", ObjectID: "", Text: "test3", Level: 1},
			lintRuleError{ID: "linterid", Module: "moduleID2", ObjectID: "objectID3", Text: "test3", Level: 1},
		},
		*t1.storage)
}
