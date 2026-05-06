package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
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
		RateLimit:   5, // per worker per second
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

	// Create rate limiters
	limiter := rate.NewLimiter(rate.Limit(p.config.RateLimit), p.config.RateBurst)

	jobCh := make(chan *entity.Content, 1000)

	// Launch workers
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i, limiter, jobCh)
	}

	// Subscribe to jobs and feed channel
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

	// Wait for shutdown
	<-ctx.Done()

	log.Println("Shutting down worker pool...")
	close(jobCh)
	p.wg.Wait()
	log.Println("All workers exited")
	return nil
}

func (p *Pool) worker(ctx context.Context, id int, limiter *rate.Limiter, jobs <-chan *entity.Content) {
	defer p.wg.Done()
	log.Printf("Worker %d started", id)

	for content := range jobs {
		// Rate limit
		if err := limiter.Wait(ctx); err != nil {
			log.Printf("Worker %d rate limit wait cancelled", id)
			return
		}

		// Set job timeout
		jobCtx, cancel := context.WithTimeout(ctx, p.config.JobTimeout)
		defer cancel()

		p.processJob(jobCtx, content)
	}
	log.Printf("Worker %d stopped", id)
}

func (p *Pool) processJob(ctx context.Context, content *entity.Content) {
	log.Printf("Worker processing content: %s (type: %s)", content.ID, content.Type)

	// Moderate the content (this will call AI, cache, and publish result)
	result, err := p.modSvc.ModerateContent(ctx, content.ID)
	if err != nil {
		log.Printf("ERROR moderating content %s: %v", content.ID, err)
		return
	}

	// Handle the result (update DB status, publish notification)
	if err := p.modSvc.HandleModerationResult(ctx, result); err != nil {
		log.Printf("ERROR handling moderation result for %s: %v", content.ID, err)
	}
}
