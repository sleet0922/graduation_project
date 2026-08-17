# Graduation Project API

本文档对应当前后端路由实现，并于 2026-08-17 在公网环境逐项验证。

- HTTPS 基础地址：`https://mini.gelsomino.cn:444`
- WebSocket 基础地址：`wss://mini.gelsomino.cn:444`
- HTTP `81` 端口会重定向到 HTTPS `444`
- 数据格式：JSON，编码：UTF-8
- 实测范围：52 个 HTTP 接口、2 个 WebSocket 入口，共 79 项断言

## 1. 通用约定

### 1.1 HTTP 响应

除 `/health` 外，HTTP 接口统一返回：

```json
{
  "code": 200,
  "message": "操作结果",
  "data": {}
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `code` | int | `200` 表示成功；失败时可能是 HTTP 状态码或业务错误码 |
| `message` | string | 中文或英文结果说明 |
| `data` | any | 成功数据；无返回数据或失败时通常为 `null` |

常用 HTTP 状态码：

| HTTP 状态码 | 说明 |
| --- | --- |
| `200` | 请求成功；注册重复用户时 HTTP 仍可能为 200，但业务 `code=10001` |
| `400` | JSON、Query 或业务参数错误 |
| `401` | 缺少认证、Token 无效、密码错误或会话失效 |
| `403` | 无操作权限 |
| `404` | 用户、群聊、动态、密钥或通话不存在 |
| `409` | 状态冲突，例如 RTC 忙线或 E2EE 版本冲突 |
| `428` | 群密钥版本存在，但当前用户的密钥盒尚未发布 |
| `500` | 服务端或外部服务异常 |

业务错误码：

| `code` | 说明 |
| --- | --- |
| `10001` | 用户已存在 |
| `10002` | 用户不存在 |
| `10003` | 密码错误 |
| `10004` | Token 生成失败 |
| `10005` | Token 解析失败 |
| `10006` | Token 已过期 |

### 1.2 认证与会话

标记为“需要认证”的 HTTP 接口应携带：

```http
Authorization: Bearer <access_token>
```

WebSocket 可使用相同 Header，也可使用 `?token=<access_token>`。Access Token
不能替换为 Refresh Token。

| Token | 默认有效期 | 用途 |
| --- | --- | --- |
| Access Token | 86400 秒 | HTTP 和 WebSocket 业务请求 |
| Refresh Token | 2592000 秒 | 轮换 Access Token 和 Refresh Token |

每次登录会创建新的 `session_id` 并使该账号之前的会话失效。旧 HTTP 请求会收到
`401` 和 `账号在其他设备登录，请重新登录`；已连接的聊天 WebSocket 会先收到
`kicked` 事件再断开。

## 2. 路由总览

| 分类 | 方法 | 路径 | 认证 |
| --- | --- | --- | --- |
| 健康 | GET | `/health` | 否 |
| 用户 | POST | `/api/user/register` | 否 |
| 用户 | POST | `/api/user/login` | 否 |
| 用户 | POST | `/api/user/refresh` | 否 |
| 用户 | POST | `/api/user/self` | 是 |
| 用户 | GET | `/api/user/search` | 是 |
| 用户 | POST | `/api/user/name_update` | 是 |
| 用户 | POST | `/api/user/avatar_update` | 是 |
| 用户 | POST | `/api/user/password_update` | 是 |
| 用户 | POST | `/api/user/profile_update` | 是 |
| 用户 | POST | `/api/user/location` | 是 |
| 用户 | POST | `/api/user/delete` | 是 |
| OSS | GET | `/api/oss/upload-url` | 是 |
| OSS | GET | `/api/oss/download-url` | 否 |
| OSS | POST | `/api/chat/upload/image` | 是 |
| OSS | POST | `/api/chat/upload/video` | 是 |
| 好友 | POST | `/api/friend/request` | 是 |
| 好友 | GET | `/api/friend/requests` | 是 |
| 好友 | POST | `/api/friend/handle` | 是 |
| 好友 | GET | `/api/friend/list` | 是 |
| 好友 | POST | `/api/friend/check` | 是 |
| 好友 | POST | `/api/friend/remark_update` | 是 |
| 好友 | POST | `/api/friend/delete` | 是 |
| 群聊 | POST | `/api/group/create` | 是 |
| 群聊 | GET | `/api/group/list` | 是 |
| 群聊 | GET | `/api/group/members` | 是 |
| 群聊 | POST | `/api/group/member/add` | 是 |
| 群聊 | POST | `/api/group/member/remove` | 是 |
| 群聊 | POST | `/api/group/leave` | 是 |
| 群聊 | POST | `/api/group/delete` | 是 |
| E2EE | POST | `/api/e2ee/keys/publish` | 是 |
| E2EE | GET | `/api/e2ee/keys/public` | 是 |
| E2EE | GET | `/api/e2ee/group/key/current` | 是 |
| E2EE | GET | `/api/e2ee/group/key/by-version` | 是 |
| E2EE | POST | `/api/e2ee/group/key/publish` | 是 |
| E2EE | POST | `/api/e2ee/group/key/rotate` | 是 |
| RTC | POST | `/api/rtc/call/invite` | 是 |
| RTC | POST | `/api/rtc/call/accept` | 是 |
| RTC | POST | `/api/rtc/call/reject` | 是 |
| RTC | POST | `/api/rtc/call/cancel` | 是 |
| RTC | POST | `/api/rtc/call/hangup` | 是 |
| RTC | POST | `/api/rtc/token` | 是 |
| 动态 | POST | `/api/feed/create` | 是 |
| 动态 | DELETE | `/api/feed/delete` | 是 |
| 动态 | GET | `/api/feed/detail` | 是 |
| 动态 | GET | `/api/feed/list` | 是 |
| 动态 | GET | `/api/feed/my_posts` | 是 |
| 动态 | POST | `/api/feed/like` | 是 |
| 动态 | GET | `/api/feed/is_liked` | 是 |
| 动态 | POST | `/api/feed/comment` | 是 |
| 动态 | DELETE | `/api/feed/comment` | 是 |
| 动态 | GET | `/api/feed/comments` | 是 |
| WebSocket | GET | `/ws/chat` | 是 |
| WebSocket | GET | `/ws/online` | 是 |

## 3. 健康检查

### GET `/health`

无需认证。成功时 HTTP 200：

```json
{
  "status": "ok"
}
```

## 4. 用户接口

### POST `/api/user/register`

无需认证。系统生成 10 位数字账号，初始昵称为 `未命名用户`。

```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

