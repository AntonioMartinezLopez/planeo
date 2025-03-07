package internal

import (
	"context"
	"planeo/libs/logger"
	"planeo/services/email/internal/resources/settings/models"
	"strconv"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

type SettingsRepositoryInterface interface {
	GetAllSettings(ctx context.Context) ([]models.Setting, error)
}

type CronService struct {
	settingsRepository SettingsRepositoryInterface
	scheduler          gocron.Scheduler
}

func NewCronService(settingsRepository SettingsRepositoryInterface) *CronService {

	scheduler, err := gocron.NewScheduler(
		gocron.WithLimitConcurrentJobs(20, gocron.LimitModeWait),
	)

	if err != nil {
		panic(err)
	}

	return &CronService{
		settingsRepository: settingsRepository,
		scheduler:          scheduler,
	}
}

func (s *CronService) Start() {

	// Get all settings
	settings, err := s.settingsRepository.GetAllSettings(context.Background())

	if err != nil {
		logger.Fatal("Error retrieving settings: %s", err.Error())
	}
	logger.Info("Settings retrieved: %d", len(settings))

	// Schedule jobs
	for _, setting := range settings {
		_, err := s.scheduler.NewJob(
			gocron.DurationJob(10*time.Second),
			gocron.NewTask(func() {
				logger.Info("Running job for setting: %d", setting.ID)
			}),
			gocron.WithEventListeners(
				gocron.BeforeJobRuns(func(jobID uuid.UUID, jobName string) {
					logger.Info("Job with id %s is about to run", jobID.String())
				}),
				gocron.AfterJobRunsWithError(
					func(jobID uuid.UUID, jobName string, err error) {
						logger.Error("Job with id %s has failed: %s", jobID.String(), err.Error())
					},
				),
			),
			gocron.WithTags(strconv.Itoa(setting.ID)),
		)

		if err != nil {
			logger.Fatal("Error scheduling job: %s", err.Error())
		}
	}

	s.scheduler.Start()
}
