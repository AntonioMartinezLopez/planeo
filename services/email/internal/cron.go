package internal

import (
	"context"
	"planeo/libs/logger"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

type CronService struct {
	scheduler gocron.Scheduler
}

func NewCronService() *CronService {

	scheduler, err := gocron.NewScheduler(
		gocron.WithLimitConcurrentJobs(20, gocron.LimitModeWait),
	)

	if err != nil {
		panic(err)
	}

	return &CronService{
		scheduler: scheduler,
	}
}

func (s *CronService) AddJob(ctx context.Context, task func(), fetchInterval time.Duration, tags []string) {

	_, err := s.scheduler.NewJob(
		gocron.DurationJob(fetchInterval),
		gocron.NewTask(task),
		gocron.WithTags(tags...),
		gocron.WithEventListeners(
			gocron.AfterJobRunsWithError(
				func(jobID uuid.UUID, jobName string, err error) {
					logger := logger.FromContext(ctx)
					logger.Error().Msgf("Job with id %s has failed: %s", jobID.String(), err.Error())
				},
			),
		),
	)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Fatal().Msgf("Error scheduling job: %s", err.Error())
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

func (s *CronService) RemoveJobByTag(ctx context.Context, tag string) {
	logger := logger.FromContext(ctx)
	logger.Info().Msgf("Removing jobs with tag: %s", tag)
	s.scheduler.RemoveByTags(tag)
}