| 字段 | 类型 | 必填 | 约束 |
| --- | --- | --- | --- |
| `email` | string | 是 | 必须符合邮箱格式且未注册 |
| `password` | string | 是 | 8 至 128 字符 |

成功 `data`：`{ id, account, name, email }`。

### POST `/api/user/login`

无需认证，`account` 可传邮箱或系统生成的数字账号。

```json
{
  "account": "user@example.com",
  "password": "password123"
}
```

成功 `data`：

```json
{
  "token": "<access_token>",
  "refresh_token": "<refresh_token>",
  "expires_in": 86400,
  "refresh_expires_in": 2592000,
  "session_id": "32位十六进制字符串",
  "user": {
    "id": 1,
    "account": "1234567890",
    "name": "未命名用户",
    "avatar": "",
    "email": "user@example.com",
    "gender": 0,
    "birthday": "",
    "location": ""
  }
}
```

### POST `/api/user/refresh`

无需 Access Token。

```json
{
  "refresh_token": "<refresh_token>"
}
```

成功 `data`：`{ token, refresh_token, expires_in, refresh_expires_in }`。每次成功
刷新都会轮换 Refresh Token，旧 Refresh Token 立即失效。

### POST `/api/user/self`

需要认证，无请求体。返回当前用户完整记录。用户业务字段使用小写键，例如
`name`、`account`、`email`、`avatar`、`gender`、`birthday`、`location` 和
`user_status`；GORM 基础字段为 `ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt`。

### GET `/api/user/search`

需要认证。

| Query | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `keyword` | string | 是 | 完整邮箱或完整数字账号，不是模糊搜索 |

成功 `data`：`{ id, account, name, avatar, email, gender, birthday, location }`。

### POST `/api/user/name_update`

需要认证。

```json
{
  "name": "新的昵称"
}
```

`name` 不能为空。成功 `data`：`{ id, name }`。

### POST `/api/user/avatar_update`

需要认证。

```json
{
  "avatar": "avatar_1.png"
}
```

该接口只保存 OSS 对象名，不负责上传文件。成功 `data`：`{ id, object_key }`。

### POST `/api/user/password_update`

需要认证。

```json
{
  "password": "old-password",
  "new_password": "new-password"
}
```

