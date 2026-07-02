package sub2api

import (
	"context"
	"fmt"
)

// PathDetector API 路径探测器
type PathDetector struct {
	client *Client
}

// NewPathDetector 创建路径探测器
func NewPathDetector(client *Client) *PathDetector {
	return &PathDetector{client: client}
}

// PathsResult 路径探测结果
type PathsResult struct {
	KeysPath   string `json:"keys_path"`
	GroupsPath string `json:"groups_path"`
}

// 常见的 API Keys 路径
var commonKeysPaths = []string{
	"/api/v1/keys",
	"/api/v1/admin/keys",
	"/api/keys",
	"/keys",
	"/api/v1/apikeys",
	"/api/v1/admin/apikeys",
}

// 常见的 Groups 路径
var commonGroupsPaths = []string{
	"/api/v1/groups/available",
	"/api/v1/groups",
	"/api/v1/admin/groups",
	"/api/groups",
	"/groups",
}

// DetectKeysPath 探测 API Keys 路径
func (d *PathDetector) DetectKeysPath(ctx context.Context) (string, error) {
	for _, path := range commonKeysPaths {
		_, err := d.client.GetAPIKeys(ctx, path)
		if err == nil {
			// 请求成功，找到正确的路径
			return path, nil
		}
		// 继续尝试下一个路径
	}

	return "", fmt.Errorf("failed to detect keys path: tried %d paths", len(commonKeysPaths))
}

// DetectGroupsPath 探测 Groups 路径
func (d *PathDetector) DetectGroupsPath(ctx context.Context) (string, error) {
	for _, path := range commonGroupsPaths {
		_, err := d.client.GetGroups(ctx, path)
		if err == nil {
			// 请求成功，找到正确的路径
			return path, nil
		}
		// 继续尝试下一个路径
	}

	return "", fmt.Errorf("failed to detect groups path: tried %d paths", len(commonGroupsPaths))
}

// DetectAllPaths 探测所有 API 路径
func (d *PathDetector) DetectAllPaths(ctx context.Context) (*PathsResult, error) {
	result := &PathsResult{}

	// 探测 Keys 路径
	keysPath, err := d.DetectKeysPath(ctx)
	if err != nil {
		return nil, fmt.Errorf("detect keys path: %w", err)
	}
	result.KeysPath = keysPath

	// 探测 Groups 路径
	groupsPath, err := d.DetectGroupsPath(ctx)
	if err != nil {
		// Groups 路径是可选的，失败不影响整体
		// 有些 Sub2API 实例可能没有 Groups 功能
		result.GroupsPath = ""
	} else {
		result.GroupsPath = groupsPath
	}

	return result, nil
}

// TestConnection 测试连接（仅登录）
func (d *PathDetector) TestConnection(ctx context.Context) error {
	return d.client.Login(ctx)
}
