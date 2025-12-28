package server

import (
    "sync/atomic"
)

// RoomMetrics 记录房间运行期的关键指标（用于监控与调试）
type RoomMetrics struct {
    TickCount         int64 // 统计的 Tick 次数
    InputsAccepted    int64 // 被接受的输入数
    RateLimited       int64 // 因同帧限流被拒绝的输入数
    OldSeqIgnored     int64 // 因旧序列被忽略的输入数
    DropsSimulated    int64 // 因模拟丢包被丢弃的输入数
    ChanFullDiscarded int64 // 因通道满被丢弃的输入数
    TotalTickNs       int64 // Tick 累计耗时（纳秒）
}

func (m *RoomMetrics) IncAccepted() { atomic.AddInt64(&m.InputsAccepted, 1) }
func (m *RoomMetrics) IncRateLimited() { atomic.AddInt64(&m.RateLimited, 1) }
func (m *RoomMetrics) IncOldSeqIgnored() { atomic.AddInt64(&m.OldSeqIgnored, 1) }
func (m *RoomMetrics) IncDropsSimulated() { atomic.AddInt64(&m.DropsSimulated, 1) }
func (m *RoomMetrics) IncChanFullDiscarded() { atomic.AddInt64(&m.ChanFullDiscarded, 1) }
func (m *RoomMetrics) AddTick(ns int64) {
    atomic.AddInt64(&m.TickCount, 1)
    atomic.AddInt64(&m.TotalTickNs, ns)
}

// Snapshot 返回只读副本，便于 HTTP 输出
func (m *RoomMetrics) Snapshot() map[string]any {
    tick := atomic.LoadInt64(&m.TickCount)
    total := atomic.LoadInt64(&m.TotalTickNs)
    var avgMs float64
    if tick > 0 {
        avgMs = float64(total) / float64(tick) / 1e6
    }
    return map[string]any{
        "tick_count":          tick,
        "inputs_accepted":     atomic.LoadInt64(&m.InputsAccepted),
        "rate_limited":        atomic.LoadInt64(&m.RateLimited),
        "old_seq_ignored":     atomic.LoadInt64(&m.OldSeqIgnored),
        "drops_simulated":     atomic.LoadInt64(&m.DropsSimulated),
        "chan_full_discarded": atomic.LoadInt64(&m.ChanFullDiscarded),
        "avg_tick_ms":         avgMs,
    }
}

