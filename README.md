# MiniArena

一个极简、可运行的实时房间制对战游戏服务端示例（Go）。

核心特点：
- 权威服务端：客户端只发“意图”，位置由服务端决定。
- Tick 驱动：20 TPS（50ms）推进世界，非请求驱动。
- 内存状态：房间与玩家状态在内存维护，DB 仅用于登录/快照（本示例未接入 DB）。

## 项目结构

```
miniarena/
├── main.go               # 入口，HTTP + WebSocket 服务
├── go.mod
├── server/
│   ├── manager.go        # 房间管理器（创建/启动 Tick）
│   ├── room.go           # 房间与世界状态（权威）
│   ├── player.go         # 玩家结构与方向枚举
│   ├── input.go          # 输入模型与 JSON 格式
│   ├── tick.go           # Tick 核心循环（20 TPS）
│   └── net_ws.go         # WebSocket 接入、读写泵
└── protocol/
    ├── input.proto       # 未来可扩展的协议示例（当前走 JSON）
    └── state.proto       # 状态广播协议示例（当前走 JSON）
```

## 运行

1. 拉取依赖并启动：

```
go mod tidy
go run .
```

服务启动后监听 `:8080`，默认房间 `room-1` 已创建并开始 Tick。

2. WebSocket 接入（示例）

连接 URL：`ws://localhost:8080/ws?room=room-1&player=alice`

入站输入（文本 JSON）：

```
{"type":"move","command":"up"}
{"type":"move","command":"down"}
{"type":"move","command":"left"}
{"type":"move","command":"right"}
```

出站状态（文本 JSON，服务端每 Tick 广播一次）：

```
{
  "type": "state",
  "players": [
    {"id":"alice","x":49,"y":50},
    {"id":"bob","x":50,"y":51}
  ]
}
```

## 并发与一致性

- 1 房间 = 1 Tick 协程，房间内不加锁，通过串行推进保证一致性。
- 网络读协程仅将输入压入 `inputChan`，Tick 帧内 drain 处理，避免立刻变更位置。
- 广播通过每玩家的发送队列异步写出，避免阻塞 Tick。

## 下一步可扩展

- 延迟模拟（输入随机延迟 30~100ms）。
- 客户端预测与服务端回滚。
- 断线重连与状态快照。
- 房间热迁移与回放。

