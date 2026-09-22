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

package bootstrap

// wellKnownScopes is the scope of the built-in Kubernetes resources a module's RBAC commonly names,
// for the first declaration written from the render: the module ships no CRD for them and the
// linter has no cluster to ask. Anything not here is left without a scope for the author to fill.
var wellKnownScopes = map[string]string{
	// core
	"/pods": "Namespaced", "/pods/log": "Namespaced", "/pods/exec": "Namespaced", "/pods/portforward": "Namespaced", "/pods/proxy": "Namespaced", "/pods/status": "Namespaced",
	"/services": "Namespaced", "/services/proxy": "Namespaced", "/endpoints": "Namespaced", "/secrets": "Namespaced", "/configmaps": "Namespaced",
	"/serviceaccounts": "Namespaced", "/serviceaccounts/token": "Namespaced", "/events": "Namespaced", "/limitranges": "Namespaced", "/resourcequotas": "Namespaced",
	"/persistentvolumeclaims": "Namespaced", "/replicationcontrollers": "Namespaced", "/podtemplates": "Namespaced", "/bindings": "Namespaced",
	"/namespaces": "Cluster", "/nodes": "Cluster", "/nodes/proxy": "Cluster", "/nodes/status": "Cluster", "/persistentvolumes": "Cluster", "/componentstatuses": "Cluster",
	// apps, batch, autoscaling, policy
	"apps/deployments": "Namespaced", "apps/daemonsets": "Namespaced", "apps/statefulsets": "Namespaced", "apps/replicasets": "Namespaced", "apps/controllerrevisions": "Namespaced",
	"apps/deployments/scale": "Namespaced", "apps/statefulsets/scale": "Namespaced", "apps/replicasets/scale": "Namespaced",
	"batch/jobs": "Namespaced", "batch/cronjobs": "Namespaced",
	"autoscaling/horizontalpodautoscalers": "Namespaced", "policy/poddisruptionbudgets": "Namespaced",
	// networking, discovery, coordination, events
	"networking.k8s.io/ingresses": "Namespaced", "networking.k8s.io/networkpolicies": "Namespaced", "networking.k8s.io/ingressclasses": "Cluster",
	"discovery.k8s.io/endpointslices": "Namespaced", "coordination.k8s.io/leases": "Namespaced", "events.k8s.io/events": "Namespaced",
	// rbac, storage, scheduling, node, admission, apiextensions, apiregistration, certificates, flowcontrol
	"rbac.authorization.k8s.io/roles": "Namespaced", "rbac.authorization.k8s.io/rolebindings": "Namespaced",
	"rbac.authorization.k8s.io/clusterroles": "Cluster", "rbac.authorization.k8s.io/clusterrolebindings": "Cluster",
	"storage.k8s.io/storageclasses": "Cluster", "storage.k8s.io/volumeattachments": "Cluster", "storage.k8s.io/csidrivers": "Cluster", "storage.k8s.io/csinodes": "Cluster", "storage.k8s.io/csistoragecapacities": "Namespaced",
	"scheduling.k8s.io/priorityclasses": "Cluster", "node.k8s.io/runtimeclasses": "Cluster",
	"admissionregistration.k8s.io/mutatingwebhookconfigurations": "Cluster", "admissionregistration.k8s.io/validatingwebhookconfigurations": "Cluster",
	"admissionregistration.k8s.io/validatingadmissionpolicies": "Cluster", "admissionregistration.k8s.io/validatingadmissionpolicybindings": "Cluster",
	"apiextensions.k8s.io/customresourcedefinitions": "Cluster", "apiregistration.k8s.io/apiservices": "Cluster",
	"certificates.k8s.io/certificatesigningrequests": "Cluster",
	"flowcontrol.apiserver.k8s.io/flowschemas":       "Cluster", "flowcontrol.apiserver.k8s.io/prioritylevelconfigurations": "Cluster",
	// authentication / authorization (virtual, cluster-scoped)
	"authentication.k8s.io/tokenreviews": "Cluster", "authorization.k8s.io/subjectaccessreviews": "Cluster", "authorization.k8s.io/selfsubjectaccessreviews": "Cluster",
	"authorization.k8s.io/selfsubjectrulesreviews": "Cluster", "authorization.k8s.io/localsubjectaccessreviews": "Namespaced",
	// metrics
	"metrics.k8s.io/pods": "Namespaced", "metrics.k8s.io/nodes": "Cluster",
}
