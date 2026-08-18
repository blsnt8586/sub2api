package service

import "context"

// 二开扩展方法的 mock 实现，从 ratelimit_session_window_test.go 拆出，
// 以隔离上游同步冲突（上游改动 sessionWindowMockRepo 时不再碰到这里）。
// 这些方法在会话窗口相关测试中不会被调用，故统一 panic。

func (m *sessionWindowMockRepo) ClearProviderLink(context.Context, int64, int64) error {
	panic("unexpected")
}
func (m *sessionWindowMockRepo) UpdateProviderLink(context.Context, int64, int64, int64) error {
	panic("unexpected")
}
func (m *sessionWindowMockRepo) UpdateRemoteGroupInfo(context.Context, int64, string, float64) error {
	panic("unexpected")
}
func (m *sessionWindowMockRepo) ListByProviderID(context.Context, int64) ([]Account, error) {
	panic("unexpected")
}
func (m *sessionWindowMockRepo) UpdateSub2APIOptimizeSettings(context.Context, int64, int64, bool, *float64, *float64, *string) error {
	panic("unexpected")
}