`new_password` 必须为 8 至 128 字符。成功后 `data=null`。

### POST `/api/user/profile_update`

需要认证。

```json
{
  "gender": 1,
  "birthday": "2000-01-02",
  "location": "Shanghai"
}
```

| 字段 | 类型 | 必填 | 约定 |
| --- | --- | --- | --- |
| `gender` | int | 否 | `0` 未知、`1` 男、`2` 女 |
| `birthday` | string | 否 | 建议使用 `YYYY-MM-DD` |
| `location` | string | 否 | 地区文本 |

成功 `data`：`{ id, gender, birthday, location }`。

### POST `/api/user/location`

需要认证。经纬度为 JSON 数字，其余字段可为空。

```json
{
  "latitude": 31.2304,
  "longitude": 121.4737,
  "province": "上海市",
  "city": "上海市",
  "district": "黄浦区",
  "address": "详细地址",
  "timestamp": 1786900000
}
```

该接口按用户覆盖保存最新位置，成功时 `data=null`。

### POST `/api/user/delete`

需要认证，无请求体。软删除当前用户，成功时 `data=null`。

## 5. OSS 与文件上传

### GET `/api/oss/upload-url`

需要认证，生成有效期 1 小时的预签名 PUT URL。

| Query | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `key` | string | 是 | 1 至 180 字符，只允许字母、数字、`.`、`_`、`-`，不能含 `..` |
| `type` | string | 否 | `avatar`、`chat`、`video`、`feed`；默认 `chat` |

目录映射：`avatar -> avatar/`，`chat/video -> chat/`，`feed -> feed/`。

成功 `data`：

```json
{
  "upload_url": "https://...",
  "access_url": "http://cdn.gelsomino.cn/...",
  "expires_in": "1小时"
}
```

客户端应使用 `PUT <upload_url>` 上传文件内容。

### GET `/api/oss/download-url`

无需认证。

| Query | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `key` | string | 是 | 对象名或完整对象路径 |

以 `avatar_`、`chat_`、`feed_` 开头的对象名会自动补对应目录，其余值原样使用。
成功 `data`：`{ download_url, expires_in: "1小时" }`。

### POST `/api/chat/upload/image`

需要认证，Content-Type 为 `multipart/form-data`，表单字段名固定为 `file`。

- 最大 10 MB
- MIME 必须以 `image/` 开头，或为 `application/octet-stream`
- 对象保存到 `chat/<user_id>/`

成功 `data`：`{ url, content, filename, contentType }`，其中 `content` 与 `url` 相同。

### POST `/api/chat/upload/video`

需要认证，表单字段同图片上传。

- 最大 100 MB
- MIME 必须以 `video/` 开头，或为 `application/octet-stream`

成功 `data`：`{ url, content, filename, contentType }`。

## 6. 好友接口

### POST `/api/friend/request`

需要认证，`friend_id` 与 `account` 二选一；若两者都传，优先使用 `friend_id`。

```json
{
  "account": "friend@example.com"
}
```

`account` 支持邮箱或数字账号。不能添加自己、已有好友或重复发送待处理申请。

### GET `/api/friend/requests`

需要认证，返回当前用户收到的申请数组。每项包含 GORM 基础字段
`ID/CreatedAt/UpdatedAt/DeletedAt`、`sender_id`、`receiver_id`、`status` 和
`sender` 用户对象。

`status`：`0` 待处理、`1` 已接受、`2` 已拒绝。

### POST `/api/friend/handle`

需要认证，仅申请接收方可操作。

```json
{
  "request_id": 1,
  "status": 1
}
```

`status` 仅支持 `1` 接受或 `2` 拒绝。

### GET `/api/friend/list`

需要认证，返回：

```text
[{ id, user_id, friend_id, account, name, email, avatar,
   gender, birthday, location, remark }]
```

### POST `/api/friend/check`

需要认证。

```json
{
  "friend_id": 2
}
```

成功 `data`：`{ is_friend: true }` 或 `{ is_friend: false }`。

### POST `/api/friend/remark_update`

需要认证，`remark` 可为空以清除备注。

```json
{
  "friend_id": 2,
  "remark": "同学"
}
```

### POST `/api/friend/delete`

需要认证，会删除双向好友关系。

```json
{
  "friend_id": 2
}
```

## 7. 群聊接口

群聊成员变更会轮换 E2EE 群密钥版本，并通过聊天 WebSocket 推送系统事件。

