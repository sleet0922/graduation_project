# ZAT API 文档

> 基础地址: `https://api.gelsomino.cn:443`  
> 协议: HTTPS · 数据格式: JSON · 编码: UTF-8

---

## 通用说明

### 响应格式

```json
{
  "code": 200,
  "message": "ok",
  "data": {}
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 200 = 成功，其他见错误码表 |
| message | string | 提示信息 |
| data | any | 业务数据，失败时为 null |

### 认证

需要认证的接口在请求头中携带：

```
Authorization: Bearer <access_token>
```

WebSocket 支持两种方式：Header（优先）或 URL 参数 `?token=<token>`。

### Token 体系

| 类型 | 有效期 | 用途 |
|------|--------|------|
| Access Token | 24h | 访问业务接口，内含 session_id |
| Refresh Token | 30d | 刷新时同步轮换，session_id 不变 |

### 多设备登录踢下线

- 每次登录生成新 `session_id` 写入 Redis，旧连接被踢下线
- WebSocket 收到 `{ "type": "kicked", "reason": "账号在其他设备登录" }` 后清 token 跳登录页
- WebSocket 连接时若 session 失效返回 401 `"账号在其他设备登录，请重新登录"`

---

## 一、用户

### POST /api/user/register

注册账号。系统自动生成 10 位数字账号，昵称默认为"未命名用户"。

| 参数 | 类型 | 必填 |
|------|------|------|
| email | string | 是 |
| password | string | 是 |

**返回** `{ id, account, name, email }`

---

### POST /api/user/login

支持邮箱或数字账号登录。

| 参数 | 类型 | 必填 |
|------|------|------|
| account | string | 是 |
| password | string | 是 |

**返回**

```json
{
  "token": "...",
  "refresh_token": "...",
  "expires_in": 86400,
  "refresh_expires_in": 2592000,
  "session_id": "a1b2c3...",
  "user": {
    "id": 1, "account": "...", "name": "...", "avatar": "...",
    "email": "...", "gender": 0, "birthday": "", "location": ""
  }
}
```

> `session_id`: 32 位 hex 字符串，已嵌入 token 中，前端无需单独存储。新设备登录会使旧 session 失效。

---

### POST /api/user/refresh

| 参数 | 类型 | 必填 |
|------|------|------|
| refresh_token | string | 是 |

**返回** `{ token, refresh_token, expires_in, refresh_expires_in }`

> 每次刷新都会 **轮换** refresh_token（旧的立即失效），前端应替换本地存储的两个 token。

---

### POST /api/user/self  🔒

获取当前登录用户完整信息。

**返回** `{ id, account, name, avatar, email, gender, birthday, location }`

---

### GET /api/user/search  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| keyword | string | 是 |

支持按邮箱或 10 位账号搜索。

**返回** `{ id, account, name, avatar, email, gender, birthday, location }`

---

### POST /api/user/name_update  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| name | string | 是 |

**返回** `{ id, name }`

---

### POST /api/user/avatar_update  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| avatar | string | 是 |

值为 OSS 上传返回的文件名（如 `avatar_6_1776183103821.jpg`）。

**返回** `{ id, object_key }`

---

### POST /api/user/password_update  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| password | string | 是（原密码） |
| new_password | string | 是（新密码） |

---

### POST /api/user/profile_update  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gender | int | 否 | 0=未知 1=男 2=女 |
| birthday | string | 否 | 格式 YYYY-MM-DD |
| location | string | 否 | 地区 |

**返回** `{ id, gender, birthday, location }`

---

### POST /api/user/location  🔒

上报用户地理位置。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| latitude | float | 是 | 纬度 |
| longitude | float | 是 | 经度 |
| province | string | 否 | 省份 |
| city | string | 否 | 城市 |
| district | string | 否 | 区/县 |
| address | string | 否 | 详细地址 |
| timestamp | int64 | 否 | Unix 时间戳（秒） |

---

### POST /api/user/delete  🔒

注销当前账号（软删除）。

---

## 二、OSS 文件存储

### GET /api/oss/upload-url  🔒

获取预签名上传 URL，前端直接用 PUT 上传文件。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 文件名 |
| type | string | 否 | `avatar` / `chat` / `video`，默认 `chat` |

**返回** `{ upload_url, access_url, expires_in: "1小时" }`

**流程**: 调此接口 → 拿 upload_url 做 PUT 上传 → 使用 access_url

---

### GET /api/oss/download-url

获取预签名下载 URL。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 自动识别前缀（`avatar_` → `avatar/`, `chat_` → `chat/`） |

**返回** `{ download_url, expires_in: "1小时" }`

---

### POST /api/chat/upload/image  🔒

直传聊天图片（multipart/form-data）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 最大 10MB，支持 image/* 或 application/octet-stream |

**返回** `{ url, content, filename, contentType }`

---

### POST /api/chat/upload/video  🔒

直传聊天视频（multipart/form-data）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 最大 100MB，支持 video/* 或 application/octet-stream |

**返回** `{ url, content, filename, contentType }`

---

## 三、好友

### POST /api/friend/request  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| friend_id | uint | 否 | 对方 ID，与 account 二选一 |
| account | string | 否 | 对方邮箱或账号，与 friend_id 二选一 |

> 边界: `不能添加自己为好友` / `你们已经是好友了` / `好友申请已存在`

---

### GET /api/friend/requests  🔒

获取收到的好友申请列表。

**返回** `[{ id, sender_id, receiver_id, status, created_at }]`

status: `0`=待处理 `1`=已接受 `2`=已拒绝

---

### POST /api/friend/handle  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| request_id | uint | 是 | 申请记录 ID |
| status | uint | 是 | 1=接受 2=拒绝 |

---

### GET /api/friend/list  🔒

好友列表（含用户详情和备注）。

**返回** `[{ id, user_id, friend_id, account, name, email, avatar, gender, birthday, location, remark }]`

---

### POST /api/friend/check  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| friend_id | uint | 是 |

**返回** `{ is_friend: true/false }`

---

### POST /api/friend/delete  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| friend_id | uint | 是 |

---

### POST /api/friend/remark_update  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| friend_id | uint | 是 |
| remark | string | 否 |

---

## 四、群聊

### POST /api/group/create  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 群名称 |
| avatar | string | 否 | 头像文件名 |
| member_ids | []uint | 否 | 初始成员（必须已是好友） |

**返回** `{ id, name, avatar, owner_id, member_count, created_at, updated_at }`

---

### GET /api/group/list  🔒

当前用户加入的所有群聊。

**返回** `[{ id, name, avatar, owner_id, member_count, created_at, updated_at }]`

---

### GET /api/group/members  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| group_id | uint | 是（query） |

**返回** `[{ user_id, account, name, email, avatar, role }]`

role: `owner` / `member`

---

### POST /api/group/member/add  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| group_id | uint | 是 |
| member_ids | []uint | 是（必须是好友） |

**返回**: 群成员列表（同上格式）

---

### POST /api/group/member/remove  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| group_id | uint | 是 | |
| member_id | uint | 是 | 仅群主可操作，不能移除群主 |

---

### POST /api/group/leave  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| group_id | uint | 是 |

> 群主不能直接退出，需先解散。

---

### POST /api/group/delete  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| group_id | uint | 是 | 仅群主可解散 |

解散后通过 WebSocket 向所有成员广播 `group_dissolved` 事件。

---

## 五、WebSocket 实时聊天

### 连接

```
wss://api.gelsomino.cn:443/ws/chat
```

认证: Header `Authorization: Bearer <token>`（主）或 `?token=<token>`（备）。

连接成功收到 `{ "type": "connected", "user_id": N }`

### 心跳

- **服务端**: 每 5s 发 Ping 帧，3s 内未收到 Pong 则断开。
- **客户端**: 可主动发送 `{ "type": "ping" }`，服务端回复 `{ "type": "pong" }`。

---

### 发送消息

**单聊**:
```json
{ "type": "chat", "to_user_id": 9, "message_type": "text", "content": "你好" }
```

**群聊**:
```json
{ "type": "chat", "group_id": 3, "message_type": "text", "content": "大家好" }
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | 是 | 固定 `"chat"` |
| to_user_id | uint | 单聊必填 | 与 group_id 二选一 |
| group_id | uint | 群聊必填 | 与 to_user_id 二选一 |
| message_type | string | 是 | `text` / `image` / `video` 等 |
| content | string | 是 | 消息正文或媒体 URL |

