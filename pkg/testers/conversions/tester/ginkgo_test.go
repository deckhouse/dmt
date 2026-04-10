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

package tester_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/dmt/pkg/testers/conversions/tester"
)

func TestConversionsTesterGinkgo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conversions Tester Ginkgo Suite")
}

var _ = Describe("Conversions Tester", func() {
	var (
		testDataPath = filepath.Join("..", "..", "..", "..", "testdata")
	)

	Describe("module-with-passing-tests", func() {
		modulePath := filepath.Join(testDataPath, "module-with-passing-tests")

		It("should pass all testcases", func() {
			t := tester.New()
			err := t.Run(modulePath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("module-with-failing-tests", func() {
		modulePath := filepath.Join(testDataPath, "module-with-failing-tests")

		It("should fail with incorrect expected error and show YAML diff", func() {
			t := tester.New()
			err := t.Run(modulePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("incorrect expected - should fail"))
			Expect(err.Error()).To(ContainSubstring("Test expects"))
			Expect(err.Error()).To(ContainSubstring("Conversion produced"))
			Expect(err.Error()).To(ContainSubstring("password: secret"))
		})
	})

	Describe("module-with-version-mismatch", func() {
		modulePath := filepath.Join(testDataPath, "module-with-version-mismatch")

		It("should fail with version mismatch error showing file context", func() {
			t := tester.New()
			err := t.Run(modulePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("x-config-version mismatch"))
			Expect(err.Error()).To(ContainSubstring("x-config-version 3"))
			Expect(err.Error()).To(ContainSubstring("latest conversion version is 2"))
		})
	})

	Describe("module-with-conversions", func() {
		modulePath := filepath.Join(testDataPath, "module-with-conversions")

		It("should pass without errors", func() {
			t := tester.New()
			err := t.Run(modulePath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("module-without-testcases", func() {
		modulePath := filepath.Join(testDataPath, "module-without-testcases")

		It("should be not applicable", func() {
			t := tester.New()
			err := t.Run(modulePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("testcases.yaml is missing"))
		})
	})

	Describe("with temp dir setup", func() {
		Describe("version mismatch error formatting", func() {
			It("should show expected vs actual YAML with diff", func() {
				tmpDir := GinkgoT().TempDir()

				openapiDir := filepath.Join(tmpDir, "openapi")
				err := os.MkdirAll(openapiDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				configValuesYAML := `x-config-version: 2`
				err = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte(configValuesYAML), 0644)
				Expect(err).NotTo(HaveOccurred())

				convDir := filepath.Join(openapiDir, "conversions")
				err = os.MkdirAll(convDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				v2yaml := `version: 2
conversions:
  - del(.auth.password)
description:
  ru: "v2"
  en: "v2"
`
				err = os.WriteFile(filepath.Join(convDir, "v2.yaml"), []byte(v2yaml), 0644)
				Expect(err).NotTo(HaveOccurred())

				testcasesYAML := `testcases:
  - name: "failing test with diff output"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
        extra: value
    expected: |
      auth:
        password: secret
`
				err = os.WriteFile(filepath.Join(convDir, "testcases.yaml"), []byte(testcasesYAML), 0644)
				Expect(err).NotTo(HaveOccurred())

				t := tester.New()
				err = t.Run(tmpDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failing test with diff output"))
				Expect(err.Error()).To(ContainSubstring("Test expects"))
			})
		})
	})
})