### POST `/api/group/create`

需要认证。`member_ids` 实际必须包含至少一位好友，不能创建只有群主的群聊。

```json
{
  "name": "项目群",
  "avatar": "group.png",
  "member_ids": [2, 3]
}
```

系统去除 `0`、群主 ID 和重复 ID；其余成员必须存在且均为群主好友。

成功 `data`：`{ id, name, avatar, owner_id, member_count, created_at, updated_at }`。
创建时群主角色为 `owner`，其他成员角色为 `member`，群密钥初始版本为 `1`。

### GET `/api/group/list`

需要认证，返回当前用户所在群聊的详情数组，字段同创建接口。

### GET `/api/group/members`

需要认证，调用者必须是群成员。

| Query | 类型 | 必填 |
| --- | --- | --- |
| `group_id` | uint | 是 |

成功 `data`：`[{ user_id, account, name, email, avatar, role }]`。

### POST `/api/group/member/add`

需要认证，调用者必须已经在群内；被邀请者必须是调用者的好友。

```json
{
  "group_id": 1,
  "member_ids": [3, 4]
}
```

新增至少一名成员后会轮换群密钥。成功返回更新后的完整群成员数组。

### POST `/api/group/member/remove`

需要认证，仅群主可操作，不能移除群主。

```json
{
  "group_id": 1,
  "member_id": 3
}
```

成功后轮换群密钥，并向被移除成员推送 `group_member_removed`。

### POST `/api/group/leave`

需要认证，仅普通成员可以退出；群主必须解散群聊。

```json
{
  "group_id": 1
}
```

成功后轮换群密钥，并向群主推送 `group_member_left`。

### POST `/api/group/delete`

需要认证，仅群主可解散。

```json
{
  "group_id": 1
}
```

所有在线成员会收到 `{ "type": "group_dissolved", "group_id": 1 }`。

## 8. E2EE 接口

### 8.1 基本约定

- 身份密钥类型固定为 X25519
- `public_key` 为 Base64，解码后必须恰好 32 字节
- 群密钥包装算法固定为 `chacha20poly1305-v1`
- `wrapped_group_key` 为 Base64，解码后必须大于 16 字节
- `wrap_nonce` 为 Base64，解码后必须恰好 12 字节
- 发布群密钥盒时必须恰好覆盖当前所有群成员，不能缺少、重复或包含非成员
- 创建群聊、成员变更、主动轮换或成员身份公钥变化都会生成新群密钥版本

Base64 接受标准或 URL-safe 形式，可带或不带 padding。

### POST `/api/e2ee/keys/publish`

需要认证，发布或替换自己的身份公钥。

```json
{
  "key_type": "x25519",
  "public_key": "<base64-encoded-32-bytes>"
}
```

成功 `data`：`{ user_id, key_type, updated_at }`。公钥真实变化时，用户所在群聊的
群密钥会自动轮换。

### GET `/api/e2ee/keys/public`

需要认证。

| Query | 类型 | 必填 |
| --- | --- | --- |
| `user_id` | uint | 是 |

成功 `data`：`{ user_id, key_type, public_key, updated_at }`。

### GET `/api/e2ee/group/key/current`

需要认证，调用者必须是群成员。

| Query | 类型 | 必填 |
| --- | --- | --- |
| `group_id` | uint | 是 |

正常成功 `data`：

```json
{
  "group_id": 1,
  "key_version": 2,
  "target_user_id": 3,
  "wrapped_group_key": "<base64>",
  "wrap_nonce": "<base64>",
  "wrapped_by_user_id": 1,
  "key_wrap_alg": "chacha20poly1305-v1"
}
```

版本已创建但当前用户没有密钥盒时返回 HTTP 428：

```json
{
  "code": 428,
  "message": "e2ee group key box not found, please upload key boxes",
  "data": {
    "group_id": 1,
    "key_version": 2,
    "need_publish": true
  }
}
```

### GET `/api/e2ee/group/key/by-version`

需要认证，读取当前用户在指定历史版本中的密钥盒。

| Query | 类型 | 必填 |
| --- | --- | --- |
| `group_id` | uint | 是 |
| `key_version` | int | 是，必须大于 0 |

成功字段与当前密钥接口一致，但该接口当前不返回 `key_wrap_alg`。

### POST `/api/e2ee/group/key/publish`

需要认证，只能发布当前版本，且同一版本只能成功发布一次。

