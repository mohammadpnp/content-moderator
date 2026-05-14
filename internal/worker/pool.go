package worker

import (
	"context"
	"sync"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/mohammadpnp/content-moderator/internal/pkg/metrics"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

type Config struct {
	WorkerCount int
	RateLimit   float64
	RateBurst   int
	JobTimeout  time.Duration
}

func DefaultConfig() Config {
	return Config{
		WorkerCount: 20,
		RateLimit:   5,
		RateBurst:   2,
		JobTimeout:  30 * time.Second,
	}
}

type Pool struct {
	config Config
	modSvc inbound.ModerationService
	broker outbound.MessageBroker
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewPool(cfg Config, modSvc inbound.ModerationService, broker outbound.MessageBroker) *Pool {
	return &Pool{
		config: cfg,
		modSvc: modSvc,
		broker: broker,
	}
}

func (p *Pool) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)
	defer p.cancel()

	limiter := rate.NewLimiter(rate.Limit(p.config.RateLimit), p.config.RateBurst)
	jobCh := make(chan *entity.Content, 1000)

	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i, limiter, jobCh)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics.JobQueueLength.Set(float64(len(jobCh)))
			}
		}
	}()

	err := p.broker.SubscribeModerationJobs(ctx, func(content *entity.Content) error {
		select {
		case jobCh <- content:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		return err
	}

	<-ctx.Done()
	log.Info().Msg("Shutting down worker pool...")
	close(jobCh)
	p.wg.Wait()
	log.Info().Msg("All workers exited")
	return nil
}

func (p *Pool) worker(ctx context.Context, id int, limiter *rate.Limiter, jobs <-chan *entity.Content) {
	defer p.wg.Done()
	log.Debug().Int("worker_id", id).Msg("worker started")

	for content := range jobs {
		if err := limiter.Wait(ctx); err != nil {
			log.Debug().Int("worker_id", id).Err(err).Msg("rate limit wait cancelled")
			return
		}

		jobCtx, jobCancel := context.WithTimeout(ctx, p.config.JobTimeout)
		p.processJob(jobCtx, content)
		jobCancel()
	}
	log.Debug().Int("worker_id", id).Msg("worker stopped")
}

func (p *Pool) processJob(ctx context.Context, content *entity.Content) {
	log.Debug().Str("content_id", content.ID).Str("type", string(content.Type)).Msg("worker processing content")

	result, err := p.modSvc.ModerateContent(ctx, content.ID)
	if err != nil {
		log.Error().Err(err).Str("content_id", content.ID).Msg("ERROR moderating content")
		return
	}

	if err := p.modSvc.HandleModerationResult(ctx, result); err != nil {
		log.Error().Err(err).Str("content_id", content.ID).Msg("ERROR handling moderation result")
	}
}
