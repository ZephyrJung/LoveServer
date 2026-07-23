# Game Server Design — 通用多人游戏服务器

## 概述

通用多人游戏服务器框架，支持多类游戏（回合制/实时）、游戏类型可扩展，客户端为 Love2D。架构遵循"游戏逻辑不感知网络层"原则，为未来分布式部署预留接口。

## 技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| 语言 | Go | 高性能并发，标准库完善 |
| 通信 | WebSocket (gorilla/websocket) | 全双工，帧边界清晰，Love2D 生态支持 |
| 消息格式 | JSON | 开发方便，Lua/Go 都有标准库 |
| 实时缓存 | Redis | 状态缓存、匹配队列、Session、Pub/Sub |
| 持久化 | PostgreSQL | 用户白名单、游戏记录、历史数据 |
| 配置 | YAML | 便于管理 |

## 架构

```
Client (Love2D)
    │ WebSocket
    ▼
┌──────────────────┐
│  WebSocket Listener │
└────────┬─────────┘
         │
┌────────▼─────────┐     ┌──────────────────┐
│  Session Layer   │◀───▶│  Auth (白名单)    │
│  (连接管理+鉴权)  │     └──────┬───────────┘
└────────┬─────────┘            │
         │                      │
┌────────▼─────────┐     ┌──────▼───────────┐
│  Hub (消息中枢)  │     │  PostgreSQL      │
│  (路由/分发/中间件)│     │  (白名单+持久化)  │
└──┬───────────┬───┘     └──────────────────┘
   │           │                ▲
┌──▼─────┐ ┌───▼────────┐      │
│ Lobby  │ │ Game Room  │      │
│(房间/匹配)│ │(游戏实例)   │      │
└────────┘ └─────┬──────┘      │
                 │              │
            ┌────▼──────┐      │
            │   Redis   │◀─────┘
            │(实时状态缓存)│
            └───────────┘
```

## 目录结构

```
LoveServer/
├── main.go                    # 入口
├── config.yaml                # 配置
├── go.mod / go.sum
│
├── internal/
│   ├── server/
│   │   └── server.go          # HTTP/WS 启动
│   │
│   ├── hub/
│   │   ├── hub.go             # 消息中枢：路由注册+分发
│   │   ├── message.go         # Message 结构定义
│   │   └── middleware.go      # 中间件链
│   │
│   ├── session/
│   │   ├── session.go         # Session 结构
│   │   └── manager.go         # SessionManager
│   │
│   ├── auth/
│   │   └── auth.go            # 白名单鉴权
│   │
│   ├── lobby/
│   │   ├── lobby.go           # 大厅逻辑
│   │   ├── room.go            # 房间管理
│   │   ├── match.go           # 匹配队列
│   │   └── handler.go         # Lobby/Room/Match Handler
│   │
│   ├── game/
│   │   ├── interface.go       # Game 接口
│   │   ├── registry.go        # 游戏注册表
│   │   └── manager.go         # 游戏实例管理器
│   │
│   ├── storage/
│   │   ├── postgres.go        # PostgreSQL 客户端
│   │   ├── redis.go           # Redis 客户端
│   │   ├── whitelist.go       # 白名单 DAO
│   │   └── gamerecord.go      # 游戏记录 DAO
│   │
│   └── config/
│       └── config.go          # 配置加载
│
└── games/                     # 具体游戏实现（按包划分）
    ├── template/              # 游戏模板脚手架
    │   └── game.go
    └── ...                    # 后续添加
```

## 消息协议

### 统一消息格式

```json
// 请求
{ "type": "xxx", "data": {...}, "id": "req-001" }

// 响应
{ "type": "xxx", "data": {...}, "id": "req-001", "code": 0, "msg": "ok" }

// 服务端推送
{ "type": "event.xxx", "data": {...}, "code": 0, "msg": "ok" }
```

### 消息分类

| 前缀 | 用途 | 示例 |
|------|------|------|
| `auth.` | 鉴权 | `auth.login` |
| `lobby.` | 大厅 | `lobby.create_room`, `lobby.list_rooms`, `lobby.join_room`, `lobby.leave_room` |
| `room.` | 房间内 | `room.ready`, `room.start_game`, `room.kick_player`, `room.chat` |
| `match.` | 匹配 | `match.start`, `match.cancel` |
| `game.` | 游戏操作 | `game.move`, `game.sync_state` |
| `event.` | 服务端推送 | `event.room_joined`, `event.game_started`, `event.player_left` |

## 连接生命周期

