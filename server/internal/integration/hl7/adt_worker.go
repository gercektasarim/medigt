package hl7

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ADTOutboxWorker drains hl7_outbound_message and pushes each row to the
// configured Dispatcher. Same shape as the Medula / e-Nabız workers —
// independent goroutine, FOR UPDATE SKIP LOCKED safe for horizontal
// scale, exponential backoff with a 5-retry budget.
//
// On success we verify the ACK contains MSA|AA (Accept). MSA|AE (Error)
// or MSA|AR (Reject) are treated as terminal failures with the peer's
// error reason captured for audit.
type ADTOutboxWorker struct {
	pool       *pgxpool.Pool
	repo       *repo.HL7OutboundRepo
	dispatcher Dispatcher
	log        *slog.Logger
	pollGap    time.Duration
}

func NewADTOutboxWorker(pool *pgxpool.Pool, r *repo.HL7OutboundRepo, dispatcher Dispatcher, log *slog.Logger) *ADTOutboxWorker {
	return &ADTOutboxWorker{
		pool:       pool,
		repo:       r,
		dispatcher: dispatcher,
		log:        log,
		pollGap:    3 * time.Second,
	}
}

func (w *ADTOutboxWorker) Run(ctx context.Context) {
	w.log.Info("hl7 ADT outbox worker starting")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("hl7 ADT outbox worker stopping")
			return
		default:
		}
		processed, err := w.pollOnce(ctx)
		if err != nil {
			w.log.Error("hl7 outbox poll failed", "err", err)
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

func (w *ADTOutboxWorker) pollOnce(ctx context.Context) (bool, error) {
	msg, err := w.repo.ClaimNext(ctx)
	if err != nil {
		return false, err
	}
	if msg == nil {
		return false, nil
	}

	ack, err := w.dispatcher.Send(ctx, msg.RawMessage)
	if err != nil {
		return true, w.repo.CompleteFailure(ctx, msg.ID, err.Error(), msg.RetryCount)
	}
	// Inspect MSA segment for AA / AE / AR. Anything other than AA is a
	// terminal peer-side rejection — we do NOT retry app-level rejects.
	if code := parseMSACode(ack); code != "AA" {
		return true, w.repo.CompleteFailure(ctx, msg.ID,
			"peer reject: MSA="+code, msg.RetryCount)
	}
	return true, w.repo.CompleteSuccess(ctx, msg.ID, ack)
}

// parseMSACode returns the acknowledgement code (AA / AE / AR / ???) from
// an HL7 ACK message. Empty if MSA is absent.
func parseMSACode(ack string) string {
	for _, seg := range strings.Split(ack, "\r") {
		if strings.HasPrefix(seg, "MSA|") {
			fields := strings.Split(seg, "|")
			if len(fields) > 1 {
				return strings.TrimSpace(fields[1])
			}
		}
	}
	return ""
}