---

### 接收消息

**发送回执**（发给发送者）:
```json
{
  "type": "sent",
  "message": {
    "id": "...", "conversation_type": "single",
    "from_user_id": 8, "to_user_id": 9,
    "message_type": "text", "content": "你好", "created_at": "..."
  }
}
```

**消息投递**（发给接收者）:
```json
{
  "type": "chat",
  "message": { ... },
  "offline": false
}
```

`offline: true` 表示这是离线期间缓存的消息。

**群聊解散**（系统推送）:
```json
{ "type": "group_dissolved", "group_id": 3 }
```

---

### 被踢下线（系统推送）

```json
{ "type": "kicked", "reason": "账号在其他设备登录" }
```

收到后应：清除本地 token → 提示用户 → 跳转登录页。

---

### 错误

```json
{ "type": "error", "error": "错误描述" }
```

常见: `只能给好友发送消息` `消息内容不能为空` `接收方或群聊不能为空`

WebSocket 连接失败 401: 除 token 过期外，也可能是 session 失效（`"账号在其他设备登录，请重新登录"`）。

---

## 六、WebSocket 在线状态

### 连接

```
wss://api.gelsomino.cn:443/ws/online
```

认证同聊天 WS。连接成功收到 `{ "type": "connected", "user_id": N }`。心跳机制同上。

