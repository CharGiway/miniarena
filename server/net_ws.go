package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// ClientConn 负责发送（写）数据到客户端的轻量包装
type ClientConn struct {
	ws   *websocket.Conn
	send chan []byte
}

func NewClientConn(ws *websocket.Conn) *ClientConn {
	return &ClientConn{
		ws:   ws,
		send: make(chan []byte, 64),
	}
}

// Enqueue 将要发送的消息压入队列（非阻塞，满则丢弃）
func (c *ClientConn) Enqueue(b []byte) {
	select {
	case c.send <- b:
	default:
		// 为了实时性，丢弃旧消息（防止阻塞 Tick）
	}
}

// Close 关闭底层连接与发送队列
func (c *ClientConn) Close() {
	if c.send != nil {
		// 关闭发送通道以结束写协程
		close(c.send)
		c.send = nil
	}
	_ = c.ws.Close()
}

// writePump 独立协程，负责从 send 队列写出到 WS
func (c *ClientConn) writePump() {
	defer c.ws.Close()
	for msg := range c.send {
		c.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// readPump 读取客户端输入，转换为 Input 注入房间
func (c *ClientConn) readPump(room *Room, playerID PlayerID) {
	defer c.ws.Close()
	// 读泵退出时，通知房间在 Tick 线程中移除该玩家
	defer room.RequestLeave(playerID)
	c.ws.SetReadLimit(1 << 20) // 1MB
	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })

	for {
		_, payload, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var im InputMessage
		if err := json.Unmarshal(payload, &im); err != nil {
			continue
		}
		if strings.ToLower(im.Type) != "move" {
			continue
		}
		var dir Direction
		switch strings.ToLower(im.Command) {
		case "up":
			dir = DirUp
		case "down":
			dir = DirDown
		case "left":
			dir = DirLeft
		case "right":
			dir = DirRight
		default:
			dir = DirNone
		}
		room.OnInput(Input{PlayerID: playerID, Command: dir})
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 演示环境：允许所有来源（生产环境需严格限制）
		return true
	},
}

// HandleWS WebSocket 接入：?room=room-1&player=alice
func HandleWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "room-1"
	}
	playerID := r.URL.Query().Get("player")
	if playerID == "" {
		http.Error(w, "missing player query", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	rm := GetRoomManager()
	room := rm.GetOrCreateRoom(roomID)

	client := NewClientConn(ws)
	room.JoinPlayer(PlayerID(playerID), client)

	go client.writePump()
	go client.readPump(room, PlayerID(playerID))
}
