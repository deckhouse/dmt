package rbac

import (
	"slices"
	"strings"

	k8SRbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

// objectRolesWildcard is a linter for checking the presence
// of a wildcard in a Role and ClusterRole
func (l *Rbac) objectRolesWildcard(m *module.Module) {
	errorList := l.ErrorList.WithModule(m.GetName()).WithRule("objectRolesWildcard")
	for _, object := range m.GetObjectStore().Storage {
		// check only `rbac-for-us.yaml` files
		if !strings.HasSuffix(object.ShortPath(), "rbac-for-us.yaml") {
			continue
		}

		errorList = errorList.WithObjectID(object.Identity()).WithFilePath(object.ShortPath())

		// check Role and ClusterRole for wildcards
		objectKind := object.Unstructured.GetKind()
		switch objectKind {
		case "Role", "ClusterRole":
			l.checkRoles(object, errorList)
		}
	}
}

func (l *Rbac) checkRoles(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	// check rbac-proxy for skip
	for path, rules := range l.cfg.SkipCheckWildcards {
		if strings.EqualFold(object.Path, path) {
			if slices.Contains(rules, object.Unstructured.GetName()) {
				return
			}
		}
	}

	converter := runtime.DefaultUnstructuredConverter
	role := new(k8SRbac.Role)

	if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), role); err != nil {
		errorList.Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)
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
			errorList.Errorf("%s contains a wildcards. Replace them with an explicit list of resources", strings.Join(objs, ", "))
			return
		}
	}
}
