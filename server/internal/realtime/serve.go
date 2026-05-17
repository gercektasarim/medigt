package realtime

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	pongWait    = 75 * time.Second
	pingInterval = 30 * time.Second
	writeWait   = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		// CORS allow-list is enforced at the HTTP layer; the upgrade itself is permissive.
		return true
	},
}

func ServeWS(hub *Hub, log *slog.Logger, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn("ws upgrade failed", "err", err)
		return
	}

	client := newClient(uuid.NewString())
	hub.registerCh <- client

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go writePump(conn, client, log)
	go readPump(conn, hub, client, log)
}

func readPump(conn *websocket.Conn, hub *Hub, client *Client, log *slog.Logger) {
	defer func() {
		hub.removeCh <- client.id
		_ = conn.Close()
	}()
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn("ws read error", "err", err)
			}
			return
		}
		// TODO: parse subscribe/unsubscribe messages and call client.subscribe.
	}
}

func writePump(conn *websocket.Conn, client *Client, log *slog.Logger) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	for {
		select {
		case msg, ok := <-client.send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Warn("ws write error", "err", err)
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
