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

package rules

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ImagePullPolicyRuleName = "image-pull-policy"
)

func NewImagePullPolicyRule(objects []ObjectContainers, errorList *errors.LintRuleErrorsList) *ImagePullPolicyRule {
	return &ImagePullPolicyRule{
		RuleMeta: pkg.RuleMeta{
			Name: ImagePullPolicyRuleName,
		},
		objects:   objects,
		errorList: errorList.WithRule(ImagePullPolicyRuleName),
	}
}

type ImagePullPolicyRule struct {
	pkg.RuleMeta

	objects   []ObjectContainers
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*ImagePullPolicyRule)(nil)

func (r *ImagePullPolicyRule) Check(_ context.Context) {
	for _, oc := range r.objects {
		r.checkObject(oc.Object, oc.All)
	}
}

// checkObject must stay a separate method rather than being inlined into the
// loop above: its early returns end the check for one object, not for the rule.
func (r *ImagePullPolicyRule) checkObject(object storage.StoreObject, containers []corev1.Container) {
	errorList := r.errorList.WithFilePath(object.GetPath())

	if object.Unstructured.GetNamespace() == "d8-system" && object.Unstructured.GetKind() == "Deployment" && object.Unstructured.GetName() == "deckhouse" {
		checkImagePullPolicyAlways(object, containers, errorList)

		return
	}

	containerImagePullPolicyIfNotPresent(object, containers, errorList)
}

func checkImagePullPolicyAlways(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	c := containers[0]
	if c.ImagePullPolicy != corev1.PullAlways {
		errorList.WithObjectID(object.Identity() + "; container = " + c.Name).WithValue(c.ImagePullPolicy).
			Error(`Container imagePullPolicy should be unspecified or "Always"`)
	}
}

func containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]

		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}

		errorList.WithObjectID(object.Identity() + "; container = " + c.Name).WithValue(c.ImagePullPolicy).
			Error(`Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
}
