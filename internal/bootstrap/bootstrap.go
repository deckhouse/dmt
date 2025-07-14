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

package bootstrap

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
)

const (
	RepositoryTypeGitHub = "github"
	RepositoryTypeGitLab = "gitlab"

	ModuleTemplateURL = "https://github.com/deckhouse/modules-template/archive/refs/heads/main.zip"

	// HTTP timeout for downloads
	downloadTimeout = 30 * time.Second

	// File permissions
	filePermissions = 0600
	dirPermissions  = 0755

	// Maximum file size to prevent DoS attacks (10MB)
	maxFileSize = 10 * 1024 * 1024
)

// BootstrapConfig holds configuration for bootstrap process
type BootstrapConfig struct {
	ModuleName     string
	RepositoryType string
	RepositoryURL  string
	Directory      string
}

// RunBootstrap initializes a new module with the given configuration
func RunBootstrap(config BootstrapConfig) error {
	logger.InfoF("Bootstrap type: %s", config.RepositoryType)

	// if config.Directory does not exist, create it
	if _, err := os.Stat(config.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(config.Directory, dirPermissions); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Check if directory is empty
	if err := checkDirectoryEmpty(config.Directory); err != nil {
		return fmt.Errorf("directory validation failed: %w", err)
	}

	// Download and extract template
	if err := downloadAndExtractTemplate(config); err != nil {
		return fmt.Errorf("template download/extraction failed: %w", err)
	}

	// Get current moduleName from module.yaml file
	currentModuleName, err := getModuleName(config.Directory)
	if err != nil {
		return fmt.Errorf("failed to get module name: %w", err)
	}

	// Replace all strings like `.Values.currentModuleName` with `.Values.moduleName`
	if err := replaceValuesModuleName(currentModuleName, config.ModuleName, config.Directory); err != nil {
		return fmt.Errorf("failed to replace values module name: %w", err)
	}

	// Replace all strings like `currentModuleName` with `moduleName`
	if err := replaceModuleName(currentModuleName, config.ModuleName, config.Directory); err != nil {
		return fmt.Errorf("failed to replace module name: %w", err)
	}

	switch config.RepositoryType {
	case RepositoryTypeGitLab:
		if err := os.RemoveAll(filepath.Join(config.Directory, ".github")); err != nil {
			return fmt.Errorf("failed to remove .github directory: %w", err)
		}
	case RepositoryTypeGitHub:
		if err := os.RemoveAll(filepath.Join(config.Directory, ".gitlab-ci.yml")); err != nil {
			return fmt.Errorf("failed to remove .gitlab-ci.yml file: %w", err)
		}
	}

	logger.InfoF("Bootstrap completed successfully")

	switch config.RepositoryType {
	case RepositoryTypeGitHub:
		fmt.Println()
		fmt.Println("Don't forget to add secrets to your GitHub repository:")
		fmt.Println("  - DECKHOUSE_PRIVATE_REPO")
		fmt.Println("  - DEFECTDOJO_API_TOKEN")
		fmt.Println("  - DEFECTDOJO_HOST")
		fmt.Println("  - DEV_MODULES_REGISTRY_PASSWORD")
		fmt.Println("  - GOPROXY")
		fmt.Println("  - PROD_MODULES_READ_REGISTRY_PASSWORD")
		fmt.Println("  - PROD_MODULES_REGISTRY_PASSWORD")
		fmt.Println("  - SOURCE_REPO")
		fmt.Println("  - SOURCE_REPO_SSH_KEY")
	case RepositoryTypeGitLab:
		fmt.Println()
		fmt.Println("Don't forget to modify variables to your .gitlab-ci.yml file:")
		fmt.Println("  - MODULES_MODULE_NAME")
		fmt.Println("  - MODULES_REGISTRY")
		fmt.Println("  - MODULES_MODULE_SOURCE")
		fmt.Println("  - MODULES_MODULE_TAG")
		fmt.Println("  - WERF_VERSION")
		fmt.Println("  - BASE_IMAGES_VERSION")
	}

	return nil
}

// checkDirectoryEmpty checks if the directory is empty
// and returns an error if it's not empty
func checkDirectoryEmpty(directory string) error {
	if directory != "" {
		currentDir, err := fsutils.ExpandDir(directory)
		if err != nil {
			return fmt.Errorf("failed to expand directory: %w", err)
		}
		directory = currentDir
	} else {
		currentDir, err := fsutils.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		directory = currentDir
	}

	files := fsutils.GetFiles(directory, false)
	if len(files) > 0 {
		return fmt.Errorf("directory is not empty. Please run bootstrap in an empty directory")
	}

	logger.InfoF("Directory is empty, proceeding with bootstrap")
	return nil
}

// replaceModuleName replaces all occurrences of currentModuleName with newModuleName in files
func replaceModuleName(currentModuleName, newModuleName, directory string) error {
	files := fsutils.GetFiles(directory, true, func(_, _ string) bool {
		return true
	})

	for _, file := range files {
		if err := replaceInFile(file, currentModuleName, newModuleName); err != nil {
			return fmt.Errorf("failed to replace in file %s: %w", file, err)
		}
	}

	return nil
}

// replaceValuesModuleName replaces .Values.currentModuleName patterns with .Values.newModuleName
// and currentModuleName.internal patterns with newModuleName.internal (in camelCase)
func replaceValuesModuleName(currentModuleName, newModuleName, directory string) error {
	files := fsutils.GetFiles(directory, true, func(_, _ string) bool {
		return true
	})

	camelCaseNewName := strcase.ToLowerCamel(newModuleName)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}

		// Replace .Values.currentModuleName with .Values.newModuleName (camelCase)
		oldPattern := fmt.Sprintf(".Values.%s", currentModuleName)
		newPattern := fmt.Sprintf(".Values.%s", camelCaseNewName)
		newContent := strings.ReplaceAll(string(content), oldPattern, newPattern)

		// Replace currentModuleName.internal with newModuleName.internal (camelCase)
		oldPattern = fmt.Sprintf("%s.internal", currentModuleName)
		newPattern = fmt.Sprintf("%s.internal", camelCaseNewName)
		newContent = strings.ReplaceAll(newContent, oldPattern, newPattern)

		if err := os.WriteFile(file, []byte(newContent), filePermissions); err != nil {
			return fmt.Errorf("failed to write file %s: %w", file, err)
		}
	}

	return nil
}

