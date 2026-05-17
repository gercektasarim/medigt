package enabiz

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

// OutboxWorker drains the e-Nabız outbox. Same shape as the Medula
// worker — independent goroutine so Bakanlık outages don't poison SGK
// flow and vice versa. Horizontally safe via FOR UPDATE SKIP LOCKED.
type OutboxWorker struct {
	pool    *pgxpool.Pool
	repo    *repo.EnabizRepo
	client  Client
	log     *slog.Logger
	pollGap time.Duration
}

func NewOutboxWorker(pool *pgxpool.Pool, r *repo.EnabizRepo, client Client, log *slog.Logger) *OutboxWorker {
	return &OutboxWorker{
		pool:    pool,
		repo:    r,
		client:  client,
		log:     log,
		pollGap: 3 * time.Second,
	}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	w.log.Info("enabiz outbox worker starting")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("enabiz outbox worker stopping")
			return
		default:
		}
		processed, err := w.pollOnce(ctx)
		if err != nil {
			w.log.Error("enabiz outbox poll failed", "err", err)
		}
		if !processed {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.pollGap):
			}
		}
	}
}

func (w *OutboxWorker) pollOnce(ctx context.Context) (bool, error) {
	msg, err := w.repo.ClaimNext(ctx)
	if err != nil {
		return false, err
	}
	if msg == nil {
		return false, nil
	}

	in := SubmitInput{
		MessageID:    msg.ID,
		PatientTC:    msg.PatientTC,
		Kind:         ResourceKind(msg.Kind),
		ResourceJSON: msg.ResourceJSON,
	}
	res, err := w.client.SubmitResource(ctx, in)
	if err != nil {
		return true, w.repo.CompleteFailure(ctx, msg.ID, err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return true, w.repo.CompleteFailure(ctx, msg.ID,
			"Bakanlık reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return true, w.repo.CompleteSuccess(ctx, msg.ID, res.ReceiptID, res.Raw)
}
