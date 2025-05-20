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

package module

import (
	"testing"
)

func TestAddWordBoundariesToNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no numbers",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "with numbers",
			input:    "abc123def",
			expected: "abc 123 def",
		},
		{
			name:     "with numbers no trailing letter",
			input:    "abc123",
			expected: "abc 123 ",
		},
		{
			name:     "with numbers no leading letter",
			input:    "123def",
			expected: "123def",
		},
		{
			name:     "multiple numbers",
			input:    "abc123def456ghi",
			expected: "abc 123 def 456 ghi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addWordBoundariesToNumbers(tt.input)
			if result != tt.expected {
				t.Errorf("addWordBoundariesToNumbers(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToCamelInitCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		initCase bool
		expected string
	}{
		{
			name:     "empty string with init case true",
			input:    "",
			initCase: true,
			expected: "",
		},
		{
			name:     "empty string with init case false",
			input:    "",
			initCase: false,
			expected: "",
		},
		{
			name:     "simple string with init case true",
			input:    "hello world",
			initCase: true,
			expected: "HelloWorld",
		},
		{
			name:     "simple string with init case false",
			input:    "hello world",
			initCase: false,
			expected: "helloWorld",
		},
		{
			name:     "with underscores init case true",
			input:    "hello_world",
			initCase: true,
			expected: "HelloWorld",
		},
		{
			name:     "with hyphens init case false",
			input:    "hello-world",
			initCase: false,
			expected: "helloWorld",
		},
		{
			name:     "with dots init case true",
			input:    "hello.world",
			initCase: true,
			expected: "HelloWorld",
		},
		{
			name:     "with numbers init case true",
			input:    "hello123world",
			initCase: true,
			expected: "Hello123World",
		},
		{
			name:     "mixed case init case false",
			input:    "HelloWorld",
			initCase: false,
			expected: "HelloWorld",
		},
		{
			name:     "with spaces and uppercase init case true",
			input:    "hello WORLD",
			initCase: true,
			expected: "HelloWORLD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toCamelInitCase(tt.input, tt.initCase)
			if result != tt.expected {
				t.Errorf("toCamelInitCase(%q, %v) = %q, expected %q", tt.input, tt.initCase, result, tt.expected)
			}
		})
	}
}

func TestToCamel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple string",
			input:    "hello world",
			expected: "HelloWorld",
		},
		{
			name:     "with underscores",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "with hyphens",
			input:    "hello-world",
			expected: "HelloWorld",
		},
		{
			name:     "with dots",
			input:    "hello.world",
			expected: "HelloWorld",
		},
		{
			name:     "already camel case",
			input:    "HelloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "ID special case",
			input:    "ID",
			expected: "Id",
		},
		{
			name:     "complex string with separators",
			input:    "hello-world_example.test",
			expected: "HelloWorldExampleTest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCamel(tt.input)
			if result != tt.expected {
				t.Errorf("ToCamel(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToLowerCamel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple string",
			input:    "hello world",
			expected: "helloWorld",
		},
		{
			name:     "with underscores",
			input:    "hello_world",
			expected: "helloWorld",
		},
		{
			name:     "with hyphens",
			input:    "hello-world",
			expected: "helloWorld",
		},
		{
			name:     "with dots",
			input:    "hello.world",
			expected: "helloWorld",
		},
		{
			name:     "already camel case with capital first letter",
			input:    "HelloWorld",
			expected: "helloWorld",
		},
		{
			name:     "already lower camel case",
			input:    "helloWorld",
			expected: "helloWorld",
		},
		{
			name:     "ID special case",
			input:    "ID",
			expected: "id",
		},
		{
			name:     "complex string with separators",
			input:    "hello-world_example.test",
			expected: "helloWorldExampleTest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLowerCamel(tt.input)
			if result != tt.expected {
				t.Errorf("ToLowerCamel(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
