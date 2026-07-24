# 坦克大战 — 服务端接口文档 (WebSocket)

> 客户端项目：loveTank (Love2D 实现)
> 
> 服务端项目：LoveServer (Go 实现)
>
> 协议：WebSocket + JSON
>
> 最后更新：2026-07-23

---

## 目录

1. [连接与鉴权](#1-连接与鉴权)
2. [消息格式](#2-消息格式)
3. [大厅与房间流程](#3-大厅与房间流程)
4. [游戏协议](#4-游戏协议)
5. [完整消息参考](#5-完整消息参考)
6. [游戏状态快照格式](#6-游戏状态快照格式)
7. [常量表](#7-常量表)
8. [对接步骤](#8-对接步骤)
9. [附录：Lua 示例代码](#9-附录lua-示例代码)

---

## 1. 连接与鉴权

### 1.1 WebSocket 连接

```
ws://<server-addr>:8080/ws
```

示例：`ws://127.0.0.1:8080/ws`

### 1.2 鉴权流程

连接建立后，客户端必须在 **10 秒内** 发送鉴权请求，否则服务端断开连接。

```
客户端 → 服务端:
{
  "type": "auth.login",
  "data": {
    "token": "test-token-001"
  },
  "id": "req-001"
}

服务端 → 客户端 (成功):
{
  "type": "auth.login",
  "data": {
    "player_id": "player-001",
    "nickname": "PlayerOne"
  },
  "id": "req-001",
  "code": 0,
  "msg": "ok"
}

服务端 → 客户端 (失败):
{
  "type": "auth.login",
  "data": null,
  "id": "req-001",
  "code": 1002,
  "msg": "invalid token"
}
```

### 1.3 白名单 Token

| Token | PlayerID | 昵称 |
|-------|----------|------|
| `test-token-001` | `player-001` | PlayerOne |
| `test-token-002` | `player-002` | PlayerTwo |
| `test-token-003` | `player-003` | PlayerThree |

> 生产环境的白名单存在 PostgreSQL 的 `whitelist_tokens` 表中，可用 `scripts/seed.sql` 初始化。

---

## 2. 消息格式

### 2.1 客户端 → 服务端（请求）

```json
{
  "type": "消息类型",
  "data": { ... },
  "id": "请求ID（可选，用于匹配响应）"
}
```

### 2.2 服务端 → 客户端（响应）

```json
{
  "type": "消息类型",
  "data": { ... },
  "id": "对应请求的ID（如有）",
  "code": 0,
  "msg": "ok"
}
```

`code = 0` 表示成功，非 0 表示错误。

### 2.3 服务端 → 客户端（推送/事件）

```json
{
  "type": "event.xxx",
  "data": { ... },
  "code": 0,
  "msg": "ok"
}
```

事件没有 `id` 字段，因为不是对某个请求的响应。

---

## 3. 大厅与房间流程

### 3.1 完整流程

```
客户端                                  服务端
  │                                       │
  │── WebSocket 连接 ───────────────────▶  │
  │                                       │
  │── auth.login (token) ───────────────▶  │
  │◀── auth.login (player_id, nickname) ── │
  │                                       │
  │── lobby.create_room ────────────────▶  │  (房主)
  │◀── lobby.create_room (房间信息) ───────  │
  │                                       │
  │── lobby.join_room ──────────────────▶  │  (其他玩家)
  │◀── lobby.join_room (房间信息) ─────────  │
  │◀── event.player_joined (广播) ────────  │
  │                                       │
  │── room.ready ───────────────────────▶  │  (所有人)
  │◀── room.ready (ready状态) ────────────  │
  │◀── event.player_ready (广播) ─────────  │
  │                                       │
  │── room.start_game ──────────────────▶  │  (房主)
  │◀── room.start_game (status=started) ──  │
  │◀── event.game_started (广播) ─────────  │
  │                                       │
  │── game.move (输入) ─────────────────▶  │  (游戏中)
  │◀── event.game_state (20Hz广播) ──────  │  │
  │◀── event.game_over (游戏结束) ────────  │
  │                                       │
  │── lobby.leave_room ────────────────▶   │
  │◀── event.player_left (广播) ─────────  │
```

### 3.2 创建房间

**请求：**
```json
{
  "type": "lobby.create_room",
  "data": {
    "name": "坦克大战01",
    "game_name": "tankbattle",
    "settings": {
      "mode": "dm"
    }
  },
  "id": "req-002"
}
```

**响应：**
```json
{
  "type": "lobby.create_room",
  "data": {
    "id": "room-uuid-xxx",
    "name": "坦克大战01",
    "game_name": "tankbattle",
    "state": "waiting",
    "owner_id": "player-001",
    "players": [
      {
        "id": "player-001",
        "nickname": "PlayerOne",
        "ready": false,
        "score": 0
      }
    ],
    "player_count": 1,
    "max_players": 6,
    "min_players": 2,
    "created_at": "2026-07-23T..."
  },
  "id": "req-002",
  "code": 0,
  "msg": "ok"
}
```

> **注意：** `settings.mode` 可选 `"dm"`（个人赛）或 `"team"`（团队赛），默认 `"dm"`。

### 3.3 加入房间

**请求：**
```json
{
  "type": "lobby.join_room",
  "data": {
    "room_id": "room-uuid-xxx"
  },
  "id": "req-003"
}
```

**响应：** 同创建房间的响应格式。

**广播（房间内所有人收到）：**
```json
{
  "type": "event.player_joined",
  "data": {
    "room_id": "room-uuid-xxx",
    "player_id": "player-002",
    "nickname": "PlayerTwo"
  },
  "code": 0,
  "msg": "ok"
}
```

### 3.4 离开房间

**请求：**
```json
{
  "type": "lobby.leave_room",
  "data": {},
  "id": "req-004"
}
```

**响应：**
```json
{
  "type": "lobby.leave_room",
  "data": {
    "room_id": "room-uuid-xxx"
  },
  "id": "req-004",
  "code": 0,
  "msg": "ok"
}
```

**广播：**
```json
{
  "type": "event.player_left",
  "data": {
    "room_id": "room-uuid-xxx",
    "player_id": "player-002"
  },
  "code": 0,
  "msg": "ok"
}
```

### 3.5 查看房间列表

**请求：**
```json
{
  "type": "lobby.list_rooms",
  "data": {
    "game_name": "tankbattle"
  },
  "id": "req-005"
}
```

**响应：**
```json
{
  "type": "lobby.list_rooms",
  "data": {
    "rooms": [
      {
        "id": "room-uuid-xxx",
        "name": "坦克大战01",
        "game_name": "tankbattle",
        "state": "waiting",
        ...
      }
    ]
  },
  "id": "req-005",
  "code": 0,
  "msg": "ok"
}
```

### 3.6 查看房间信息

**请求：**
```json
{
  "type": "lobby.room_info",
  "data": {
    "room_id": "room-uuid-xxx"
  },
  "id": "req-006"
}
```

**响应：** 同创建房间的响应格式。

### 3.7 准备/取消准备

**请求：**
```json
{
  "type": "room.ready",
  "data": {},
  "id": "req-007"
}
```

**响应：**
```json
{
  "type": "room.ready",
  "data": {
    "ready": true
  },
  "id": "req-007",
  "code": 0,
  "msg": "ok"
}
```

**广播：**
```json
{
  "type": "event.player_ready",
  "data": {
    "room_id": "room-uuid-xxx",
    "player_id": "player-001",
    "ready": true
  },
  "code": 0,
  "msg": "ok"
}
```

> 每次调用会切换准备状态（true ↔ false）。

### 3.8 开始游戏（仅房主）

**请求：**
```json
{
  "type": "room.start_game",
  "data": {},
  "id": "req-008"
}
```

**响应：**
```json
{
  "type": "room.start_game",
  "data": {
    "status": "started"
  },
  "id": "req-008",
  "code": 0,
  "msg": "ok"
}
```

**广播：**
```json
{
  "type": "event.game_started",
  "data": {
    "room_id": "room-uuid-xxx",
    "game": "tankbattle"
  },
  "code": 0,
  "msg": "ok"
}
```

> 游戏开始后，服务端会以 20Hz 频率广播游戏状态快照。

### 3.9 房间内聊天

**请求：**
```json
{
  "type": "room.chat",
  "data": {
    "text": "大家好！"
  },
  "id": "req-009"
}
```

**广播：**
```json
{
  "type": "event.chat",
  "data": {
    "room_id": "room-uuid-xxx",
    "player_id": "player-001",
    "nickname": "PlayerOne",
    "data": {
      "text": "大家好！"
    }
  },
  "code": 0,
  "msg": "ok"
}
```

---

## 4. 游戏协议

### 4.1 发送输入

游戏开始后，客户端以 **20Hz** 频率发送按键输入。

**请求：**
```json
{
  "type": "game.move",
  "data": {
    "seq": 1,
    "keys": {
      "w": true,
      "s": false,
      "a": false,
      "d": false,
      "u": false,
      "i": false,
      "fire": false
    }
  },
  "id": "req-010"
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `seq` | int | 递增序号，用于客户端去重或检测丢包 |
| `keys.w` | bool | 前进（W键） |
| `keys.s` | bool | 后退（S键） |
| `keys.a` | bool | 左转车身（A键） |
| `keys.d` | bool | 右转车身（D键） |
| `keys.u` | bool | 左转炮塔（U键） |
| `keys.i` | bool | 右转炮塔（I键） |
| `keys.fire` | bool | 开火（J键或空格） |

> **注意：** 如果有任何键按下，建议每 tick 都发送一次输入（即使按键没变），保证服务端收到最新状态。
> 松开按键时发送 `false`。

### 4.2 开火（独立消息）

开火也可以通过 `keys.fire` 字段发送。但如果你需要更精确的开火控制，也可以用独立消息：

```json
{
  "type": "game.move",
  "data": {
    "type": "fire"
  },
  "id": "req-011"
}
```

> 两种方式效果相同。建议统一使用 `keys.fire` 方式。

### 4.3 游戏状态广播（20Hz）

服务端每 tick（50ms）向房间内所有玩家广播一次游戏状态快照。

**推送事件：**
```json
{
  "type": "event.game_state",
  "data": {
    "tick": 1234,
    "phase": "playing",
    "mode": "dm",
    "players": [
      {
        "id": 1,
        "name": "PlayerOne",
        "x": 100.5,
        "y": 200.3,
        "angle": 1.234,
        "turret_angle": 0.567,
        "health": 3,
        "max_health": 3,
        "team": 0,
        "alive": true,
        "kills": 2,
        "deaths": 0
      }
    ],
    "bullets": [
      {
        "id": 1,
        "x": 150.0,
        "y": 250.0,
        "vx": 300.0,
        "vy": 0.0,
        "owner_id": 1
      }
    ],
    "walls": [
      {
        "x": 0,
        "y": 0,
        "w": 800,
        "h": 16,
        "type": "steel",
        "alive": true
      }
    ],
    "scores": {
      "red": 0,
      "blue": 0
    },
    "winner": null
  },
  "code": 0,
  "msg": "ok"
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `tick` | int | 当前 tick 序号（从 0 递增） |
| `phase` | string | 游戏阶段：`"playing"` 或 `"game_over"` |
| `mode` | string | 游戏模式：`"dm"` 或 `"team"` |
| `players` | array | 玩家列表（见下文） |
| `bullets` | array | 子弹列表（见下文） |
| `walls` | array | 墙壁列表（见下文） |
| `scores` | object | 团队分数（`{"red": N, "blue": N}`） |
| `winner` | int/string/null | 获胜者：DM 模式为玩家ID，Team 模式为 `"red"`/`"blue"` |

**Player 对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int | 游戏内玩家ID（1-6） |
| `name` | string | 玩家昵称 |
| `x` | float | 坦克左上角 X 坐标 |
| `y` | float | 坦克左上角 Y 坐标 |
| `angle` | float | 车身角度（弧度，0-2π） |
| `turret_angle` | float | 炮塔角度（弧度，0-2π） |
| `health` | int | 当前生命值 |
| `max_health` | int | 最大生命值（3） |
| `team` | int | 0=个人, 1=红队, 2=蓝队 |
| `alive` | bool | 是否存活 |
| `kills` | int | 击杀数 |
| `deaths` | int | 死亡数 |

**Bullet 对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int | 子弹唯一ID |
| `x` | float | 子弹中心 X 坐标 |
| `y` | float | 子弹中心 Y 坐标 |
| `vx` | float | 水平速度（像素/秒） |
| `vy` | float | 垂直速度（像素/秒） |
| `owner_id` | int | 发射者的玩家ID |

**Wall 对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `x` | float | 墙壁左上角 X 坐标 |
| `y` | float | 墙壁左上角 Y 坐标 |
| `w` | float | 墙壁宽度 |
| `h` | float | 墙壁高度 |
| `type` | string | `"steel"`（钢墙，不可摧毁）或 `"brick"`（砖墙，可摧毁） |
| `alive` | bool | 是否存活（`false` 表示已被摧毁） |

### 4.4 游戏结束广播

当游戏结束时（达到击杀上限），服务端推送：

```json
{
  "type": "event.game_over",
  "data": {
    "winner": 1
  },
  "code": 0,
  "msg": "ok"
}
```

`winner` 字段：
- DM 模式：获胜玩家的 `id`（int）
- Team 模式：`"red"` 或 `"blue"`（string）

同时，最后一个 `event.game_state` 的 `phase` 也会变为 `"game_over"`。

---

## 5. 完整消息参考

### 5.1 客户端 → 服务端

| 消息类型 | 用途 | 发送时机 |
|----------|------|----------|
| `auth.login` | 鉴权 | 连接建立后立即发送 |
| `lobby.create_room` | 创建房间 | 房主点击"创建房间" |
| `lobby.list_rooms` | 查看房间列表 | 房主或加入者查看列表 |
| `lobby.join_room` | 加入房间 | 加入者点击"加入房间" |
| `lobby.leave_room` | 离开房间 | 点击"返回" |
| `lobby.room_info` | 查看房间详情 | 查询当前房间信息 |
| `room.ready` | 准备/取消准备 | 按 Enter 切换准备状态 |
| `room.start_game` | 开始游戏 | 房主点击"开始游戏" |
| `room.chat` | 发送聊天消息 | 发送聊天内容 |
| `game.move` | 游戏输入 | 游戏中每 tick 发送 |

### 5.2 服务端 → 客户端（推送事件）

| 事件类型 | 用途 |
|----------|------|
| `event.player_joined` | 有玩家加入房间 |
| `event.player_left` | 有玩家离开房间 |
| `event.player_ready` | 有玩家切换准备状态 |
| `event.game_started` | 游戏开始 |
| `event.game_state` | 游戏状态快照（20Hz） |
| `event.game_over` | 游戏结束 |
| `event.game_move` | 游戏内操作结果（预留） |
| `event.chat` | 聊天消息 |

### 5.3 错误码

| code | 含义 | 说明 |
|------|------|------|
| 0 | 成功 | 操作成功 |
| 1001 | 未鉴权 | 需要先发 `auth.login` |
| 1002 | 鉴权失败 | Token 无效 |
| 1003 | 消息格式错误 | JSON 解析失败或缺少必要字段 |
| 2001 | 房间不存在 | 指定的 room_id 无效 |
| 2002 | 房间已满 | 房间人数已达上限 |
| 2003 | 不在房间中 | 当前玩家不在任何房间 |
| 2004 | 不是房主 | 只有房主才能执行该操作 |
| 3001 | 游戏类型不存在 | 指定的 game_name 未注册 |
| 3002 | 游戏已开始 | 游戏已经开始，无法重复开始 |
| 3003 | 游戏未开始 | 游戏尚未开始，无法执行游戏操作 |
| 5001 | 内部错误 | 服务端异常 |

---

## 6. 游戏状态快照格式

### 6.1 完整 JSON 结构

```json
{
  "tick": 1234,
  "phase": "playing",
  "mode": "dm",
  "players": [
    {
      "id": 1,
      "name": "PlayerOne",
      "x": 100.5,
      "y": 200.3,
      "angle": 1.234,
      "turret_angle": 0.567,
      "health": 3,
      "max_health": 3,
      "team": 0,
      "alive": true,
      "kills": 2,
      "deaths": 0
    }
  ],
  "bullets": [
    {
      "id": 5,
      "x": 350.0,
      "y": 300.0,
      "vx": 300.0,
      "vy": 0.0,
      "owner_id": 1
    }
  ],
  "walls": [
    {"x": 0, "y": 0, "w": 800, "h": 16, "type": "steel", "alive": true}
  ],
  "scores": {"red": 0, "blue": 0},
  "winner": null
}
```

### 6.2 客户端渲染所需的信息

- **地图**：`walls` 数组，`alive=true` 的墙壁需要渲染
- **坦克**：`players` 数组，每个 `alive=true` 的玩家需要渲染
- **子弹**：`bullets` 数组，所有子弹需要渲染
- **HUD**：`players` 中的 `health`、`kills`；`scores`（团队模式）
- **游戏结束**：`phase == "game_over"` 时显示结果，`winner` 为获胜者

### 6.3 坐标系统

- 屏幕大小：**800 × 600** 像素
- 原点：左上角 (0, 0)
- 坦克位置 `(x, y)` 为 **坦克矩形的左上角**
- 子弹位置 `(x, y)` 为 **子弹圆心**
- 墙壁位置 `(x, y)` 为 **墙壁矩形的左上角**

---

## 7. 常量表

### 7.1 坦克

| 常量 | 值 | 说明 |
|------|-----|------|
| 速度 | 100 px/s | 坦克移动速度 |
| 车身宽 | 30 px | 坦克矩形宽度 |
| 车身长 | 40 px | 坦克矩形高度 |
| 炮塔半径 | 12 px | 炮塔头部圆半径 |
| 炮管宽 | 4 px | 炮管矩形宽度 |
| 炮管长 | 30 px | 炮管矩形长度 |
| 最大HP | 3 | 坦克生命值 |
| 复活时间 | 3 秒 | 死亡后等待时间 |

### 7.2 子弹

| 常量 | 值 | 说明 |
|------|-----|------|
| 速度 | 300 px/s | 子弹飞行速度 |
| 半径 | 3 px | 子弹圆形半径 |
| 冷却时间 | 0.5 秒 | 两次开火间隔 |

### 7.3 墙壁

| 类型 | 宽度 | 高度 | 可摧毁 |
|------|------|------|--------|
| 钢墙 (steel) | 32 px | 16 px | ❌ |
| 砖墙 (brick) | 32 px | 16 px | ✅ |
| 边界 | 16 px | 全屏 | ❌ |

### 7.4 游戏规则

| 常量 | 值 |
|------|-----|
| DM 击杀上限 | 10 |
| Team 击杀上限 | 15 |
| 最小玩家数 | 2 |
| 最大玩家数 | 6 |
| Tick 频率 | 20 Hz |
| Tick 间隔 | 50 ms |

### 7.5 出生点

```
Spawn 1: (100, 100)    Spawn 2: (700, 100)
Spawn 3: (100, 500)    Spawn 4: (700, 500)
Spawn 5: (400,  80)    Spawn 6: (400, 520)
```

---

## 8. 对接步骤

### 第一步：替换传输层

将 `Network.lua` 中的 ENet 代码替换为 WebSocket：

```
ENet: host:create() / host:service() / peer:send()
  → WebSocket: websocket.new() / ws:connect() / ws:send() / ws:onmessage()
```

推荐 Lua WebSocket 库：
- [lua-websocket](https://github.com/lipp/lua-websocket) — 纯 Lua 实现
- Love2D 内置的 `love.websocket`（如果可用）

### 第二步：替换 JSON 编码

删除 `Network.lua` 中的自定义 JSON 编码器，改用标准 JSON 库：

```lua
-- 使用标准 JSON 库（如 dkjson 或 json.lua）
local json = require "dkjson"

-- 编码
local msg = json.encode({type="auth.login", data={token="test-token-001"}})

-- 解码
local decoded = json.decode(data)
```

### 第三步：重写 Client.lua

```lua
-- 修改后的 Client:connect()
function Client:connect(host, port)
  self.ws = websocket.new()
  self.ws:connect("ws://" .. host .. ":" .. port .. "/ws")
  
  self.ws:onmessage(function(data)
    local msg = json.decode(data)
    self:_handleMessage(msg)
  end)
  
  -- 连接成功后发送鉴权
  self:send({type="auth.login", data={token="test-token-001"}})
end

-- 发送消息
function Client:send(data)
  self.ws:send(json.encode(data))
end
```

### 第四步：适配大厅流程

用服务端房间管理替代客户端的本地大厅逻辑：

```
原来是：
  localhost 启动 ENet 服务端 → 客户端直连

现在是：
  WebSocket 连接 LoveServer → auth.login → lobby.create_room → room.ready → room.start_game
```

### 第五步：适配游戏输入

```lua
-- 每帧发送输入
function Client:sendInput(input)
  local msg = {
    type = "game.move",
    data = {
      seq = self.inputSeq,
      keys = input
    }
  }
  self:send(msg)
  self.inputSeq = self.inputSeq + 1
end
```

### 第六步：适配游戏状态渲染

```lua
-- 收到 event.game_state 后更新渲染
function Client:_handleMessage(msg)
  if msg.type == "event.game_state" then
    self.latestState = msg.data
  elseif msg.type == "event.game_over" then
    self.gameOver = true
    self.winner = msg.data.winner
  end
end
```

---

## 9. 附录：Lua 示例代码

### 9.1 完整 WebSocket 客户端骨架

```lua
-- TankClient.lua
local json = require "dkjson"

local TankClient = {}
TankClient.__index = TankClient

function TankClient.new()
  return setmetatable({
    ws = nil,
    connected = false,
    authed = false,
    myId = nil,
    myName = nil,
    roomId = nil,
    inputSeq = 0,
    latestState = nil,
    gameOver = false,
    winner = nil,
  }, TankClient)
end

function TankClient:connect(host, port)
  local url = "ws://" .. host .. ":" .. (port or 8080) .. "/ws"
  self.ws = websocket.new()
  self.ws:connect(url)

  self.ws:onmessage(function(data)
    local ok, msg = pcall(json.decode, data)
    if ok and msg then
      self:_onMessage(msg)
    end
  end)
end

function TankClient:_onMessage(msg)
  if msg.type == "auth.login" and msg.code == 0 then
    self.authed = true
    self.myId = msg.data.player_id
    self.myName = msg.data.nickname
  elseif msg.type == "event.game_started" then
    self.gameOver = false
  elseif msg.type == "event.game_state" then
    self.latestState = msg.data
  elseif msg.type == "event.game_over" then
    self.gameOver = true
    self.winner = msg.data.winner
  elseif msg.type == "event.player_joined" then
    -- 更新玩家列表
  elseif msg.type == "event.player_left" then
    -- 更新玩家列表
  end
end

function TankClient:send(msg)
  if self.ws then
    self.ws:send(json.encode(msg))
  end
end

function TankClient:sendAuth(token)
  self:send({type="auth.login", data={token=token}, id="auth-1"})
end

function TankClient:createRoom(name, mode)
  self:send({
    type="lobby.create_room",
    data={name=name, game_name="tankbattle", settings={mode=mode}},
    id="create-1"
  })
end

function TankClient:joinRoom(roomId)
  self:send({type="lobby.join_room", data={room_id=roomId}, id="join-1"})
end

function TankClient:sendReady()
  self:send({type="room.ready", data={}, id="ready-1"})
end

function TankClient:startGame()
  self:send({type="room.start_game", data={}, id="start-1"})
end

function TankClient:sendInput(keys)
  self.inputSeq = self.inputSeq + 1
  self:send({
    type="game.move",
    data={seq=self.inputSeq, keys=keys},
    id="input-" .. self.inputSeq
  })
end

function TankClient:disconnect()
  if self.ws then
    self.ws:close()
    self.ws = nil
  end
  self.connected = false
  self.authed = false
end

return TankClient
```

### 9.2 游戏循环示例

```lua
-- 在 love.update() 中
function love.update(dt)
  if not client.authed then
    -- 等待鉴权
    return
  end

  -- 游戏中：发送输入
  if gameState and gameState.phase == "playing" then
    local input = {
      w = love.keyboard.isDown("w"),
      s = love.keyboard.isDown("s"),
      a = love.keyboard.isDown("a"),
      d = love.keyboard.isDown("d"),
      u = love.keyboard.isDown("u"),
      i = love.keyboard.isDown("i"),
      fire = love.keyboard.isDown("j") or love.keyboard.isDown("space"),
    }
    client:sendInput(input)
  end
end

-- 在 love.draw() 中
function love.draw()
  local state = client.latestState
  if not state then return end

  -- 绘制墙壁
  for _, w in ipairs(state.walls) do
    if w.alive then
      local color = (w.type == "steel") and {128,128,128} or {255,200,0}
      love.graphics.setColor(color)
      love.graphics.rectangle("fill", w.x, w.y, w.w, w.h)
    end
  end

  -- 绘制坦克
  for _, p in ipairs(state.players) do
    if p.alive then
      drawTank(p)
    end
  end

  -- 绘制子弹
  for _, b in ipairs(state.bullets) do
    love.graphics.setColor(255, 255, 255)
    love.graphics.circle("fill", b.x, b.y, 3)
  end

  -- 绘制 HUD
  drawHUD(state)
end
```

---

## 变更日志

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-07-23 | v1.0 | 初版 |