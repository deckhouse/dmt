package roles

import (
	"slices"
	"strings"

	k8SRbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

// ObjectRolesWildcard is a linter for checking the presence
// of a wildcard in a Role and ClusterRole
func ObjectRolesWildcard(object storage.StoreObject, lintError *errors.Error) {
	// check only `rbac-for-us.yaml` files
	if !strings.HasSuffix(object.ShortPath(), "rbac-for-us.yaml") {
		return
	}

	// check Role and ClusterRole for wildcards
	objectKind := object.Unstructured.GetKind()
	switch objectKind {
	case "Role", "ClusterRole":
		checkRoles(object, lintError)
	default:
	}
}

func checkRoles(object storage.StoreObject, lintError *errors.Error) {
	// check rbac-proxy for skip
	for path, rules := range Cfg.SkipCheckWildcards {
		if strings.EqualFold(object.Path, path) {
			if slices.Contains(rules, object.Unstructured.GetName()) {
				return
			}
		}
	}

	converter := runtime.DefaultUnstructuredConverter

	role := new(k8SRbac.Role)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), role)
	if err != nil {
		lintError.WithObjectID(object.Identity()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)
		return
	}

	for _, rule := range role.Rules {
		var objs []string
		if slices.Contains(rule.APIGroups, "*") {
			objs = append(objs, "apiGroups")
		}
		if slices.Contains(rule.Resources, "*") {
			objs = append(objs, "resources")
		}
		if slices.Contains(rule.Verbs, "*") {
			objs = append(objs, "verbs")
		}
		if len(objs) > 0 {
			lintError.WithObjectID(object.Identity()).Add(
				"%s contains a wildcards. Replace them with an explicit list of resources",
				strings.Join(objs, ", "),
			)
			return
		}
	}
}