---

### 查询

**查单个**:
```json
{ "type": "check_online", "user_id": 9 }
```
→ `{ "type": "online_status", "user_id": 9, "online": true }`

**查多个**:
```json
{ "type": "check_online", "user_ids": [9, 10] }
```
→ `{ "type": "online_status", "statuses": [{ "user_id": 9, "online": true }, ...] }`

> `user_id` 和 `user_ids` 至少传一个。在线状态基于用户是否持有有效聊天 WS 连接。

---

## 七、E2EE 端到端加密

算法: X25519 + ChaCha20-Poly1305。

### POST /api/e2ee/keys/publish  🔒

发布个人公钥。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key_type | string | 是 | 仅 `"x25519"` |
| public_key | string | 是 | Base64，解码后 32 字节 |

**返回** `{ user_id, key_type, updated_at }`

---

### GET /api/e2ee/keys/public  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| user_id | uint | 是（query） |

**返回** `{ user_id, key_type, public_key, updated_at }`

---

### GET /api/e2ee/group/key/current  🔒

获取当前群密钥盒子。

| 参数 | 类型 | 必填 |
|------|------|------|
| group_id | uint | 是（query） |

**正常返回** `{ group_id, key_version, target_user_id, wrapped_group_key, wrap_nonce, wrapped_by_user_id }`，若 `key_wrap_alg` 非空则一并返回。

**若未上传密钥盒子 → 428** + `{ group_id, key_version, need_publish: true }`，提示前端调用发布接口。

---

### POST /api/e2ee/group/key/publish  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| group_id | uint | 是 | |
| key_version | int | 是 | 必须等于当前版本号 |
| key_wrap_alg | string | 否 | 默认 `"chacha20poly1305-v1"` |
| boxes | []object | 是 | 见下 |

**boxes 元素**:

| 字段 | 类型 | 必填 |
|------|------|------|
| user_id | uint | 是 |
| wrapped_group_key | string | 是 |
| wrap_nonce | string | 是 |

**返回** `{ group_id, key_version, box_count }`

---

### GET /api/e2ee/group/key/by-version  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| group_id | uint | 是（query） |
| key_version | int | 是（query） |

