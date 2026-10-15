package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Sandbox provides path validation to restrict operations inside a specific directory.
type Sandbox struct {
	Workspace string
}

// NewSandbox 建立沙盒，並將 workspace 正規化為展開 symlink 後的實體路徑。
//
// workspace 必須與 CheckPath 的展開結果位於同一命名空間，否則只要 workspace
// 路徑含有任何一段 symlink（例如 macOS 的 /var → private/var、或指向家目錄的
// 連結），展開後的合法路徑就會因為字串前綴不符而被誤判為逃出沙盒。
func NewSandbox(workspacePath string) (*Sandbox, error) {
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, err
	}
	absPath = filepath.Clean(absPath)

	// 權限 0700：workspace 內含對話歷史與人格檔，且 pkg/config 會對過於寬鬆的
	// workspace 權限發出警告——建立時就該給它期望的權限
	if err := os.MkdirAll(absPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace path %s: %w", absPath, err)
	}

	return &Sandbox{
		Workspace: resolved,
	}, nil
}

// pathContained 判斷 target 是否等於 workspace，或是它的子路徑。
//
// foldCase 為 true 時，前綴比對與逐層回溯共用同一份小寫字串。Windows 的
// 路徑比對不區分大小寫，但回傳給呼叫端的路徑必須維持原始大小寫；因此
// 小寫只存在於這份比較用副本，不能寫回原始路徑。
func pathContained(workspace, target string, foldCase bool) bool {
	cmpWorkspace, cmpTarget := workspace, target
	if foldCase {
		cmpWorkspace = strings.ToLower(workspace)
		cmpTarget = strings.ToLower(target)
	}

	if cmpTarget != cmpWorkspace && !strings.HasPrefix(cmpTarget, cmpWorkspace+string(filepath.Separator)) {
		return false
	}

	checkPath := cmpTarget
	for {
		if checkPath == cmpWorkspace {
			return true
		}
		parent := filepath.Dir(checkPath)
		if parent == checkPath {
			return false
		}
		checkPath = parent
	}
}

// CheckPath resolves input path to absolute and ensures it's inside Workspace.
func (s *Sandbox) CheckPath(inputPath string) (string, error) {
	var targetPath string
	if filepath.IsAbs(inputPath) {
		targetPath = inputPath
	} else {
		targetPath = filepath.Join(s.Workspace, inputPath)
	}

	absTargetPath, err := resolvePath(targetPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	// Windows 檔案系統不區分大小寫。比較時才折疊，回傳仍用 resolvePath 的原始大小寫。
	if !pathContained(s.Workspace, absTargetPath, runtime.GOOS == "windows") {
		return "", fmt.Errorf("path escapes workspace bounds: %s", inputPath)
	}

	return absTargetPath, nil
}

// resolvePath 展開路徑中「已存在的最長前綴」的 symlink，再拼回尚未存在的部分。
//
// filepath.EvalSymlinks 對不存在的目標會回 ENOENT；此時若直接退回 filepath.Clean，
// 等於對不存在的目標跳過 symlink 展開，讓 write_file 這類建立檔案的操作可以穿過
// workspace/leak -> <外部目錄> 的連結寫到沙盒外。因此改為逐層往上找出第一個
// 存在的祖先展開後再接回剩餘片段，讓「已存在」與「不存在」的目標走同一套判斷。
func resolvePath(path string) (string, error) {
	clean := filepath.Clean(path)

	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	remainder := []string{filepath.Base(clean)}
	current := filepath.Dir(clean)

	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(append([]string{resolved}, remainder...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			// 連根目錄都不存在（理論上不會發生），退回字面路徑
			return clean, nil
		}
		remainder = append([]string{filepath.Base(current)}, remainder...)
		current = parent
	}
}
