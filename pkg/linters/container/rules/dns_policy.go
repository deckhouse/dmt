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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
)

const (
	DNSPolicyRuleName = "dns-policy"
)

func NewDNSPolicyRule(excludeRules []pkg.KindRuleExclude) *DNSPolicyRule {
	return &DNSPolicyRule{
		RuleMeta: pkg.RuleMeta{
			Name: DNSPolicyRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

func NewDNSPolicyRuleWithTracker(excludeRules []pkg.KindRuleExclude, tracker *exclusions.ExclusionTracker, linterID, ruleID, moduleName string) *DNSPolicyRule {
	// Register exclusions with tracker if provided
	if tracker != nil {
		exclusions := make([]string, len(excludeRules))
		for i, rule := range excludeRules {
			exclusions[i] = fmt.Sprintf("%s/%s", rule.Kind, rule.Name)
		}
		tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)
	}

	return &DNSPolicyRule{
		RuleMeta: pkg.RuleMeta{
			Name: DNSPolicyRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
		tracker:  tracker,
		linterID: linterID,
		ruleID:   ruleID,
	}
}

type DNSPolicyRule struct {
	pkg.RuleMeta
	pkg.KindRule
	tracker  *exclusions.ExclusionTracker
	linterID string
	ruleID   string
}

func (r *DNSPolicyRule) ObjectDNSPolicy(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	// Check if object should be excluded and track usage if tracker is available
	if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
		// Mark exclusion as used if tracker is available
		if r.tracker != nil {
			exclusionKey := fmt.Sprintf("%s/%s", object.Unstructured.GetKind(), object.Unstructured.GetName())
			r.tracker.MarkExclusionUsed(r.linterID, r.ruleID, exclusionKey)
		}
		// TODO: add metrics
		return
	}

	dnsPolicy, hostNetwork, err := getDNSPolicyAndHostNetwork(object)
	if err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	validateDNSPolicy(dnsPolicy, hostNetwork, object, errorList)
}

func getDNSPolicyAndHostNetwork(object storage.StoreObject) (string, bool, error) {
	converter := runtime.DefaultUnstructuredConverter

	var dnsPolicy string
	var hostNetwork bool
	var err error

	switch object.Unstructured.GetKind() {
	case "Deployment":
		deployment := new(appsv1.Deployment)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		dnsPolicy = string(deployment.Spec.Template.Spec.DNSPolicy)
		hostNetwork = deployment.Spec.Template.Spec.HostNetwork
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		dnsPolicy = string(daemonset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = daemonset.Spec.Template.Spec.HostNetwork
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		dnsPolicy = string(statefulset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = statefulset.Spec.Template.Spec.HostNetwork
	}

	return dnsPolicy, hostNetwork, err
}

func validateDNSPolicy(dnsPolicy string, hostNetwork bool, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if !hostNetwork {
		return
	}

	if dnsPolicy != "ClusterFirstWithHostNet" {
		errorList.WithObjectID(object.Identity()).WithValue(dnsPolicy).
			WithFilePath(object.ShortPath()).
			Error("dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`")
	}
}
