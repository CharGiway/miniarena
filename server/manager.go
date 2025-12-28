package server

import "sync"

// RoomManager 管理多个房间的生命周期
type RoomManager struct {
    mu    sync.RWMutex
    rooms map[string]*Room
}

var (
    defaultManager *RoomManager
    once           sync.Once
)

// GetRoomManager 单例房间管理器
func GetRoomManager() *RoomManager {
    once.Do(func() {
        defaultManager = &RoomManager{rooms: make(map[string]*Room)}
    })
    return defaultManager
}

// GetOrCreateRoom 获取或创建房间，并确保开始 Tick
func (m *RoomManager) GetOrCreateRoom(id string) *Room {
    m.mu.Lock()
    defer m.mu.Unlock()
    r, ok := m.rooms[id]
    if !ok {
        r = NewRoom(id)
        m.rooms[id] = r
        r.StartTicker()
    }
    return r
}

