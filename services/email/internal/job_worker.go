package internal

import (
	"context"
	"planeo/libs/events"
	"planeo/libs/logger"
	"time"

	"github.com/rs/zerolog"
)

// JobWorker processes email check jobs from NATS queue
// Can be scaled horizontally - each job is processed by exactly one worker
type JobWorker struct {
	imapService  IMAPServiceInterface
	eventService events.EventServiceInterface
	logger       zerolog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewJobWorker(imapService IMAPServiceInterface, eventService events.EventServiceInterface) *JobWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobWorker{
		imapService:  imapService,
		eventService: eventService,
		logger:       logger.New("job-worker"),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins consuming email check jobs from NATS
func (w *JobWorker) Start(ctx context.Context) error {
	w.logger.Info().Msg("Starting job worker - listening for email check jobs")

	// Subscribe to job queue
	// NATS ensures each job goes to exactly one worker (work queue semantics)
	err := w.eventService.SubscribeEmailCheckJob(ctx, w.processJob)
	if err != nil {
		return err
	}

	w.logger.Info().Msg("Job worker started successfully")
	return nil
}

// processJob handles a single email check job
func (w *JobWorker) processJob(job events.EmailCheckJobPayload) error {
	start := time.Now()

	jobLogger := w.logger.With().
		Int("setting_id", job.SettingID).
		Int("organization_id", job.OrganizationID).
		Str("host", job.Host).
		Logger()

	ctx := logger.WithContext(context.Background(), jobLogger)

	jobLogger.Debug().Msg("Processing email check job")

	imapSettings := IMAPSettings{
		Host:     job.Host,
		Port:     job.Port,
		Username: job.Username,
		Password: job.Password,
	}

	// Fetch unseen emails
	emails, err := w.imapService.FetchAllUnseenMails(ctx, imapSettings)
	
	duration := time.Since(start)

	if err != nil {
		jobLogger.Error().
			Err(err).
			Dur("duration_ms", duration).
			Msg("Failed to fetch emails")
		return err // Will trigger retry via NAK
	}

	jobLogger.Info().
		Int("email_count", len(emails)).
		Dur("duration_ms", duration).
		Msg("Fetched emails successfully")

	// Publish each email as an event
	for _, email := range emails {
		emailLogger := jobLogger.With().Str("message_id", email.MessageID).Logger()

		emailLogger.Debug().Msg("Publishing email received event")

		err = w.eventService.PublishEmailReceived(ctx, events.EmailCreatedPayload{
			Subject:        email.Subject,
			Body:           email.Body,
			From:           email.From,
			Date:           email.Date,
			MessageID:      email.MessageID,
			OrganizationId: job.OrganizationID,
		})

		if err != nil {
			emailLogger.Error().
				Err(err).
				Msg("Failed to publish email received event")
			// Continue processing other emails even if one fails
		} else {
			emailLogger.Debug().Msg("Email published successfully")
		}
	}

	return nil
}

// Shutdown gracefully stops the worker
func (w *JobWorker) Shutdown() {
	w.logger.Info().Msg("Shutting down job worker")
	w.cancel()
}
