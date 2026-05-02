# ZAT API 文档

> 基础地址: `https://api.gelsomino.cn`  
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
| Access Token | 24h | 访问业务接口 |
| Refresh Token | 30d | 刷新 Access Token |

---

## 一、用户

### POST /api/user/register

注册账号。系统自动生成 10 位数字账号。

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
  "user": {
    "id": 1, "account": "...", "name": "...", "avatar": "...",
    "email": "...", "gender": 0, "birthday": "", "location": ""
  }
}
```

---

### POST /api/user/refresh

| 参数 | 类型 | 必填 |
|------|------|------|
| refresh_token | string | 是 |

**返回** `{ token, refresh_token, expires_in, refresh_expires_in }`

---

### POST /api/user/self  🔒

获取当前登录用户完整信息。

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

---

### POST /api/user/avatar_update  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| avatar | string | 是 |

值为 OSS 上传返回的文件名（如 `avatar_6_1776183103821.jpg`）。

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

| 参数 | 类型 | 必填 |
|------|------|------|
| file | file | 是 |

---

### POST /api/chat/upload/video  🔒

直传聊天视频（multipart/form-data）。

| 参数 | 类型 | 必填 |
|------|------|------|
| file | file | 是 |

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
wss://api.gelsomino.cn/ws/chat
```

认证: Header `Authorization: Bearer <token>`（主）或 `?token=<token>`（备）。

连接成功收到 `{ "type": "connected", "user_id": N }`

### 心跳

服务端每 5s 发 Ping 帧，3s 内未收到 Pong 则断开。

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

### 错误

```json
{ "type": "error", "error": "错误描述" }
```

常见: `只能给好友发送消息` `消息内容不能为空` `接收方或群聊不能为空`

---

## 六、WebSocket 在线状态

### 连接

```
wss://api.gelsomino.cn/ws/online
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

**正常返回** `{ group_id, key_version, wrapped_group_key, wrap_nonce, wrapped_by_user_id, key_wrap_alg }`

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

---

### GET /api/e2ee/group/key/by-version  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| group_id | uint | 是（query） |
| key_version | int | 是（query） |

---

## 八、RTC 实时通话

### POST /api/rtc/call/invite  🔒

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| call_type | string | 是 | `"audio"` / `"video"` |
| peer_id | uint | 单聊必填 | 与 group_id 二选一 |
| group_id | uint | 群聊必填 | 与 peer_id 二选一 |

**返回** `{ call_id, room_id, call_type, peer_id, group_id }`

---

### POST /api/rtc/call/accept  🔒

| 参数 | 类型 | 必填 |
|------|------|------|
| call_id | string | 是 |

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

---

### POST /api/rtc/token  🔒

获取 RTC 房间 Token。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| call_id | string | 是 | |
| call_type | string | 是 | `"audio"` / `"video"` |
| room_id | string | 否 | |
| peer_id | uint | 否 | |
| group_id | uint | 否 | |

**返回** `{ app_id, room_id, uid, token }`

---

## 附录: 错误码

| HTTP | 说明 |
|------|------|
| 200 | 成功 |
| 400 | 参数错误 |
| 401 | 未授权 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 冲突 |
| 428 | 需上传 E2EE 密钥盒子 |
| 500 | 服务端错误 |

| 业务码 | 说明 |
|--------|------|
| 10001 | 用户已存在 |
| 10002 | 用户不存在 |
| 10003 | 密码错误 |
| 10004 | Token 生成失败 |
| 10005 | Token 解析失败 |
| 10006 | Token 已过期 |
| 20001 | 好友已存在 |
| 20002 | 好友不存在 |
| 20003 | 好友请求处理失败 |
| 30001 | 文件上传失败 |
| 30002 | 文件下载失败 |
