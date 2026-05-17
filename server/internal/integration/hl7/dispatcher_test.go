package hl7

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMockDispatcher_ReturnsAAAck(t *testing.T) {
	d := &MockDispatcher{}
	msg := "MSH|^~\\&|MEDIGT|HOSP1|PACS|PACS1|20260101120000||ADT^A01|CTRL-1|P|2.5\rEVN|A01|20260101120000\r"
	ack, err := d.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(ack, "MSA|AA|CTRL-1") {
		t.Fatalf("expected MSA|AA echoing CTRL-1; got:\n%s", ack)
	}
}

func TestNewDispatcher_PicksMockWhenAddressEmpty(t *testing.T) {
	d := NewDispatcher("", 0, nil)
	if _, ok := d.(*MockDispatcher); !ok {
		t.Fatalf("expected MockDispatcher when no peer, got %T", d)
	}
}

func TestNewDispatcher_PicksMLLPWhenAddressPresent(t *testing.T) {
	d := NewDispatcher("localhost:6661", 2*time.Second, nil)
	mllp, ok := d.(*MLLPDispatcher)
	if !ok {
		t.Fatalf("expected MLLPDispatcher when peer set, got %T", d)
	}
	if mllp.Address != "localhost:6661" {
		t.Fatalf("address not set, got %q", mllp.Address)
	}
}

// fakeMLLPServer accepts a single MLLP message and echoes back an AA ack.
// We don't try to be HL7-compliant on the receiver side — just enough
// framing to exercise the dispatcher's write+read loop.
func fakeMLLPServer(t *testing.T, ackBody string) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		// Type-assert to TCPListener so we can poll Accept with a deadline;
		// the plain net.Listener interface doesn't expose SetDeadline.
		tln := ln.(*net.TCPListener)
		for {
			select {
			case <-done:
				return
			default:
			}
			_ = tln.SetDeadline(time.Now().Add(50 * time.Millisecond))
			conn, err := tln.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				return
			}
			go handleMLLPConn(conn, ackBody)
		}
	}()
	return ln.Addr().String(), func() {
		close(done)
		_ = ln.Close()
		wg.Wait()
	}
}

func handleMLLPConn(conn net.Conn, ackBody string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(conn)
	// Read until we see 0x1C (end-of-block).
	buf := []byte{}
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		buf = append(buf, b)
		if b == 0x1C {
			// Consume trailing 0x0D.
			_, _ = br.ReadByte()
			break
		}
	}
	// Write ACK with MLLP framing.
	_, _ = io.WriteString(conn, "\x0B"+ackBody+"\x1C\r")
}

func TestMLLPDispatcher_RoundTrip_AA(t *testing.T) {
	addr, stop := fakeMLLPServer(t, "MSH|^~\\&|FAKE|PEER|MEDIGT|HOSP1|20260101120000||ACK|ACK-1|P|2.5\rMSA|AA|CTRL-1\r")
	defer stop()

	d := &MLLPDispatcher{Address: addr, Timeout: 2 * time.Second}
	ack, err := d.Send(context.Background(),
		"MSH|^~\\&|MEDIGT|HOSP1|PEER|FAKE|20260101120000||ADT^A01|CTRL-1|P|2.5\rEVN|A01|20260101120000\r")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(ack, "MSA|AA|CTRL-1") {
		t.Fatalf("expected AA ack, got %q", ack)
	}
}

func TestMLLPDispatcher_ConnectionRefused(t *testing.T) {
	// Listen + immediately close to grab a free port that nothing serves.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	_ = ln.Close()

	d := &MLLPDispatcher{Address: addr, Timeout: 500 * time.Millisecond}
	_, err := d.Send(context.Background(), "MSH|^~\\&|MEDIGT|HOSP1|||||")
	if err == nil {
		t.Fatal("expected error when peer is unreachable")
	}
}

func TestParseMSACode(t *testing.T) {
	cases := []struct {
		name string
		ack  string
		want string
	}{
		{"AA", "MSH|^~\\&|...\rMSA|AA|CTRL\r", "AA"},
		{"AE", "MSH|x\rMSA|AE|CTRL|reason\r", "AE"},
		{"AR", "MSH|x\rMSA|AR|CTRL|\r", "AR"},
		{"missing", "MSH|x\rEVN|x\r", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseMSACode(tc.ack); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
