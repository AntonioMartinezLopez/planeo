package settings

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/email/internal/resources/settings/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepository struct {
	db *pgxpool.Pool
}

func NewSettingsRepository(database *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{
		db: database,
	}
}

func (repo *SettingsRepository) GetAllSettings(ctx context.Context) ([]models.Setting, error) {
	query := `SELECT * FROM settings`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Setting])
	if err != nil {
		logger.Error("Error collecting row: %s", err.Error())
		return nil, err
	}

	return settings, nil
}

func (repo *SettingsRepository) GetSettings(ctx context.Context, organizationId int) ([]models.Setting, error) {
	query := `
	SELECT * FROM settings 
	WHERE organization_id = @organizationId`

	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := repo.db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Setting])
	if err != nil {
		logger.Error("Error collecting row: %s", err.Error())
		return nil, err
	}

	return settings, nil
}

func (repo *SettingsRepository) CreateSetting(ctx context.Context, setting models.Setting) error {
	query := `
	INSERT INTO settings (host, port, username, password, organization_id) 
	VALUES (@host, @port, @username, @password, @organizationId)`

	args := pgx.NamedArgs{
		"host":           setting.Host,
		"port":           setting.Port,
		"username":       setting.Username,
		"password":       setting.Password,
		"organizationId": setting.OrganizationID,
	}

	_, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *SettingsRepository) UpdateSetting(ctx context.Context, setting models.Setting) error {
	query := `
	UPDATE settings SET host = @host, port = @port, username = @username, password = @password 
	WHERE id = @settingId AND organization_id = @organizationId`

	args := pgx.NamedArgs{
		"settingId":      setting.ID,
		"host":           setting.Host,
		"port":           setting.Port,
		"username":       setting.Username,
		"password":       setting.Password,
		"organizationId": setting.OrganizationID,
	}

	_, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *SettingsRepository) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	query := `
	DELETE FROM settings 
	WHERE id = @settingId AND organization_id = @organizationId`

	args := pgx.NamedArgs{"settingId": settingId, "organizationId": organizationId}

	_, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}
