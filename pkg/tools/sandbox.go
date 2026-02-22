package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Sandbox provides path validation to restrict operations inside a specific directory.
type Sandbox struct {
	Workspace string
}

func NewSandbox(workspacePath string) (*Sandbox, error) {
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, err
	}
	return &Sandbox{
		Workspace: filepath.Clean(absPath),
	}, nil
}

// CheckPath resolves input path to absolute and ensures it's inside Workspace.
func (s *Sandbox) CheckPath(inputPath string) (string, error) {
	// Support absolute path resolving relative to workspace if given as relative
	var targetPath string
	if filepath.IsAbs(inputPath) {
		targetPath = inputPath
	} else {
		targetPath = filepath.Join(s.Workspace, inputPath)
	}

	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}
	absTargetPath = filepath.Clean(absTargetPath)

	// Ensure prefix matches
	// Using strings.HasPrefix is okay, but `filepath.Clean` might leave trailing slash issue.
	// Best approach: ensure absTargetPath equals Workspace OR absTargetPath starts with Workspace + separator
	if !strings.HasPrefix(absTargetPath, s.Workspace) {
		return "", fmt.Errorf("path escapes workspace bounds: %s", inputPath)
	}

	return absTargetPath, nil
}
