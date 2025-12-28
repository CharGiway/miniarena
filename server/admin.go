package server

import (
    "encoding/json"
    "net/http"
)

// HandleAdminConfig 提供房间配置的读取与更新（热更新基本规则）
// GET /admin/config?room=room-1  返回当前配置
// POST /admin/config?room=room-1 以 JSON 载荷更新部分字段
func HandleAdminConfig(w http.ResponseWriter, r *http.Request) {
    roomID := r.URL.Query().Get("room")
    if roomID == "" { roomID = "room-1" }
    rm := GetRoomManager()
    room := rm.GetOrCreateRoom(roomID)

    type cfg struct {
        Step                *float64 `json:"step,omitempty"`
        MaxInputsPerTick    *int     `json:"maxInputsPerTick,omitempty"`
        SimulateDelayMinMs  *int     `json:"simulateDelayMinMs,omitempty"`
        SimulateDelayMaxMs  *int     `json:"simulateDelayMaxMs,omitempty"`
        SimulateDropProb    *float64 `json:"simulateDropProb,omitempty"`
    }

    switch r.Method {
    case http.MethodGet:
        cur := cfg{
            Step:               &room.step,
            MaxInputsPerTick:   &room.maxInputsPerTick,
            SimulateDelayMinMs: &room.simulateDelayMinMs,
            SimulateDelayMaxMs: &room.simulateDelayMaxMs,
            SimulateDropProb:   &room.simulateDropProb,
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(cur)
        return
    case http.MethodPost:
        var body cfg
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "invalid json", http.StatusBadRequest)
            return
        }
        if body.Step != nil { room.step = *body.Step }
        if body.MaxInputsPerTick != nil { room.maxInputsPerTick = *body.MaxInputsPerTick }
        if body.SimulateDelayMinMs != nil { room.simulateDelayMinMs = *body.SimulateDelayMinMs }
        if body.SimulateDelayMaxMs != nil { room.simulateDelayMaxMs = *body.SimulateDelayMaxMs }
        if body.SimulateDropProb != nil { room.simulateDropProb = *body.SimulateDropProb }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
        Log.Infof("config updated: room=%s step=%.2f maxInputsPerTick=%d delay=[%d,%d] drop=%.2f",
            roomID, room.step, room.maxInputsPerTick, room.simulateDelayMinMs, room.simulateDelayMaxMs, room.simulateDropProb)
        return
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
}

// HandleMetrics 输出指定房间的运行指标
// GET /metrics?room=room-1
func HandleMetrics(w http.ResponseWriter, r *http.Request) {
    roomID := r.URL.Query().Get("room")
    if roomID == "" { roomID = "room-1" }
    rm := GetRoomManager()
    room := rm.GetOrCreateRoom(roomID)
    payload := map[string]any{
        "room":    roomID,
        "tick":    room.tickSeq,
        "metrics": room.metrics.Snapshot(),
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(payload)
}

