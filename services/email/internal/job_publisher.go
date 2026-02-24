package internal

import (
	"context"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/email/internal/resources/settings/models"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// JobPublisher schedules email checks by publishing jobs to NATS
// This is a lightweight scheduler - the actual work happens in JobWorker
type JobPublisher struct {
	eventService events.EventServiceInterface
	settings     map[int]*scheduledAccount
	logger       zerolog.Logger
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

type scheduledAccount struct {
	setting  models.Setting
	ticker   *time.Ticker
	stopChan chan struct{}
}

func NewJobPublisher(eventService events.EventServiceInterface) *JobPublisher {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobPublisher{
		eventService: eventService,
		settings:     make(map[int]*scheduledAccount),
		logger:       logger.New("job-publisher"),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// AddAccount starts scheduling jobs for an email account
func (p *JobPublisher) AddAccount(ctx context.Context, setting models.Setting) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop existing schedule if any
	if existing, ok := p.settings[setting.ID]; ok {
		existing.ticker.Stop()
		close(existing.stopChan)
	}

	// Create new schedule
	interval := p.calculateInterval(setting)
	ticker := time.NewTicker(interval)
	stopChan := make(chan struct{})

	scheduled := &scheduledAccount{
		setting:  setting,
		ticker:   ticker,
		stopChan: stopChan,
	}

	p.settings[setting.ID] = scheduled

	// Start publishing jobs
	go p.publishLoop(setting, ticker, stopChan)

	p.logger.Info().
		Int("setting_id", setting.ID).
		Int("organization_id", setting.OrganizationID).
		Dur("interval", interval).
		Msg("Started publishing jobs for account")
}

// RemoveAccount stops scheduling jobs for an email account
func (p *JobPublisher) RemoveAccount(settingID int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if scheduled, ok := p.settings[settingID]; ok {
		scheduled.ticker.Stop()
		close(scheduled.stopChan)
		delete(p.settings, settingID)

		p.logger.Info().
			Int("setting_id", settingID).
			Msg("Stopped publishing jobs for account")
	}
}

func (p *JobPublisher) publishLoop(setting models.Setting, ticker *time.Ticker, stopChan chan struct{}) {
	// Publish immediately on start
	p.publishJob(setting)

	for {
		select {
		case <-ticker.C:
			p.publishJob(setting)

			// Dynamically adjust interval based on activity
			// TODO: Implement adaptive intervals based on email volume
			// newInterval := p.calculateInterval(setting)
			// ticker.Reset(newInterval)

		case <-stopChan:
			return

		case <-p.ctx.Done():
			return
		}
	}
}

func (p *JobPublisher) publishJob(setting models.Setting) {
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	payload := events.EmailCheckJobPayload{
		SettingID:      setting.ID,
		OrganizationID: setting.OrganizationID,
		Host:           setting.Host,
		Port:           setting.Port,
		Username:       setting.Username,
		Password:       setting.Password,
	}

	err := p.eventService.PublishEmailCheckJob(ctx, payload)
	if err != nil {
		p.logger.Error().
			Err(err).
			Int("setting_id", setting.ID).
			Msg("Failed to publish email check job")
		return
	}

	p.logger.Debug().
		Int("setting_id", setting.ID).
		Int("organization_id", setting.OrganizationID).
		Msg("Published email check job")
}

// calculateInterval determines how often to check an account
// TODO: Make this dynamic based on account activity metrics
func (p *JobPublisher) calculateInterval(setting models.Setting) time.Duration {
	// For now, use a fixed interval
	// In production, you'd adjust based on:
	// - Historical email volume
	// - Time of day patterns
	// - Organization priority tier
	return 10 * time.Second
}

// Shutdown gracefully stops all scheduling
func (p *JobPublisher) Shutdown() {
	p.logger.Info().Msg("Shutting down job publisher")
	p.cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	for id, scheduled := range p.settings {
		scheduled.ticker.Stop()
		close(scheduled.stopChan)
		p.logger.Debug().Int("setting_id", id).Msg("Stopped account scheduler")
	}

	p.settings = make(map[int]*scheduledAccount)
}