**返回** `{ group_id, key_version, target_user_id, wrapped_group_key, wrap_nonce, wrapped_by_user_id }`

---

## 八、RTC 实时通话

RTC 相关 WebSocket 推送事件（通过聊天 WS 通道下发）：

| 事件类型 | 方向 | 说明 |
|---------|------|------|
| `rtc_invite` | 服务端→被叫 | 来电邀请，含 `call_id`、`room_id`、`call_type`、`from_user_id`、`from_name`、`avatar` |
| `rtc_accept` | 服务端→主叫 | 对方已接听 |
| `rtc_reject` | 服务端→主叫 | 对方已拒绝 |
| `rtc_busy` | 服务端→主叫 | 对方忙线 |
| `rtc_cancel` | 服务端→被叫 | 主叫取消 |
| `rtc_hangup` | 服务端→对方 | 通话挂断 |
| `rtc_timeout` | 服务端→双方 | 45s 超时未接听 |

---

### POST /api/rtc/call/invite  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| call_type | string | 是 | `"voice"` / `"video"` |
| peer_id | uint | 单聊必填 | 与 group_id 二选一 |
| group_id | uint | 群聊必填 | 与 peer_id 二选一 |

**返回** `{ call_id, room_id, call_type, peer_id, group_id }`

> 单聊需对方在线且为好友，群聊需至少一个在线成员。45s 内无人接听自动超时。

---

### POST /api/rtc/call/accept  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| call_id | string | 是 |

**返回** `{ call_id, room_id }`

---

### POST /api/rtc/call/reject  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| call_id | string | 是 |
| reason | string | 否 |

---

### POST /api/rtc/call/cancel  🔒

主叫方在对方接听前取消。

| 参数 | 类型 | 必填 |
|------|------|------|
| call_id | string | 是 |

---

### POST /api/rtc/call/hangup  🔒

通话中任意一方挂断。

| 参数 | 类型 | 必填 |
|------|------|------|
| call_id | string | 是 |

> 未接通前主叫方请使用 cancel 接口。

---

### POST /api/rtc/token  🔒

获取 RTC 房间 Token。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| call_id | string | 是 | |
| call_type | string | 是 | `"voice"` / `"video"` |
| room_id | string | 否 | |
| peer_id | uint | 否 | |
| group_id | uint | 否 | |

**返回** `{ app_id, room_id, uid, token }`

---

## 九、朋友圈/动态 🔒

> 所有接口均需认证。支持发布文字、图片、视频动态，点赞（Toggle）和评论（含回复）。

### 数据模型

**Post 动态**:
```json
{
  "id": 1, "user_id": 24, "content": "文字内容",
  "like_count": 5, "comment_count": 3,
  "created_at": "2026-05-31T08:56:50+08:00",
  "author": { "id": 24, "name": "未命名用户", "avatar": "..." },
  "media": [
    { "id": 1, "media_type": 1, "media_url": "https://cdn.gelsomino.cn/xxx.jpg", "sort_order": 0 }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| media_type | int | 1=图片 2=视频 |
| sort_order | int | 排序，小的在前 |

---

### POST /api/feed/create  🔒

发布动态。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content | string | 否 | 文字内容（与 media 至少有一个不为空） |
| media | []object | 否 | 媒体附件列表 |

**media 元素**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| media_type | int | 是 | 1=图片 2=视频 |
| media_url | string | 是 | OSS 上传后的 access_url |
| sort_order | int | 否 | 排序，默认 0 |

**请求示例**:
```json
// 纯文字
{ "content": "今天天气真好！" }

