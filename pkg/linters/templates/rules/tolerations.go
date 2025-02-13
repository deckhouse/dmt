package rules

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	VPATolerationsRuleName = "vpa-tolerations"
)

func NewVPATolerationsRule(excludeRules []pkg.TargetRefRuleExclude) *TolerationsRule {
	return &TolerationsRule{
		RuleMeta: pkg.RuleMeta{
			Name: VPATolerationsRuleName,
		},
		TargetRefRule: pkg.TargetRefRule{
			ExcludeRules: excludeRules,
		},
	}
}

type TolerationsRule struct {
	pkg.RuleMeta
	pkg.TargetRefRule
}

// returns true if linting passed, otherwise returns false
func (r *TolerationsRule) EnsureTolerations(
	vpaTolerationGroups map[storage.ResourceIndex]string,
	index storage.ResourceIndex,
	object storage.StoreObject,
	errorList *errors.LintRuleErrorsList,
) {
	errorList = errorList.WithRule(r.GetName())

	kind, _, _ := unstructured.NestedString(object.Unstructured.Object, "spec", "targetRef", "kind")
	name, _, _ := unstructured.NestedString(object.Unstructured.Object, "spec", "targetRef", "name")

	if !r.Enabled(kind, name) {
		// TODO: add metrics
		return
	}

	tolerations, err := getTolerationsList(object)

	errorListObj := errorList.WithObjectID(object.Identity())

	if err != nil {
		errorListObj.Errorf("Get tolerations list for object failed: %v", err)
	}

	isTolerationFound := false
	for _, toleration := range tolerations {
		if toleration.Key == "node-role.kubernetes.io/master" || toleration.Key == "node-role.kubernetes.io/control-plane" || (toleration.Key == "" && toleration.Operator == "Exists") {
			isTolerationFound = true
			break
		}
	}

	workloadLabelValue := vpaTolerationGroups[index]

	errorListObjValue := errorListObj.WithValue(workloadLabelValue)

	if isTolerationFound && workloadLabelValue != "every-node" && workloadLabelValue != "master" {
		errorListObjValue.Error(`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource not found`)
	}

	if !isTolerationFound && workloadLabelValue != "" {
		errorListObjValue.Error(`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource found, but tolerations is not right`)
	}
}

func getTolerationsList(object storage.StoreObject) ([]v1.Toleration, error) {
	var tolerations []v1.Toleration

	converter := runtime.DefaultUnstructuredConverter

	switch object.Unstructured.GetKind() {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return nil, err
		}

		tolerations = deployment.Spec.Template.Spec.Tolerations

	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return nil, err
		}

		tolerations = daemonset.Spec.Template.Spec.Tolerations

	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return nil, err
		}

		tolerations = statefulset.Spec.Template.Spec.Tolerations
	}

	return tolerations, nil
}