```json
{
  "group_id": 1,
  "key_version": 2,
  "key_wrap_alg": "chacha20poly1305-v1",
  "boxes": [
    {
      "user_id": 1,
      "wrapped_group_key": "<base64>",
      "wrap_nonce": "<base64>"
    },
    {
      "user_id": 2,
      "wrapped_group_key": "<base64>",
      "wrap_nonce": "<base64>"
    }
  ]
}
```

`key_wrap_alg` 省略时默认 `chacha20poly1305-v1`。成功 `data`：
`{ group_id, key_version, box_count }`。

### POST `/api/e2ee/group/key/rotate`

需要认证，调用者必须是群成员，并通过期望版本执行并发保护。

```json
{
  "group_id": 1,
  "expected_key_version": 2
}
```

成功 `data`：`{ group_id, key_version: 3 }`。版本不匹配返回 HTTP 409。

## 9. 聊天 WebSocket

### 9.1 连接

```text
wss://mini.gelsomino.cn:444/ws/chat?client=foreground
```

| Query | 可选值 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `client` | `foreground`、`background` | `foreground` | 前台连接会拉取离线消息；后台连接不会 |
| `token` | Access Token | 无 | 未使用 Authorization Header 时可传 |

连接成功：

```json
{
  "type": "connected",
  "user_id": 1
}
```

应用层心跳请求 `{ "type": "ping" }`，响应 `{ "type": "pong" }`。

### 9.2 E2EE 单聊消息

除 `message_type=call` 或 `message_type=video` 外，服务端要求消息 `content` 是合法
E2EE 信封的 JSON 字符串。直接发送普通明文会返回 `端到端加密消息格式无效`。

外层消息：

```json
{
  "type": "chat",
  "to_user_id": 2,
  "message_type": "text",
  "content": "{\"e2ee\":1,\"v\":\"x25519+chacha20poly1305:v1\",...}",
  "client_message_id": "client-uuid"
}
```

单聊 E2EE 信封：

```json
{
  "e2ee": 1,
  "v": "x25519+chacha20poly1305:v1",
  "key_id": "16位小写十六进制字符串",
  "sender_key_id": "发送方公钥SHA-256，64位小写十六进制",
  "recipient_key_id": "接收方公钥SHA-256，64位小写十六进制",
  "nonce": "<base64-encoded-12-bytes>",
  "ct": "<base64-ciphertext-longer-than-16-bytes>"
}
```

发送方收到：

```json
{
  "type": "sent",
  "message": {
    "id": "...",
    "conversation_type": "single",
    "from_user_id": 1,
    "to_user_id": 2,
    "group_id": 0,
    "message_type": "text",
    "content": "...",
    "created_at": "..."
  },
  "client_message_id": "client-uuid"
}
```

接收方收到 `type=chat` 和相同 `message`；补发的离线消息还包含 `offline=true`。

### 9.3 E2EE 群聊消息

外层使用 `group_id` 替换 `to_user_id`。当前群密钥盒必须已完整发布。

```json
{
  "e2ee": 1,
  "v": "group+chacha20poly1305:v1",
  "scope": "group",
  "group_id": 1,
  "key_version": 2,
  "sender_key_id": "发送方公钥SHA-256",
  "nonce": "<base64-encoded-12-bytes>",
  "ct": "<base64-ciphertext-longer-than-16-bytes>"
}
```

版本不是当前版本时返回 `群聊密钥版本已更新，请重新加密后重试`；密钥盒未覆盖全员时
返回 `群聊当前密钥尚未完成全员分发，请稍后重试`。

### 9.4 已读与撤回

单聊已读：

```json
{
  "type": "mark_read",
  "to_user_id": 1
}
```

对端收到：

```json
{
  "type": "read_ack",
  "reader_id": 2,
  "peer_id": 1
}
```

群聊已读使用 `group_id`。撤回仅允许原发送者在消息发出后 1 分钟内操作：

```json
{
  "type": "recall",
  "to_user_id": 2,
  "message_id": "服务端消息ID"
}
```

接收方收到 `{ "type": "message_recalled", "message_id": "...", "from_user": 1 }`。
群消息撤回改传 `group_id`。

### 9.5 系统事件与错误

常见系统事件：

