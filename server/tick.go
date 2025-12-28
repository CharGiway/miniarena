package server

import "time"

const (
	// TicksPerSecond 世界推进频率（20 TPS）
	TicksPerSecond = 20
)

var tickInterval = time.Duration(1000/TicksPerSecond) * time.Millisecond // 50ms

// StartTicker 启动房间的 Tick 循环（单线程推进世界）
func (r *Room) StartTicker() {
	if r.tickerStarted {
		return
	}
	r.tickerStarted = true
	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for range ticker.C {
			// 核心循环：处理输入 → 更新世界 → 广播结果
			start := time.Now()
			r.BeginTick() // 同一 Tick 时间线：重置输入计数等帧内状态
			r.ProcessInputs()
			r.UpdateWorld()
			r.BroadcastDelta()
			elapsed := time.Since(start)
			if r.metrics != nil {
				r.metrics.AddTick(elapsed.Nanoseconds())
			}
		}
	}()
}