// replaceInFile replaces oldString with newString in a single file
func replaceInFile(filePath, oldString, newString string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	newContent := strings.ReplaceAll(string(content), oldString, newString)
	if err := os.WriteFile(filePath, []byte(newContent), filePermissions); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// getModuleName extracts the module name from module.yaml file
func getModuleName(directory string) (string, error) {
	moduleYamlPath := filepath.Join(directory, "module.yaml")
	moduleYaml, err := os.ReadFile(moduleYamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read module.yaml: %w", err)
	}

	var module struct {
		Name string `yaml:"name"`
	}

	if err := yaml.Unmarshal(moduleYaml, &module); err != nil {
		return "", fmt.Errorf("failed to unmarshal module.yaml: %w", err)
	}
	return module.Name, nil
}

// downloadAndExtractTemplate downloads the template zip file and extracts it to current directory
func downloadAndExtractTemplate(config BootstrapConfig) error {
	repositoryURL := ModuleTemplateURL
	if config.RepositoryURL != "" {
		repositoryURL = config.RepositoryURL
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "dmt-bootstrap-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download zip file
	zipPath := filepath.Join(tempDir, "template.zip")
	if err := downloadFile(repositoryURL, zipPath); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	// Extract zip file
	if err := extractZip(zipPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract template: %w", err)
	}

	// Move extracted content to current directory
	if err := moveExtractedContent(tempDir, config.Directory); err != nil {
		return fmt.Errorf("failed to move extracted content: %w", err)
	}

	return nil
}

// downloadFile downloads a file from URL to local path with timeout
func downloadFile(url, targetPath string) error {
	logger.InfoF("Downloading template from: %s", url)

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file, status: %d", resp.StatusCode)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Limit the size of the downloaded file to prevent DoS attacks
	limitedReader := io.LimitReader(resp.Body, maxFileSize)
	_, err = io.Copy(file, limitedReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.InfoF("Template downloaded successfully")
	return nil
}

// extractZip extracts a zip file to the specified directory
func extractZip(zipPath, extractDir string) error {
	logger.InfoF("Extracting template archive")

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Find the root directory name (usually the first directory)
	var rootDir string
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			rootDir = file.Name
			break
		}
	}

	if rootDir == "" {
		return fmt.Errorf("no root directory found in zip file")
	}

	// Extract files
	for _, file := range reader.File {
		// Skip the root directory itself
		if file.Name == rootDir {
			continue
		}

		// Create relative path by removing root directory prefix
		relativePath := strings.TrimPrefix(file.Name, rootDir+"/")
		if relativePath == "" {
			continue
		}

		filePath := filepath.Join(extractDir, relativePath)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, dirPermissions); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(filePath), dirPermissions); err != nil {
			return fmt.Errorf("failed to create parent directories for %s: %w", filePath, err)
		}

		// Create file
		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}

		// Open zip file
		zipFile, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip file entry %s: %w", file.Name, err)
		}

		// Copy content with size limit to prevent DoS attacks
		limitedReader := io.LimitReader(zipFile, maxFileSize)
		_, err = io.Copy(outFile, limitedReader)
		zipFile.Close()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to copy file content %s: %w", filePath, err)
		}

		if err := outFile.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", filePath, err)
		}
	}

	logger.InfoF("Template extracted successfully")
	return nil
}

