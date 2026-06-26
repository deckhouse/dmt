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
	"testing"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

func makeStorage(kind, name string, annotations map[string]string) map[storage.ResourceIndex]storage.StoreObject {
	u := unstructured.Unstructured{}
	u.SetKind(kind)
	u.SetName(name)
	u.SetAnnotations(annotations)

	return map[storage.ResourceIndex]storage.StoreObject{
		{Kind: kind, Name: name}: {
			Unstructured: u,
			AbsPath:      "/test/" + name + ".yaml",
		},
	}
}

func TestWebhookConfigurationRule_BothAnnotationsPresent(t *testing.T) {
	mc := minimock.NewController(t)

	storage := makeStorage("ValidatingWebhookConfiguration", "my-webhook", map[string]string{
		"werf.io/weight":           "10",
		"werf.io/deploy-dependency": "some-dep",
	})

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	rule := NewWebhookConfigurationRule(nil)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.Empty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_OnlyWeight(t *testing.T) {
	mc := minimock.NewController(t)

	storage := makeStorage("ValidatingWebhookConfiguration", "my-webhook", map[string]string{
		"werf.io/weight": "10",
	})

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	rule := NewWebhookConfigurationRule(nil)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.Empty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_OnlyDeployDependency(t *testing.T) {
	mc := minimock.NewController(t)

	storage := makeStorage("ValidatingWebhookConfiguration", "my-webhook", map[string]string{
		"werf.io/deploy-dependency": "dep-a",
	})

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	rule := NewWebhookConfigurationRule(nil)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.Empty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_NeitherAnnotation(t *testing.T) {
	mc := minimock.NewController(t)

	storage := makeStorage("ValidatingWebhookConfiguration", "my-webhook", map[string]string{
		"other-annotation": "value",
	})

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	rule := NewWebhookConfigurationRule(nil)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.NotEmpty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_MutatingWebhookConfiguration(t *testing.T) {
	mc := minimock.NewController(t)

	storage := makeStorage("MutatingWebhookConfiguration", "my-hook", map[string]string{
		"werf.io/weight": "5",
	})

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	rule := NewWebhookConfigurationRule(nil)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.Empty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_SkipsNonWebhookResources(t *testing.T) {
	mc := minimock.NewController(t)

	u := unstructured.Unstructured{}
	u.SetKind("Deployment")
	u.SetName("my-deploy")

	storage := map[storage.ResourceIndex]storage.StoreObject{
		{Kind: "Deployment", Name: "my-deploy"}: {
			Unstructured: u,
			AbsPath:      "/test/deploy.yaml",
		},
	}

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	rule := NewWebhookConfigurationRule(nil)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.Empty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_ExcludedResource(t *testing.T) {
	mc := minimock.NewController(t)

	storage := makeStorage("ValidatingWebhookConfiguration", "excluded-hook", map[string]string{})

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(storage)

	exclude := []pkg.KindRuleExclude{
		{Kind: "ValidatingWebhookConfiguration", Name: "excluded-hook"},
	}
	rule := NewWebhookConfigurationRule(exclude)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.Empty(t, errorList.GetErrors())
}

func TestWebhookConfigurationRule_ExcludedResourceDoesNotAffectOthers(t *testing.T) {
	mc := minimock.NewController(t)

	store := makeStorage("ValidatingWebhookConfiguration", "excluded-hook", nil)
	store[storage.ResourceIndex{Kind: "MutatingWebhookConfiguration", Name: "other-hook"}] = func() storage.StoreObject {
		u := unstructured.Unstructured{}
		u.SetKind("MutatingWebhookConfiguration")
		u.SetName("other-hook")
		return storage.StoreObject{Unstructured: u, AbsPath: "/test/other.yaml"}
	}()

	mod := mocks.NewModuleMock(mc)
	mod.GetStorageMock.Return(store)

	exclude := []pkg.KindRuleExclude{
		{Kind: "ValidatingWebhookConfiguration", Name: "excluded-hook"},
	}
	rule := NewWebhookConfigurationRule(exclude)
	errorList := errors.NewLintRuleErrorsList()

	rule.ValidateWebhookConfigurationAnnotations(mod, errorList)
	assert.NotEmpty(t, errorList.GetErrors())
}
