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

package config

import "github.com/deckhouse/dmt/pkg"

// ServicePortExcludeList represents a list of service port exclusions
type ServicePortExcludeList []ServicePortExclude

// Get converts ServicePortExcludeList to []pkg.ServicePortExclude
func (l ServicePortExcludeList) Get() []pkg.ServicePortExclude {
	result := make([]pkg.ServicePortExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapServicePortRuleExclude(&l[idx]))
	}

	return result
}

// StringRuleExcludeList represents a list of string exclusions
type StringRuleExcludeList []string

// Get converts StringRuleExcludeList to []pkg.StringRuleExclude
func (l StringRuleExcludeList) Get() []pkg.StringRuleExclude {
	result := make([]pkg.StringRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, pkg.StringRuleExclude(l[idx]))
	}

	return result
}

// PrefixRuleExcludeList represents a list of prefix exclusions
type PrefixRuleExcludeList []string

// Get converts PrefixRuleExcludeList to []pkg.PrefixRuleExclude
func (l PrefixRuleExcludeList) Get() []pkg.PrefixRuleExclude {
	result := make([]pkg.PrefixRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, pkg.PrefixRuleExclude(l[idx]))
	}

	return result
}

// KindRuleExcludeList represents a list of kind exclusions
type KindRuleExcludeList []KindRuleExclude

// Get converts KindRuleExcludeList to []pkg.KindRuleExclude
func (l KindRuleExcludeList) Get() []pkg.KindRuleExclude {
	result := make([]pkg.KindRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapKindRuleExclude(&l[idx]))
	}

	return result
}

// ContainerRuleExcludeList represents a list of container exclusions
type ContainerRuleExcludeList []ContainerRuleExclude

// Get converts ContainerRuleExcludeList to []pkg.ContainerRuleExclude
func (l ContainerRuleExcludeList) Get() []pkg.ContainerRuleExclude {
	result := make([]pkg.ContainerRuleExclude, 0, len(l))

	for idx := range l {
		result = append(result, *remapContainerRuleExclude(&l[idx]))
	}

	return result
}

// KindRuleExclude represents a kind-based rule exclusion
type KindRuleExclude struct {
	Kind string `mapstructure:"kind"`
	Name string `mapstructure:"name"`
}

// ContainerRuleExclude represents a container-based rule exclusion
type ContainerRuleExclude struct {
	Kind      string `mapstructure:"kind"`
	Name      string `mapstructure:"name"`
	Container string `mapstructure:"container"`
}

// ServicePortExclude represents a service port exclusion
type ServicePortExclude struct {
	Name string `mapstructure:"name"`
	Port string `mapstructure:"port"`
}

func remapKindRuleExclude(input *KindRuleExclude) *pkg.KindRuleExclude {
	return &pkg.KindRuleExclude{
		Name: input.Name,
		Kind: input.Kind,
	}
}

func remapServicePortRuleExclude(input *ServicePortExclude) *pkg.ServicePortExclude {
	return &pkg.ServicePortExclude{
		Name: input.Name,
		Port: input.Port,
	}
}

func remapContainerRuleExclude(input *ContainerRuleExclude) *pkg.ContainerRuleExclude {
	return &pkg.ContainerRuleExclude{
		Kind:      input.Kind,
		Name:      input.Name,
		Container: input.Container,
	}
}