// moveExtractedContent moves extracted content from temp directory to directory
func moveExtractedContent(tempDir, directory string) error {
	// Find the single directory (template) inside tempDir
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %w", err)
	}

	var templateDir string
	for _, entry := range entries {
		if entry.IsDir() {
			templateDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if templateDir == "" {
		return fmt.Errorf("template directory not found in temp directory")
	}

	// Move only the contents of templateDir to currentDir
	templateEntries, err := os.ReadDir(templateDir)
	if err != nil {
		return fmt.Errorf("failed to read template directory: %w", err)
	}

	for _, entry := range templateEntries {
		srcPath := filepath.Join(templateDir, entry.Name())
		dstPath := filepath.Join(directory, entry.Name())

		if err := moveFileOrDirectory(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to move %s to current directory: %w", entry.Name(), err)
		}
	}

	logger.InfoF("Template files moved to current directory")
	return nil
}

// moveFileOrDirectory moves a file or directory with fallback to copy-and-remove
// when os.Rename fails (e.g., across different filesystems)
func moveFileOrDirectory(src, dst string) error {
	// Try direct rename first
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If rename fails, fall back to copy-and-remove approach
	return copyAndRemove(src, dst)
}

// copyAndRemove copies a file or directory and then removes the original
func copyAndRemove(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if info.IsDir() {
		return copyDirectoryAndRemove(src, dst)
	}
	return copyFileAndRemove(src, dst)
}

// copyDirectoryAndRemove recursively copies a directory and removes the original
func copyDirectoryAndRemove(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, dirPermissions); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirectoryAndRemove(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy subdirectory %s: %w", entry.Name(), err)
			}
		} else {
			if err := copyFileAndRemove(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", entry.Name(), err)
			}
		}
	}

	// Remove the original directory
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("failed to remove original directory: %w", err)
	}

	return nil
}

// copyFileAndRemove copies a file and removes the original
func copyFileAndRemove(src, dst string) error {
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(dst), dirPermissions); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy content with size limit to prevent DoS attacks
	limitedReader := io.LimitReader(srcFile, maxFileSize)
	if _, err := io.Copy(dstFile, limitedReader); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Preserve file permissions
	if err := dstFile.Chmod(filePermissions); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Remove the original file
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove original file: %w", err)
	}

	return nil
}