```
1. WebSocket 连接建立 → Session 创建（未鉴权状态）
2. 客户端发送 auth.login → 服务器查 PG 验证白名单 token
3. 鉴权成功 → Session 标记已鉴权，返回玩家信息
4. 鉴权失败或 10 秒无鉴权 → 断开连接
5. 正常通信 → Hub 路由消息到对应 Handler
6. 断开连接 → Session 清理，玩家离开房间/匹配队列
```

## Session

```go
type Session struct {
    ID         string
    Conn       *websocket.Conn
    PlayerID   string
    Nickname   string
    RoomID     string
    Authed     bool
    CreatedAt  time.Time
    mu         sync.Mutex    // 并发写保护
}
```

SessionManager 管理所有在线 Session，提供按 PlayerID 查询、广播到房间等功能。

## Hub 消息中枢

### 路由机制

Handler 按消息 type 前缀注册，Hub 解析消息后按前缀路由：

```go
type Handler interface {
    Route() string           // 返回前缀，如 "lobby"
    Handle(session *Session, msg *Message) error
}
```

消息处理链：
```
原始二进制 → JSON 解析 → Middleware 链 → 查找 Handler → Handler.Handle()
                                                                    │
                                                             响应/推送回 Session
```

### Middleware

- **LoggerMiddleware** — 请求日志，记录消息类型和耗时
- **AuthMiddleware** — 检查 Session 是否已鉴权（auth.* 除外）
- **RateLimitMiddleware** — 简单速率限制

## 房间生命周期

```
创建 → Waiting（等待中）
         │ 玩家准备完毕 + 房主点击开始
         ▼
Playing（游戏中）
         │ 游戏逻辑判断结束
         ▼
Ended（已结束）
         │ 倒计时或手动解散
         ▼
Closed（已关闭，从内存清除）
```

## Room 结构

```go
type Room struct {
    ID          string
    Name        string
    GameName    string              // 游戏类型
    State       RoomState           // waiting/playing/ended/closed
    OwnerID     string
    Players     map[string]*Player
    MaxPlayers  int
    MinPlayers  int
    Settings    map[string]any      // 房间自定义参数
    CreatedAt   time.Time
    GameInstance Game               // 当前游戏实例
    mu          sync.RWMutex
}
```

## 匹配系统

按游戏类型分队列，定时检查（每 2 秒）：

```
match.start → 玩家进入对应游戏类型队列
              └→ 定时器检查队列人数 ≥ MinPlayers
                  └→ 自动创建房间，拉入玩家
                      └→ 所有人确认 → OnStart()
```

## Game 接口（扩展点）

```go
type Game interface {
    Name() string
    DisplayName() string
    MinPlayers() int
    MaxPlayers() int

    OnInit(room *Room, settings map[string]any) error
    OnPlayerJoin(room *Room, player *Player) error
    OnPlayerLeave(room *Room, player *Player) error
    OnStart(room *Room) error
    OnMessage(room *Room, player *Player, data json.RawMessage) (any, error)
    OnTick(room *Room, dt float64)          // 实时游戏用
    OnEnd(room *Room) (*GameResult, error)

    // 状态序列化（断线重连/持久化）
    MarshalState() ([]byte, error)
    UnmarshalState(data []byte) error
}
```

新增游戏 = 实现 Game 接口 + 在 GameRegistry 注册：

```go
gameRegistry.Register(func() Game { return &MyGame{} })
```

## 数据存储

| 数据 | 存储 | 用途 |
|------|------|------|
| 白名单 token | PostgreSQL | 鉴权 |
| 在线 Session | Redis | 快速查询连接状态 |
| 房间/匹配队列 | Redis | 实时操作 |
| 游戏实时状态 | Redis | 高频读写 |
| 游戏记录 | PostgreSQL | 持久化存档 |

## 未来分布式扩展路径

- 替换 Hub 消息分发：`channel` → `Redis Pub/Sub`
- 游戏实例管理：本地 `map` → `Redis 共享状态 + 分布式锁`
- **Game 接口实现无需修改**，只改基础设施层

## 错误码

| code | 含义 |
|------|------|
| 0 | 成功 |
| 1001 | 未鉴权 |
| 1002 | 鉴权失败 |
| 1003 | 消息格式错误 |
| 2001 | 房间不存在 |
| 2002 | 房间已满 |
| 2003 | 不在房间中 |
| 2004 | 不是房主 |
| 3001 | 游戏类型不存在 |
| 3002 | 游戏已开始 |
| 4001 | 匹配队列已满 |
| 5001 | 内部错误 |
