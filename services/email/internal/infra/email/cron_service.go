// services/email/internal/infra/email/cron_service.go
package email

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
	return &CronService{scheduler: scheduler}
}

func (s *CronService) Start() {
	s.scheduler.Start()
}

func (s *CronService) Stop() {
	_ = s.scheduler.StopJobs()
}

func (s *CronService) RemoveJob(id uuid.UUID) error {
	return s.scheduler.RemoveJob(id)
}

func (s *CronService) AddJob(ctx context.Context, task func(), fetchInterval time.Duration, tags []string) {
	_, err := s.scheduler.NewJob(
		gocron.DurationJob(fetchInterval),
		gocron.NewTask(task),
		gocron.WithTags(tags...),
		gocron.WithEventListeners(
			gocron.AfterJobRunsWithError(func(jobID uuid.UUID, jobName string, err error) {
				l := logger.FromContext(ctx)
				l.Error().Msgf("Job with id %s has failed: %s", jobID.String(), err.Error())
			}),
		),
	)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Msgf("Error scheduling job: %s", err.Error())
	}
}

func (s *CronService) RemoveJobByTag(ctx context.Context, tag string) {
	l := logger.FromContext(ctx)
	l.Info().Msgf("Removing jobs with tag: %s", tag)
	s.scheduler.RemoveByTags(tag)
}