| `type` | 说明 |
| --- | --- |
| `friend_request` | 收到好友申请 |
| `friend_accepted` | 好友申请被接受 |
| `group_member_added` | 被加入群聊 |
| `group_member_removed` | 被移出群聊 |
| `group_member_left` | 群成员退出，通知群主 |
| `group_dissolved` | 群聊解散 |
| `e2ee_group_key_changed` | 群密钥版本变化 |
| `kicked` | 新登录导致当前会话失效 |
| `rtc_*` | RTC 通话信令，见 RTC 章节 |

业务错误使用 `{ "type": "error", "error": "错误说明", "client_message_id": "..." }`。

## 10. 在线状态 WebSocket

连接地址：

```text
wss://mini.gelsomino.cn:444/ws/online
```

认证方式同聊天 WebSocket。连接成功和应用层 ping/pong 格式也相同。

查询单个用户：

```json
{
  "type": "check_online",
  "user_id": 2
}
```

响应：`{ "type": "online_status", "user_id": 2, "online": true }`。

查询多个用户：

```json
{
  "type": "check_online",
  "user_ids": [2, 3]
}
```

响应：

```json
{
  "type": "online_status",
  "statuses": [
    { "user_id": 2, "online": true },
    { "user_id": 3, "online": false }
  ]
}
```

在线状态依据用户是否持有聊天 WebSocket 连接，而不是 `/ws/online` 连接本身。

## 11. RTC 通话接口

RTC 呼叫状态保存在后端进程内存中；后端重启后未结束的 `call_id` 不再有效。被叫必须
保持聊天 WebSocket 在线才能收到邀请。单聊要求双方为好友，群聊要求发起者为群成员且
至少有一名其他成员在线。

### POST `/api/rtc/call/invite`

需要认证，`peer_id` 与 `group_id` 必须二选一。

```json
{
  "peer_id": 2,
  "call_type": "video"
}
```

`call_type` 仅支持 `voice` 或 `video`。成功 `data`：

```json
{
  "call_id": "call_20260817010101001",
  "room_id": "rtc_room_20260817010101001",
  "call_type": "video",
  "peer_id": 2,
  "group_id": 0
}
```

45 秒内无人接听会释放通话并推送 `rtc_timeout`。单聊被叫离线返回 HTTP 409。

被叫收到：

```json
{
  "type": "rtc_invite",
  "call_id": "...",
  "room_id": "...",
  "call_type": "video",
  "from_user_id": 1,
  "to_user_id": 2,
  "from_name": "昵称",
  "avatar": "头像"
}
```

群邀请还包含 `group_id`。

### POST `/api/rtc/call/accept`

需要认证，仅受邀用户可操作。

```json
{
  "call_id": "call_..."
}
```

成功 `data`：`{ call_id, room_id }`，其他参与者收到 `rtc_accept`。

### POST `/api/rtc/call/reject`

需要认证，仅尚未接听的受邀用户可操作。

```json
{
  "call_id": "call_...",
  "reason": "rejected"
}
```

`reason=busy` 会向主叫推送 `rtc_busy`，其他值统一为 `rejected` 并推送 `rtc_reject`。

### POST `/api/rtc/call/cancel`

需要认证，仅主叫可在接通前取消。

```json
{
  "call_id": "call_..."
}
```

被叫收到 `rtc_cancel`。

### POST `/api/rtc/call/hangup`

需要认证，已接通后主叫或已接听参与者可挂断。未接通的主叫必须使用取消接口。

```json
{
  "call_id": "call_..."
}
```

其他参与者收到 `rtc_hangup`。

### POST `/api/rtc/token`

需要认证，只为当前通话参与者生成 LiveKit Token。

