/*
Copyright 2026 Flant JSC

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
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	SysCgroupMountRuleName = "sys-cgroup-mount"

	// sysMountPath is the host sysfs mount whose presence triggers the rule.
	sysMountPath = "/sys"
	// sysCgroupMountPath is the cgroup mount that must accompany a /sys mount.
	sysCgroupMountPath = "/sys/fs/cgroup"
)

func NewSysCgroupMountRule(excludeRules []pkg.ContainerRuleExclude,
	objects []ObjectContainers, errorList *errors.LintRuleErrorsList) *SysCgroupMountRule {
	return &SysCgroupMountRule{
		RuleMeta: pkg.RuleMeta{
			Name: SysCgroupMountRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
		objects:   objects,
		errorList: errorList.WithRule(SysCgroupMountRuleName),
	}
}

type SysCgroupMountRule struct {
	pkg.RuleMeta
	pkg.ContainerRule

	objects   []ObjectContainers
	errorList *errors.LintRuleErrorsList
}

var _ pkg.Rule = (*SysCgroupMountRule)(nil)

func (r *SysCgroupMountRule) Check(_ context.Context) {
	for _, oc := range r.objects {
		r.checkObject(oc.Object, oc.All)
	}
}

// checkObject verifies that every container mounting the host /sys also
// mounts /sys/fs/cgroup.
//
// Rationale: on a hardened containerd (integrity checks + full read-only root, as
// used by the CSE edition) the recursive bind that normally carries the nested
// /sys/fs/cgroup submount along with /sys does not apply, so a container that
// relies on cgroup data has to mount /sys/fs/cgroup explicitly. The extra
// read-only mount is harmless on ordinary runtimes (there the submount is already
// present), so requiring it keeps modules portable to the CSE edition by default.
//
// The rule is self-scoping: it only fires for containers that actually mount /sys.
// The check is per container, because mounts are per-container — the very
// container that mounts /sys must also mount /sys/fs/cgroup. It stays a separate
// method rather than being inlined into Check: its early return ends the check
// for one object, not for the rule.
func (r *SysCgroupMountRule) checkObject(object storage.StoreObject, containers []corev1.Container) {
	errorList := r.errorList.WithFilePath(object.ShortPath())

	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			continue
		}

		var hasSys, hasCgroup bool

		for j := range c.VolumeMounts {
			switch strings.TrimRight(c.VolumeMounts[j].MountPath, "/") {
			case sysMountPath:
				hasSys = true
			case sysCgroupMountPath:
				hasCgroup = true
			}
		}

		if hasSys && !hasCgroup {
			errorList.WithObjectID(object.Identity()+" ; container = "+c.Name).
				WithValue(sysCgroupMountPath).
				Errorf("Container %q mounts %q but not %q: on a hardened (read-only) containerd the nested cgroup mount is not propagated together with %q, so %q must be mounted explicitly", c.Name, sysMountPath, sysCgroupMountPath, sysMountPath, sysCgroupMountPath)
		}
	}
}
