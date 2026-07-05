package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiprovider"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Sub2APIProviderRepository 处理 Provider 的数据访问
type Sub2APIProviderRepository struct {
	client *ent.Client
}

// NewSub2APIProviderRepository 创建 Repository 实例
func NewSub2APIProviderRepository(client *ent.Client) service.Sub2APIProviderRepository {
	return &Sub2APIProviderRepository{client: client}
}

// Create 创建 Provider
func (r *Sub2APIProviderRepository) Create(ctx context.Context, input *service.CreateSub2APIProviderInput) (*ent.Sub2APIProvider, error) {
	builder := r.client.Sub2APIProvider.Create().
		SetName(input.Name).
		SetBaseURL(input.BaseURL).
		SetEmail(input.Email).
		SetPasswordEncrypted(input.Password)

	// 上游类型：显式指定时写入，未指定则由 ent schema 默认值（sub2api）兜底
	if input.ProviderType != "" {
		builder = builder.SetProviderType(input.ProviderType)
	}

	// 登录方式：显式指定时写入，未指定则由 ent schema 默认值（http）兜底
	if input.LoginMethod != "" {
		builder = builder.SetLoginMethod(input.LoginMethod)
	}

	if input.Notes != nil {
		builder = builder.SetNotes(*input.Notes)
	}

	return builder.Save(ctx)
}

// GetByID 根据 ID 获取 Provider
func (r *Sub2APIProviderRepository) GetByID(ctx context.Context, id int64) (*ent.Sub2APIProvider, error) {
	return r.client.Sub2APIProvider.Query().
		Where(
			sub2apiprovider.ID(id),
			sub2apiprovider.DeletedAtIsNil(),
		).
		First(ctx)
}

// GetByIDWithAccounts 根据 ID 获取 Provider（包含关联的 Accounts）
func (r *Sub2APIProviderRepository) GetByIDWithAccounts(ctx context.Context, id int64) (*ent.Sub2APIProvider, error) {
	return r.client.Sub2APIProvider.Query().
		Where(
			sub2apiprovider.ID(id),
			sub2apiprovider.DeletedAtIsNil(),
		).
		WithAccounts().
		First(ctx)
}

// List 列出所有 Provider（支持过滤和分页）
func (r *Sub2APIProviderRepository) List(ctx context.Context, filters *service.Sub2APIProviderFilters, page, pageSize int) ([]*ent.Sub2APIProvider, int, error) {
	query := r.client.Sub2APIProvider.Query().
		Where(sub2apiprovider.DeletedAtIsNil())

	// 应用过滤条件
	if filters != nil {
		if filters.Status != "" {
			query = query.Where(sub2apiprovider.Status(filters.Status))
		}

		if filters.Search != "" {
			query = query.Where(
				sub2apiprovider.Or(
					sub2apiprovider.NameContains(filters.Search),
					sub2apiprovider.BaseURLContains(filters.Search),
					sub2apiprovider.EmailContains(filters.Search),
				),
			)
		}
	}

	// 计算总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	// eager-load 关联账号，仅用于计数（只 select 账号 ID，避免拉取大字段）
	providers, err := query.
		Order(ent.Desc(sub2apiprovider.FieldCreatedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		WithAccounts(func(q *ent.AccountQuery) {
			q.Select(dbaccount.FieldID)
		}).
		All(ctx)

	return providers, total, err
}

// ListAll 列出所有 Provider（不分页，用于下拉选择等场景）
func (r *Sub2APIProviderRepository) ListAll(ctx context.Context, filters *service.Sub2APIProviderFilters) ([]*ent.Sub2APIProvider, error) {
	query := r.client.Sub2APIProvider.Query().
		Where(sub2apiprovider.DeletedAtIsNil())

	if filters != nil {
		if filters.Status != "" {
			query = query.Where(sub2apiprovider.Status(filters.Status))
		}
	}

	return query.
		Order(ent.Asc(sub2apiprovider.FieldName)).
		All(ctx)
}

// Update 更新 Provider
func (r *Sub2APIProviderRepository) Update(ctx context.Context, id int64, input *service.UpdateSub2APIProviderInput) (*ent.Sub2APIProvider, error) {
	update := r.client.Sub2APIProvider.UpdateOneID(id)

	if input.Name != nil {
		update = update.SetName(*input.Name)
	}
	if input.BaseURL != nil {
		update = update.SetBaseURL(*input.BaseURL)
	}
	if input.Email != nil {
		update = update.SetEmail(*input.Email)
	}
	if input.Password != nil {
		update = update.SetPasswordEncrypted(*input.Password)
	}
	if input.LoginMethod != nil {
		update = update.SetLoginMethod(*input.LoginMethod)
	}
	if input.Status != nil {
		update = update.SetStatus(*input.Status)
	}
	if input.Notes != nil {
		if *input.Notes == "" {
			update = update.ClearNotes()
		} else {
			update = update.SetNotes(*input.Notes)
		}
	}

	return update.Save(ctx)
}

// Delete 软删除 Provider
func (r *Sub2APIProviderRepository) Delete(ctx context.Context, id int64) error {
	return r.client.Sub2APIProvider.UpdateOneID(id).
		SetDeletedAt(time.Now()).
		Exec(ctx)
}

// UpdateSyncStatus 更新同步状态
func (r *Sub2APIProviderRepository) UpdateSyncStatus(
	ctx context.Context,
	id int64,
	status string,
	errorMsg *string,
) error {
	update := r.client.Sub2APIProvider.UpdateOneID(id).
		SetLastSyncAt(time.Now()).
		SetLastSyncStatus(status)

	if errorMsg != nil {
		update = update.SetLastSyncError(*errorMsg)
	} else {
		update = update.ClearLastSyncError()
	}

	return update.Exec(ctx)
}

// UpdateAPIPaths 更新 API 路径配置
func (r *Sub2APIProviderRepository) UpdateAPIPaths(
	ctx context.Context,
	id int64,
	keysPath, groupsPath string,
) error {
	return r.client.Sub2APIProvider.UpdateOneID(id).
		SetAPIPathKeys(keysPath).
		SetAPIPathGroups(groupsPath).
		Exec(ctx)
}
