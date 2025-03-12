package internal

import (
	"planeo/libs/logger"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type CronService struct {
	scheduler gocron.Scheduler
	logger    zerolog.Logger
}

func NewCronService() *CronService {

	scheduler, err := gocron.NewScheduler(
		gocron.WithLimitConcurrentJobs(20, gocron.LimitModeWait),
	)

	logger := logger.New("cron-service")

	if err != nil {
		panic(err)
	}

	return &CronService{
		scheduler: scheduler,
		logger:    logger,
	}
}

func (s *CronService) AddJob(task func(), fetchInterval time.Duration, tags []string) {

	_, err := s.scheduler.NewJob(
		gocron.DurationJob(fetchInterval),
		gocron.NewTask(task),
		gocron.WithTags(tags...),
		gocron.WithEventListeners(
			gocron.AfterJobRunsWithError(
				func(jobID uuid.UUID, jobName string, err error) {
					s.logger.Error().Msgf("Job with id %s has failed: %s", jobID.String(), err.Error())
				},
			),
		),
	)

	if err != nil {
		s.logger.Fatal().Msgf("Error scheduling job: %s", err.Error())
	}
}

func (s *CronService) Start() {
	s.scheduler.Start()
}

func (s *CronService) Stop() {
	s.scheduler.StopJobs()
}

func (s *CronService) RemoveJob(id uuid.UUID) error {
	return s.scheduler.RemoveJob(id)
}

func (s *CronService) RemoveJobByTag(tag string) {
	s.logger.Info().Msgf("Removing jobs with tag: %s", tag)
	s.scheduler.RemoveByTags(tag)
}
