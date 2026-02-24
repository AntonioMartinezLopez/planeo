package internal

import (
	"context"
	"planeo/services/email/internal/resources/settings/models"
)

// EmailSchedulerService adapts JobPublisher and IMAPService for the settings service
// This provides the same interface as the old EmailService but uses NATS jobs instead
type EmailSchedulerService struct {
	jobPublisher *JobPublisher
	imapService  IMAPServiceInterface
}

func NewEmailSchedulerService(jobPublisher *JobPublisher, imapService IMAPServiceInterface) *EmailSchedulerService {
	return &EmailSchedulerService{
		jobPublisher: jobPublisher,
		imapService:  imapService,
	}
}

func (s *EmailSchedulerService) StartFetching(ctx context.Context, settings []models.Setting) error {
	for _, setting := range settings {
		s.jobPublisher.AddAccount(ctx, setting)
	}
	return nil
}

func (s *EmailSchedulerService) StopFetching(ctx context.Context, settingsId int) {
	s.jobPublisher.RemoveAccount(settingsId)
}

func (s *EmailSchedulerService) TestConnection(ctx context.Context, setting models.Setting) error {
	return s.imapService.TestConnection(ctx, IMAPSettings{
		Host:     setting.Host,
		Port:     setting.Port,
		Username: setting.Username,
		Password: setting.Password,
	})
}
