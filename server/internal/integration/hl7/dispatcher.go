package hl7

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"
)

// Dispatcher pushes an outbound ADT message to the configured peer (PACS,
// LIS, regional HIE). Return the peer's MSA / ACK body on success so the
// outbox can persist it for audit.
//
// Two impls: MockDispatcher (no network, deterministic ack) for dev /
// pilot, MLLPDispatcher for real peers (MLLP over TCP — HL7's wire
// protocol). The interface stays minimal so a future HTTP/S dispatcher
// can slot in without churn.
type Dispatcher interface {
	Send(ctx context.Context, msg string) (ack string, err error)
}

// NewDispatcher returns the right Dispatcher for the given peer address.
// Empty address → MockDispatcher (in-process log + synthetic AA ack).
// Otherwise → MLLPDispatcher pointed at the peer. The log line is loud
// on purpose: ops should be able to grep `journalctl` to confirm a
// production deployment didn't silently downgrade to mock.
func NewDispatcher(peerAddress string, timeout time.Duration, log *slog.Logger) Dispatcher {
	if log == nil {
		log = slog.Default()
	}
	if strings.TrimSpace(peerAddress) == "" {
		log.Info("hl7 ADT: mock dispatcher (no peer address configured)")
		return &MockDispatcher{Log: log}
	}
	log.Info("hl7 ADT: MLLP dispatcher wired", "peer", peerAddress)
	return &MLLPDispatcher{Address: peerAddress, Timeout: timeout, Log: log}
}

// MockDispatcher always succeeds and returns a synthetic ACK. Used when
// no peer is configured — dev environments + pilot test runs.
type MockDispatcher struct {
	Log *slog.Logger
}

func (m *MockDispatcher) Send(_ context.Context, msg string) (string, error) {
	if m.Log != nil {
		m.Log.Debug("hl7 mock dispatch", "bytes", len(msg))
	}
	// Echo back a minimal HL7 ACK.
	controlID := extractMSH10(msg)
	ack := fmt.Sprintf("MSH|^~\\&|MEDIGT|MOCK||MEDIGT|%s||ACK|%s-ACK|P|2.5\rMSA|AA|%s\r",
		time.Now().UTC().Format("20060102150405"),
		controlID, controlID)
	return ack, nil
}

// MLLPDispatcher sends a single message over MLLP and reads the ACK back.
// MLLP framing: 0x0B <message> 0x1C 0x0D. Keep-alive is connection-per-
// message for simplicity; production with a high-volume peer would
// pool connections — defer until real measurements show contention.
type MLLPDispatcher struct {
	Address string        // "host:port"
	Timeout time.Duration // dial+read+write deadline
	Log     *slog.Logger
}

func (d *MLLPDispatcher) Send(ctx context.Context, msg string) (string, error) {
	if d.Address == "" {
		return "", fmt.Errorf("MLLP address not configured")
	}
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", d.Address)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// MLLP framing.
	if _, err := conn.Write(append([]byte{0x0B}, []byte(msg)...)); err != nil {
		return "", err
	}
	if _, err := conn.Write([]byte{0x1C, 0x0D}); err != nil {
		return "", err
	}

	// Read until 0x1C is seen (end-of-block).
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, rerr := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if idx := indexByte(buf, 0x1C); idx >= 0 {
				// Strip leading 0x0B and trailing 0x1C 0x0D.
				start := 0
				if len(buf) > 0 && buf[0] == 0x0B {
					start = 1
				}
				return string(buf[start:idx]), nil
			}
		}
		if rerr != nil {
			if len(buf) > 0 {
				return string(buf), nil
			}
			return "", rerr
		}
	}
}

func indexByte(buf []byte, b byte) int {
	for i, c := range buf {
		if c == b {
			return i
		}
	}
	return -1
}

// extractMSH10 reads the message control id from a built ADT message so
// the mock ACK can reflect it back.
func extractMSH10(msg string) string {
	end := strings.IndexByte(msg, '\r')
	if end < 0 {
		end = len(msg)
	}
	msh := msg[:end]
	fields := strings.Split(msh, "|")
	if len(fields) > 9 {
		return fields[9]
	}
	return ""
}
