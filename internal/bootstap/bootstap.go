package bootstap

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
)

const (
	RepositoryTypeGitHub = "github"
	RepositoryTypeGitLab = "gitlab"

	ModuleTemplateURL = "https://github.com/deckhouse/modules-template/archive/refs/heads/main.zip"
)

func RunBootstrap(moduleName string, repositoryType string, repositoryURL string, directory string) error {
	logger.InfoF("Bootstrap type: %s", repositoryType)

	// Check if current directory is empty
	if err := checkDirectoryEmpty(directory); err != nil {
		return err
	}

	// Download and extract template
	if err := downloadAndExtractTemplate(directory); err != nil {
		return err
	}

	// Get current moduleName from module.yaml file
	currentModuleName, err := getModuleName(directory)
	if err != nil {
		return err
	}

	// Replace all strings like `.Values.currentModuleName` with `.Values.moduleName`
	if err := replaceValuesModuleName(currentModuleName, moduleName, directory); err != nil {
		return err
	}

	// Replace all strings like `currentModuleName` with `moduleName`
	if err := replaceModuleName(currentModuleName, moduleName, directory); err != nil {
		return err
	}

	logger.InfoF("Bootstrap completed successfully")
	return nil
}

// checkDirectoryEmpty checks if the directory is empty
// and exits with error if it's not empty
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
		logger.ErrorF("Directory is not empty. Please run bootstrap in an empty directory.")
		os.Exit(1)
	}

	logger.InfoF("Directory is empty, proceeding with bootstrap")
	return nil
}

// Replace all strings like `currentModuleName` with `newModuleName`
func replaceModuleName(currentModuleName, newModuleName, directory string) error {
	files := fsutils.GetFiles(directory, true, func(_, _ string) bool {
		return true
	})

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		newContent := strings.ReplaceAll(string(content), currentModuleName, newModuleName)
		if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	return nil
}

// Find all strings like `.Values.currentModuleName` in files and replace them with `.Values.newModuleName`
// find all strings like `.currentModuleName.internal` in files and replace them with `.newModuleName.internal`
// newModuleName is currentModuleName but lowerCamelCase
func replaceValuesModuleName(currentModuleName, newModuleName, directory string) error {
	files := fsutils.GetFiles(directory, true, func(_, _ string) bool {
		return true
	})

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		oldPattern := fmt.Sprintf(".Values.%s", currentModuleName)
		newPattern := fmt.Sprintf(".Values.%s", strcase.ToLowerCamel(newModuleName))
		newContent := strings.ReplaceAll(string(content), oldPattern, newPattern)

		oldPattern = fmt.Sprintf("%s.internal", currentModuleName)
		newPattern = fmt.Sprintf("%s.internal", strcase.ToLowerCamel(newModuleName))
		newContent = strings.ReplaceAll(newContent, oldPattern, newPattern)

		if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	return nil
}

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
func downloadAndExtractTemplate(directory string) error {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "dmt-bootstrap-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download zip file
	zipPath := filepath.Join(tempDir, "template.zip")
	if err := downloadFile(ModuleTemplateURL, zipPath); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	// Extract zip file
	if err := extractZip(zipPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract template: %w", err)
	}

	// Move extracted content to current directory
	if err := moveExtractedContent(tempDir, directory); err != nil {
		return fmt.Errorf("failed to move extracted content: %w", err)
	}

	return nil
}

// downloadFile downloads a file from URL to local path
func downloadFile(url, filepath string) error {
	logger.InfoF("Downloading template from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file, status: %d", resp.StatusCode)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
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
			if err := os.MkdirAll(filePath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
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

		// Copy content
		_, err = io.Copy(outFile, zipFile)
		zipFile.Close()
		outFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy file content %s: %w", filePath, err)
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

		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to move %s to current directory: %w", entry.Name(), err)
		}
	}

	logger.InfoF("Template files moved to current directory")
	return nil
}
