package exclusions

import (
	"fmt"

	"github.com/deckhouse/dmt/pkg"
)

func ExampleNewTrackedKindRule() {
	tracker := NewExclusionTracker()
	excludeRules := []pkg.KindRuleExclude{
		{Kind: "Deployment", Name: "test-deployment"},
		{Kind: "StatefulSet", Name: "test-statefulset"},
	}
	rule := NewTrackedRule(
		pkg.NewKindRuleWithTracker(excludeRules, tracker, "container", "dns-policy"),
		KindRuleKeys(excludeRules),
		tracker, "container", "dns-policy", "my-module",
	)

	fmt.Printf("Enabled for Deployment/test-deployment: %t\n", rule.Enabled("Deployment", "test-deployment"))
	fmt.Printf("Enabled for Deployment/other-deployment: %t\n", rule.Enabled("Deployment", "other-deployment"))
	stats := tracker.GetUsageStats()
	fmt.Printf("Usage stats: %+v\n", stats)
	unused := tracker.GetUnusedExclusions()
	fmt.Printf("Unused exclusions: %+v\n", unused)
	// Output:
	// Enabled for Deployment/test-deployment: false
	// Enabled for Deployment/other-deployment: true
	// Usage stats: map[container:map[dns-policy:map[Deployment/test-deployment:1]]]
	// Unused exclusions: map[container:map[dns-policy:[StatefulSet/test-statefulset]]]
}

func ExampleNewTrackedStringRule() {
	tracker := NewExclusionTracker()
	excludeRules := []pkg.StringRuleExclude{"skip-this-string", "another-skip"}
	rule := NewTrackedRule(
		pkg.NewStringRuleWithTracker(excludeRules, tracker, "rbac", "binding-subject"),
		StringRuleKeys(excludeRules),
		tracker, "rbac", "binding-subject", "my-module",
	)

	fmt.Printf("Enabled for 'skip-this-string': %t\n", rule.Enabled("skip-this-string"))
	fmt.Printf("Enabled for 'normal-string': %t\n", rule.Enabled("normal-string"))
	stats := tracker.GetUsageStats()
	fmt.Printf("Usage stats: %+v\n", stats)
	// Output:
	// Enabled for 'skip-this-string': false
	// Enabled for 'normal-string': true
	// Usage stats: map[rbac:map[binding-subject:map[skip-this-string:1]]]
}

func ExampleNewTrackedBoolRule() {
	tracker := NewExclusionTracker()
	rule := NewTrackedRule(
		pkg.NewBoolRuleWithTracker(true, tracker, "module", "conversions"),
		[]string{},
		tracker, "module", "conversions", "my-module",
	)

	fmt.Printf("Rule enabled: %t\n", rule.Enabled())
	stats := tracker.GetUsageStats()
	fmt.Printf("Usage stats: %+v\n", stats)
	// Output:
	// Rule enabled: false
	// Usage stats: map[module:map[conversions:map[disabled:1]]]
}