```json
{
  "call_id": "call_...",
  "room_id": "rtc_room_...",
  "call_type": "video",
  "peer_id": 2
}
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `call_id` | 是 | 当前有效通话 ID |
| `call_type` | 是 | 必须与邀请一致 |
| `room_id` | 否 | 传入时必须与通话一致 |
| `peer_id` | 否 | 单聊传入时必须与通话一致 |
| `group_id` | 否 | 群聊传入时必须与通话一致 |

成功 `data`：`{ url, room_id, uid, token }`，其中 `url` 是 LiveKit WebSocket 地址，
`uid` 是当前用户数字 ID 的字符串形式。

## 12. 动态接口

动态只对发布者本人及其好友可见。分页 `page` 默认 `1`，`page_size` 默认 `20`；
`page_size` 小于等于 0 或大于 50 时，查询内部使用 20。

### 12.1 动态数据结构

核心字段：

```json
{
  "id": 1,
  "user_id": 1,
  "content": "动态内容",
  "author": {
    "name": "昵称",
    "account": "1234567890",
    "avatar": "avatar.png"
  },
  "media": [
    {
      "id": 1,
      "post_id": 1,
      "media_type": 1,
      "media_url": "https://...",
      "sort_order": 0
    }
  ],
  "like_count": 0,
  "comment_count": 0,
  "created_at": "...",
  "updated_at": "..."
}
```

`media_type` 约定：`1` 图片、`2` 视频。

### POST `/api/feed/create`

需要认证，`content` 和 `media` 至少有一个非空。

```json
{
  "content": "分享图片",
  "media": [
    {
      "media_type": 1,
      "media_url": "https://cdn.example/image.jpg",
      "sort_order": 0
    }
  ]
}
```

成功 `data` 为完整动态对象。

### DELETE `/api/feed/delete`

需要认证，仅动态作者可删除。

```json
{
  "post_id": 1
}
```

不存在返回 404，非作者返回 403。

### GET `/api/feed/detail`

需要认证。

| Query | 类型 | 必填 |
| --- | --- | --- |
| `post_id` | uint | 是 |

成功 `data` 的实际结构是 `{ post: <动态对象>, is_liked: bool }`。评论不嵌入动态详情，
应通过 `/api/feed/comments` 单独读取。

### GET `/api/feed/list`

需要认证，返回本人及好友的动态，按时间倒序。

| Query | 类型 | 必填 | 默认值 |
| --- | --- | --- | --- |
| `page` | int | 否 | `1` |
| `page_size` | int | 否 | `20` |

成功 `data`：`{ list, total, page, page_size }`。每个列表项包含动态字段和
`is_liked`。

### GET `/api/feed/my_posts`

需要认证，分页参数与动态列表相同，只返回当前用户的动态。响应结构同动态列表。

### POST `/api/feed/like`

需要认证，为 Toggle 操作：未点赞时点赞，已点赞时取消。

```json
{
  "post_id": 1
}
```

成功 `data`：`{ is_liked: true }` 或 `{ is_liked: false }`，并同步更新
`like_count`。

### GET `/api/feed/is_liked`

需要认证。

| Query | 类型 | 必填 |
| --- | --- | --- |
| `post_id` | uint | 是 |

成功 `data`：`{ is_liked: bool }`。

### POST `/api/feed/comment`

需要认证。

```json
{
  "post_id": 1,
  "content": "评论内容",
  "reply_to_id": null
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `post_id` | uint | 是 | 可见动态 ID |
| `content` | string | 是 | 不能为空 |
| `reply_to_id` | uint/null | 否 | 被回复评论 ID |

成功 `data` 为新评论对象，并同步更新 `comment_count`。

### DELETE `/api/feed/comment`

需要认证。评论作者或动态作者可删除评论。

```json
{
  "comment_id": 1
}
```

### GET `/api/feed/comments`

需要认证，调用者必须可见该动态。

| Query | 类型 | 必填 | 默认值 |
| --- | --- | --- | --- |
| `post_id` | uint | 是 | 无 |
| `page` | int | 否 | `1` |
| `page_size` | int | 否 | `20` |

成功 `data`：`{ list, total, page, page_size }`，评论按创建时间正序。

## 13. 实测说明

2026-08-17 使用三个唯一测试账号在 `https://mini.gelsomino.cn:444` 完成以下闭环：

1. 注册、登录、刷新、用户资料、位置、改密与重新登录。
2. OSS 上传/下载 URL、图片直传与视频直传。
3. 好友申请、接受、列表、检查、备注和双向删除。
4. 群聊创建、列表、成员查询、添加、移除、主动退出和解散。
5. 身份公钥发布、群密钥 428 恢复流程、密钥盒发布、历史版本读取和主动轮换。
6. 双端聊天 WebSocket 连接、E2EE 消息、发送回执、投递、已读和撤回。
7. 在线状态 WebSocket 的连接、心跳和批量查询。
8. RTC 邀请、接受、Token、挂断、拒绝和取消，并核对对应 WebSocket 事件。
9. 动态发布、详情、朋友圈、我的动态、点赞状态、评论、删除评论和删除动态。
10. 解散测试群聊、删除测试好友关系并软删除三个测试账号。

所有 52 个 HTTP 路由和 2 个 WebSocket 入口均通过，公网健康检查为 HTTP 200。
