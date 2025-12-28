package server

// Input 客户端输入（意图），由服务端在 Tick 中解释并驱动世界状态
type Input struct {
    PlayerID PlayerID
    Command  Direction
    Seq      int64 // 客户端本地序列号，用于去重与确认
}

// 入站输入的简单 JSON 结构（WebSocket 文本消息）
// 示例：{"type":"move","command":"up"}
type InputMessage struct {
    Type    string `json:"type"`
    Command string `json:"command"`
    Seq     int64  `json:"seq,omitempty"`
}
