package setting

import "context"

type Repository interface {
	GetAllSettings(ctx context.Context) ([]Setting, error)
	GetSettings(ctx context.Context, organizationId int) ([]Setting, error)
	CreateSetting(ctx context.Context, setting NewSetting) (Setting, error)
	UpdateSetting(ctx context.Context, setting UpdateSetting) (Setting, error)
	DeleteSetting(ctx context.Context, organizationId int, settingId int) error
}

type EmailFetcher interface {
	StartFetching(ctx context.Context, settings []Setting) error
	StopFetching(ctx context.Context, settingId int)
	TestConnection(ctx context.Context, settings Setting) error
}

type Service interface {
	GetSettings(ctx context.Context, organizationId int) ([]Setting, error)
	CreateSetting(ctx context.Context, setting NewSetting) error
	UpdateSetting(ctx context.Context, setting UpdateSetting) error
	DeleteSetting(ctx context.Context, organizationId int, settingId int) error
	TestConnection(ctx context.Context, setting Setting) error
}
