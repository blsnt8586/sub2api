package sub2api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// browserLoginResult Python 脚本返回的 JSON 结果
type browserLoginResult struct {
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error,omitempty"`
}

// BrowserLogin 通过 undetected-chromedriver 浏览器登录有 Turnstile 验证的平台。
// 调用 Python 脚本完成登录，返回 JWT token。
// 环境要求：python3、undetected-chromedriver、selenium（服务器还需 Xvfb）
func BrowserLogin(ctx context.Context, baseURL, email, password string) (string, error) {
	scriptPath, err := findLoginScript()
	if err != nil {
		return "", fmt.Errorf("browser login script not found: %w", err)
	}

	// 超时控制：浏览器登录最长 120 秒
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", scriptPath,
		"--url", baseURL,
		"--email", email,
		"--password", password,
		"--output", "json",
	)

	// 捕获 stdout（JSON 结果）和 stderr（日志）
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("browser login timeout (120s): %w", err)
		}
		return "", fmt.Errorf("browser login script failed: %w\nstderr: %s", err, stderr.String())
	}

	// 解析 JSON 输出
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", fmt.Errorf("browser login script produced no output\nstderr: %s", stderr.String())
	}

	// 找到最后一行 JSON（脚本可能有调试日志输出）
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			jsonLine = line
			break
		}
	}
	if jsonLine == "" {
		return "", fmt.Errorf("no JSON found in browser login output: %s", output)
	}

	var result browserLoginResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		return "", fmt.Errorf("parse browser login result: %w\noutput: %s", err, jsonLine)
	}

	if result.Error != "" {
		return "", fmt.Errorf("browser login failed: %s", result.Error)
	}

	if result.AuthToken == "" {
		return "", fmt.Errorf("browser login returned empty token")
	}

	return result.AuthToken, nil
}

// findLoginScript 查找 Python 登录脚本路径。
// 查找顺序：1. 环境变量 SUB2API_LOGIN_SCRIPT
//
//	2. 当前可执行文件旁的 scripts/browser_login.py
//	3. 项目根目录 scripts/browser_login.py
func findLoginScript() (string, error) {
	// 1. 环境变量优先
	if envPath := os.Getenv("SUB2API_LOGIN_SCRIPT"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
		return "", fmt.Errorf("SUB2API_LOGIN_SCRIPT=%s not found", envPath)
	}

	// 2. 可执行文件旁
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), "scripts", "browser_login.py")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 3. 源码 scripts/ 目录（开发环境）
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		// thisFile = .../backend/internal/pkg/sub2api/browser_login.go
		// 向上4级到项目根，再找 scripts/
		projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..")
		candidate := filepath.Clean(filepath.Join(projectRoot, "scripts", "browser_login.py"))
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("browser_login.py not found; set SUB2API_LOGIN_SCRIPT env or place scripts/browser_login.py next to binary")
}
