package server

import (
	"errors"
	humaUtils "planeo/libs/huma_utils"
	"planeo/services/email/internal/domain/setting"

	"github.com/danielgtaylor/huma/v2"
)

func NewHTTPError(err error) huma.StatusError {
	if errors.Is(err, setting.ErrSettingNotFound) {
		return huma.Error404NotFound("setting not found")
	}
	return humaUtils.NewHumaError(err)
}
