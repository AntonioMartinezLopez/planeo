package postgres

import (
	"context"
	"errors"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/email/internal/domain/setting"

	"github.com/jackc/pgx/v5"
)

func (c *Client) GetAllSettings(ctx context.Context) ([]setting.Setting, error) {
	rows, err := c.db.Query(ctx, `SELECT * FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings, err := pgx.CollectRows(rows, pgx.RowToStructByName[setting.Setting])
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Msg("Error collecting rows")
		return nil, err
	}
	return settings, nil
}

func (c *Client) GetSettings(ctx context.Context, organizationId int) ([]setting.Setting, error) {
	args := pgx.NamedArgs{"organizationId": organizationId}
	rows, err := c.db.Query(ctx, `SELECT * FROM settings WHERE organization_id = @organizationId`, args)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "GetSettings").Msg("Error querying database")
		return nil, err
	}
	defer rows.Close()

	settings, err := pgx.CollectRows(rows, pgx.RowToStructByName[setting.Setting])
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "GetSettings").Msg("Error collecting rows")
		return nil, err
	}
	return settings, nil
}

func (c *Client) CreateSetting(ctx context.Context, s setting.NewSetting) (setting.Setting, error) {
	query := `
		INSERT INTO settings (host, port, username, password, organization_id)
		VALUES (@host, @port, @username, @password, @organizationId)
		RETURNING *`
	args := pgx.NamedArgs{
		"host":           s.Host,
		"port":           s.Port,
		"username":       s.Username,
		"password":       s.Password,
		"organizationId": s.OrganizationID,
	}

	row, err := c.db.Query(ctx, query, args)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "CreateSetting").Msg("Error executing query")
		return setting.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	created, err := pgx.CollectExactlyOneRow(row, pgx.RowToStructByName[setting.Setting])
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "CreateSetting").Msg("Error collecting row")
		return setting.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}
	return created, nil
}

func (c *Client) UpdateSetting(ctx context.Context, s setting.UpdateSetting) (setting.Setting, error) {
	query := `
		UPDATE settings SET host = @host, port = @port, username = @username, password = @password
		WHERE id = @settingId AND organization_id = @organizationId
		RETURNING *`
	args := pgx.NamedArgs{
		"settingId":      s.ID,
		"host":           s.Host,
		"port":           s.Port,
		"username":       s.Username,
		"password":       s.Password,
		"organizationId": s.OrganizationID,
	}

	row, err := c.db.Query(ctx, query, args)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "UpdateSetting").Msg("Error executing query")
		return setting.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	updated, err := pgx.CollectExactlyOneRow(row, pgx.RowToStructByName[setting.Setting])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return setting.Setting{}, appError.New(appError.EntityNotFound, "could not find setting", err)
		}
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "UpdateSetting").Msg("Error collecting row")
		return setting.Setting{}, appError.New(appError.InternalError, "Something went wrong", err)
	}
	return updated, nil
}

func (c *Client) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	args := pgx.NamedArgs{"settingId": settingId, "organizationId": organizationId}
	_, err := c.db.Exec(ctx, `DELETE FROM settings WHERE id = @settingId AND organization_id = @organizationId`, args)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Error().Err(err).Str("operation", "DeleteSetting").Msg("Error executing query")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}
	return nil
}
