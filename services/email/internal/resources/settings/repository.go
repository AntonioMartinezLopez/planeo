package settings

import (
	"context"
	"errors"
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
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetSettings").Msg("Error querying database")
		return nil, err
	}
	defer rows.Close()

	settings, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Setting])
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetSettings").Msg("Error collecting row")
		return nil, err
	}

	return settings, nil
}

func (repo *SettingsRepository) CreateSetting(ctx context.Context, setting models.NewSetting) (models.Setting, error) {
	query := `
	INSERT INTO settings (host, port, username, password, organization_id) 
	VALUES (@host, @port, @username, @password, @organizationId)
	RETURNING *`

	args := pgx.NamedArgs{
		"host":           setting.Host,
		"port":           setting.Port,
		"username":       setting.Username,
		"password":       setting.Password,
		"organizationId": setting.OrganizationID,
	}

	row, err := repo.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "CreateSetting").Msg("Error executing query")
		return models.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	createdSetting, err := pgx.CollectExactlyOneRow(row, pgx.RowToStructByName[models.Setting])
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "CreateSetting").Msg("Error collecting row")
		return models.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return createdSetting, nil
}

func (repo *SettingsRepository) UpdateSetting(ctx context.Context, setting models.UpdateSetting) (models.Setting, error) {
	query := `
	UPDATE settings SET host = @host, port = @port, username = @username, password = @password 
	WHERE id = @settingId AND organization_id = @organizationId
	RETURNING *`

	args := pgx.NamedArgs{
		"settingId":      setting.ID,
		"host":           setting.Host,
		"port":           setting.Port,
		"username":       setting.Username,
		"password":       setting.Password,
		"organizationId": setting.OrganizationID,
	}

	row, err := repo.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "UpdateSetting").Msg("Error executing query")
		return models.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	updatedSetting, err := pgx.CollectExactlyOneRow(row, pgx.RowToStructByName[models.Setting])
	if err != nil {
		logger := logger.FromContext(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Setting{}, appError.New(appError.EntityNotFound, "could not find setting", err)
		}
		logger.Error().Err(err).Str("operation", "UpdatingSetting").Msg("Error collecting row")
		return models.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return updatedSetting, nil
}

func (repo *SettingsRepository) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	query := `
	DELETE FROM settings 
	WHERE id = @settingId AND organization_id = @organizationId`

	args := pgx.NamedArgs{"settingId": settingId, "organizationId": organizationId}

	_, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "DeleteSetting").Msg("Error executing query")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}
