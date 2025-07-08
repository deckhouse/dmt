#!/bin/bash

# Script to add license header to Go files that don't have it

LICENSE_HEADER='/*
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

'

# Find all Go files without any license block
# Check for any Copyright block (not just specific text)
FILES_WITHOUT_LICENSE=""
for file in $(find . -name "*.go" -type f); do
    # Check if file has any Copyright block
    if ! grep -q "Copyright" "$file"; then
        FILES_WITHOUT_LICENSE="$FILES_WITHOUT_LICENSE $file"
    fi
done

echo "Found $(echo "$FILES_WITHOUT_LICENSE" | wc -w) files without any license header"

for file in $FILES_WITHOUT_LICENSE; do
    if [ -f "$file" ]; then
        echo "Adding license to: $file"
        # Create temporary file with license + original content
        echo -n "$LICENSE_HEADER" >"${file}.tmp"
        cat "$file" >>"${file}.tmp"
        mv "${file}.tmp" "$file"
    fi
done

echo "License headers added to Go files without any existing license"