// 图文
{
  "content": "分享一组照片",
  "media": [
    { "media_type": 1, "media_url": "https://cdn.gelsomino.cn/chat/xxx.jpg", "sort_order": 0 },
    { "media_type": 2, "media_url": "https://cdn.gelsomino.cn/chat/yyy.mp4", "sort_order": 1 }
  ]
}
```

**返回**: 完整 Post 对象（含 author 和 media）。

---

### DELETE /api/feed/delete  🔒

删除自己的动态。

| 参数 | 类型 | 必填 |
|------|------|------|
| post_id | uint | 是 |

> 非本人动态返回 403。

---

### GET /api/feed/detail  🔒

查看动态详情（含点赞用户列表和评论）。

| 参数 | 类型 | 必填 |
|------|------|------|
| post_id | uint | 是（query） |

**返回**: 完整 Post 对象，包含 `likes`（点赞用户列表）和 `comments`（最近 3 条评论）。

---

### GET /api/feed/list  🔒

朋友圈列表，返回自己 + 所有好友的动态，按发布时间倒序。

| 参数 | 类型 | 必填 | 默认值 |
|------|------|------|--------|
| page | int | 否 | 1 |
| page_size | int | 否 | 20（最大 50） |

**返回**:
```json
{
  "list": [ Post, ... ],
  "total": 10,
  "page": 1,
  "page_size": 20
}
```

---

### GET /api/feed/my_posts  🔒

查看自己发布的所有动态。

| 参数 | 类型 | 必填 | 默认值 |
|------|------|------|--------|
| page | int | 否 | 1 |
| page_size | int | 否 | 20（最大 50） |

**返回**: 同上分页格式。

---

### POST /api/feed/like  🔒

点赞 / 取消点赞（Toggle 模式）。已赞则取消，未赞则点赞。

| 参数 | 类型 | 必填 |
|------|------|------|
| post_id | uint | 是 |

**返回**:
```json
{ "is_liked": true }   // true=已赞, false=已取消
```

> `like_count` 自动同步。

---

### POST /api/feed/comment  🔒

发表评论或回复他人评论。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| post_id | uint | 是 | |
| content | string | 是 | 评论内容 |
| reply_to_id | uint | 否 | 回复某条评论的 ID，传 null 或不传表示直接评论 |

**请求示例**:
```json
// 直接评论
{ "post_id": 1, "content": "写得真不错！" }

// 回复评论
{ "post_id": 1, "content": "谢谢支持！", "reply_to_id": 1 }
```

> `comment_count` 自动同步。

---

### DELETE /api/feed/comment  🔒

删除自己的评论。

| 参数 | 类型 | 必填 |
|------|------|------|
| comment_id | uint | 是 |

---

### GET /api/feed/comments  🔒

获取某条动态的评论列表，按时间正序。

| 参数 | 类型 | 必填 | 默认值 |
|------|------|------|--------|
| post_id | uint | 是（query） | |
| page | int | 否 | 1 |
| page_size | int | 否 | 20（最大 50） |

**返回**:
```json
{
  "list": [
    {
      "id": 3,
      "post_id": 1,
      "user_id": 24,
      "content": "谢谢支持！",
      "reply_to_id": 1,
      "created_at": "2026-05-31T08:57:20+08:00",
      "user": { "id": 24, "name": "未命名用户", "avatar": "" },
      "reply_to": {
        "id": 1,
        "content": "写得真不错！",
        "user": { "id": 24, "name": "未命名用户", "avatar": "" }
      }
    }
  ],
  "total": 3,
  "page": 1,
  "page_size": 20
}
```

> `reply_to` 仅在 `reply_to_id` 不为 null 时返回。

---

## 附录: 错误码

### HTTP 状态码

| HTTP | 说明 |
|------|------|
| 200 | 成功 |
| 400 | 参数错误 |
| 401 | 未授权 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 冲突（如忙线、版本冲突） |
| 428 | 需上传 E2EE 密钥盒子 |
| 500 | 服务端错误 |

### 业务码

| 业务码 | 说明 |
|--------|------|
| 400 | 请求参数错误 |
| 401 | 未授权 |
| 403 | 禁止访问 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |
| 10001 | 用户已存在 |
| 10002 | 用户不存在 |
| 10003 | 密码错误 |
| 10004 | Token 生成失败 |
| 10005 | Token 解析失败 |
| 10006 | Token 已过期 |

> 注：业务码 400/401/403/404/500 与 HTTP 状态码含义一致，用于响应 body 中的 `code` 字段。