# ZAT 即时通讯系统 API 文档

> 版本: v1.0  
> 基础地址: `https://api.gelsomino.cn:444`  
> 文档更新时间: 2026-04-24

---

## 目录

1. [概述](#概述)
2. [认证机制](#认证机制)
3. [用户相关 API](#用户相关-api)
4. [OSS 文件存储 API](#oss-文件存储-api)
5. [好友相关 API](#好友相关-api)
6. [群聊相关 API](#群聊相关-api)
7. [WebSocket 实时聊天](#websocket-实时聊天)
8. [WebSocket 在线状态](#websocket-在线状态)
9. [E2EE 端到端加密 API](#e2ee-端到端加密-api)
10. [RTC 实时通话 API](#rtc-实时通话-api)
11. [附录](#附录)

---

## 概述

### 基础信息

| 项目 | 说明 |
|------|------|
| 协议 | HTTPS / WSS |
| 数据格式 | JSON |
| 字符编码 | UTF-8 |
| 时区 | 服务器使用 UTC，返回时间字符串带时区信息 |

### 通用响应格式

所有 API 响应均遵循以下格式：

```json
{
  "code": 200,
  "data": {},
  "message": "操作成功"
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | 业务状态码，200 表示成功 |
| `data` | any | 响应数据，失败时可能为 null |
| `message` | string | 提示信息 |

### 认证方式

需要认证的接口必须在请求头中携带：

```
Authorization: Bearer <token>
```

---

## 认证机制

### Token 体系

系统采用双 Token 机制：

| Token 类型 | 有效期 | 用途 |
|------------|--------|------|
| Access Token | 1 天 | 访问受保护接口 |
| Refresh Token | 30 天 | 刷新 Access Token |

### Token 刷新流程

```
┌─────────────┐     Access Token 过期      ┌─────────────┐
│   客户端     │ ─────────────────────────> │   服务端     │
└─────────────┘                            └─────────────┘
       │                                          │
       │  使用 Refresh Token 调用 /api/user/refresh │
       │<───────────────────────────────────────────┘
       │                                          │
       │        返回新的 Access & Refresh Token    │
       │<───────────────────────────────────────────┘
```

---

## 用户相关 API

### 1. 用户注册

**接口地址：** `POST /api/user/register`

**认证要求：** 否

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `email` | string | 是 | 邮箱地址，用于登录 |
| `password` | string | 是 | 密码，建议 6-20 位 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "your_password"
  }'
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "id": 8,
    "account": "0762353747",
    "email": "user@example.com",
    "name": "未命名用户"
  },
  "message": "注册成功"
}
```

**说明：**
- 注册成功后系统自动生成 10 位随机数字账号
- 账号用于好友搜索，邮箱用于登录

---

### 2. 用户登录

**接口地址：** `POST /api/user/login`

**认证要求：** 否

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `account` | string | 是 | 邮箱或 10 位数字账号 |
| `password` | string | 是 | 密码 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "account": "user@example.com",
    "password": "your_password"
  }'
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 86400,
    "refresh_expires_in": 2592000,
    "user": {
      "id": 8,
      "account": "0762353747",
      "name": "未命名用户",
      "avatar": "",
      "email": "user@example.com",
      "gender": 0,
      "birthday": "",
      "location": ""
    }
  },
  "message": "登录成功"
}
```

**用户对象字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 用户唯一 ID |
| `account` | string | 10 位数字账号 |
| `name` | string | 用户昵称 |
| `avatar` | string | 头像 URL 或文件名 |
| `email` | string | 邮箱地址 |
| `gender` | int | 性别：0-未知，1-男，2-女 |
| `birthday` | string | 生日，格式 YYYY-MM-DD |
| `location` | string | 地区 |

---

### 3. 刷新 Token

**接口地址：** `POST /api/user/refresh`

**认证要求：** 否

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `refresh_token` | string | 是 | 登录时返回的 refresh token |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "your_refresh_token"
  }'
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "token": "新的access_token",
    "refresh_token": "新的refresh_token",
    "expires_in": 86400,
    "refresh_expires_in": 2592000
  },
  "message": "刷新token成功"
}
```

---

### 4. 获取当前用户信息

**接口地址：** `POST /api/user/self`

**认证要求：** 是

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/self \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN"
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "ID": 6,
    "name": "张三",
    "account": "6158726193",
    "email": "user@example.com",
    "avatar": "avatar_6_1776183103821.jpg",
    "gender": 1,
    "birthday": "2000-01-01",
    "location": "北京",
    "user_status": 0
  },
  "message": "获取用户信息成功"
}
```

---

### 5. 搜索用户

**接口地址：** `GET /api/user/search`

**认证要求：** 否

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 是 | 邮箱或 10 位数字账号 |

**请求示例：**

```bash
curl -X GET "https://api.gelsomino.cn:444/api/user/search?keyword=user@example.com"
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "id": 8,
    "account": "0762353747",
    "name": "张三",
    "avatar": "avatar_8_1776183103821.jpg",
    "email": "user@example.com",
    "gender": 1,
    "birthday": "2000-01-01",
    "location": "北京"
  },
  "message": "搜索用户成功"
}
```

---

### 6. 更新用户名

**接口地址：** `POST /api/user/name_update`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 新用户名 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/name_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "张三"}'
```

---

### 7. 更新用户头像

**接口地址：** `POST /api/user/avatar_update`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `avatar` | string | 是 | 头像文件名或完整 URL |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/avatar_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"avatar": "avatar_6_1776183103821.jpg"}'
```

**说明：**
- 建议先调用 `/api/oss/upload-url` 上传头像图片
- 上传成功后使用返回的文件名作为 avatar 值

---

### 8. 更新密码

**接口地址：** `POST /api/user/password_update`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `password` | string | 是 | 原密码 |
| `new_password` | string | 是 | 新密码 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/password_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "password": "old_password",
    "new_password": "new_password"
  }'
```

---

### 9. 更新用户资料

**接口地址：** `POST /api/user/profile_update`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `gender` | int | 否 | 性别：0-未知，1-男，2-女 |
| `birthday` | string | 否 | 生日，格式 YYYY-MM-DD |
| `location` | string | 否 | 地区 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/profile_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "gender": 1,
    "birthday": "2000-01-01",
    "location": "北京"
  }'
```

---

### 10. 删除用户/注销账号

**接口地址：** `POST /api/user/delete`

**认证要求：** 是

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/user/delete \
  -H "Authorization: Bearer $TOKEN"
```

---

## OSS 文件存储 API

### 存储路径说明

| 文件类型 | 存储路径 | 示例 |
|----------|----------|------|
| 头像 | `avatar/` | `avatar/avatar_6_1776183103821.jpg` |
| 聊天图片 | `chat/` | `chat/chat_6_1776183103821_0.jpg` |
| 聊天视频 | `chat/` | `chat/chat_6_1776183103821_0.mp4` |

### 11. 获取上传 URL

**接口地址：** `GET /api/oss/upload-url`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `key` | string | 是 | 文件名 |
| `type` | string | 否 | 文件类型：`avatar`(头像)、`chat`(聊天图片) 或 `video`(聊天视频)，默认 `chat` |

**请求示例：**

```bash
# 上传头像
curl -X GET "https://api.gelsomino.cn:444/api/oss/upload-url?key=avatar_6_1776183103821.jpg&type=avatar" \
  -H "Authorization: Bearer $TOKEN"

# 上传聊天图片
curl -X GET "https://api.gelsomino.cn:444/api/oss/upload-url?key=chat_6_1776183103821_0.jpg&type=chat" \
  -H "Authorization: Bearer $TOKEN"

# 上传聊天视频
curl -X GET "https://api.gelsomino.cn:444/api/oss/upload-url?key=chat_6_1776183103821_0.mp4&type=video" \
  -H "Authorization: Bearer $TOKEN"
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "upload_url": "https://sleet.s3-cn-east-1.qiniucs.com/avatar/avatar_6_1776183103821.jpg?X-Amz-Algorithm=...",
    "access_url": "https://cdn.gelsomino.cn/avatar/avatar_6_1776183103821.jpg",
    "expires_in": "1小时"
  },
  "message": "获取上传URL成功"
}
```

**字段说明：**

| 字段 | 说明 |
|------|------|
| `upload_url` | 预签名上传 URL，直接使用 PUT 方法上传文件 |
| `access_url` | 文件访问 URL，上传成功后可通过此 URL 访问 |
| `expires_in` | 上传 URL 有效期 |

**上传流程：**

```
1. 调用 /api/oss/upload-url 获取预签名上传 URL
2. 使用 PUT 方法将文件上传到 upload_url
3. 使用 access_url 作为文件的访问地址
```

---

### 12. 获取下载 URL

**接口地址：** `GET /api/oss/download-url`

**认证要求：** 否

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `key` | string | 是 | 文件名 |

**请求示例：**

```bash
curl -X GET "https://api.gelsomino.cn:444/api/oss/download-url?key=avatar_6_1776183103821.jpg"
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "download_url": "https://sleet.s3-cn-east-1.qiniucs.com/avatar/avatar_6_1776183103821.jpg?X-Amz-Algorithm=...",
    "expires_in": "1小时"
  },
  "message": "获取下载URL成功"
}
```

**说明：**
- 后端会自动根据 key 前缀添加路径（`avatar_` → `avatar/`，`chat_` → `chat/`）
- 下载 URL 有效期为 1 小时

---

### 13. 上传聊天图片（直传）

**接口地址：** `POST /api/chat/upload/image`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | 图片文件，支持常见 image/* 类型，最大 10MB |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/chat/upload/image \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./test.png"
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "url": "https://cdn.gelsomino.cn/chat/chat_6_1776183103821_0.png",
    "content": "https://cdn.gelsomino.cn/chat/chat_6_1776183103821_0.png",
    "filename": "test.png",
    "contentType": "image/png"
  },
  "message": "上传聊天图片成功"
}
```

---

### 14. 上传聊天视频（直传）

**接口地址：** `POST /api/chat/upload/video`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | 视频文件，支持常见 video/* 类型，最大 100MB |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/chat/upload/video \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./test.mp4"
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "url": "https://cdn.gelsomino.cn/chat/chat_6_1776183103821_0.mp4",
    "content": "https://cdn.gelsomino.cn/chat/chat_6_1776183103821_0.mp4",
    "filename": "test.mp4",
    "contentType": "video/mp4"
  },
  "message": "上传聊天视频成功"
}
```

---

## 好友相关 API

### 15. 发送好友请求

**接口地址：** `POST /api/friend/request`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `account` | string | 可选 | 好友的邮箱或账号 |
| `friend_id` | uint | 可选 | 好友用户 ID，与 account 二选一 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/friend/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"account": "user@example.com"}'
```

**边界场景响应：**

| 场景 | 响应码 | 消息 |
|------|--------|------|
| 添加自己 | 400 | 不能添加自己为好友 |
| 已是好友 | 400 | 你们已经是好友了 |
| 申请已存在 | 400 | 好友申请已存在 |

---

### 16. 获取好友请求列表

**接口地址：** `GET /api/friend/requests`

**认证要求：** 是

**响应示例：**

```json
{
  "code": 200,
  "data": [
    {
      "ID": 1,
      "CreatedAt": "2026-03-25T01:23:45Z",
      "sender_id": 8,
      "receiver_id": 9,
      "status": 0
    }
  ],
  "message": "获取好友申请列表成功"
}
```

**状态说明：**

| 状态值 | 含义 |
|--------|------|
| 0 | 待处理 |
| 1 | 已接受 |
| 2 | 已拒绝 |

---

### 17. 处理好友申请

**接口地址：** `POST /api/friend/handle`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `request_id` | uint | 是 | 申请记录 ID |
| `status` | int | 是 | 1-接受，2-拒绝 |

**请求示例：**

```bash
# 接受申请
curl -X POST https://api.gelsomino.cn:444/api/friend/handle \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"request_id": 1, "status": 1}'

# 拒绝申请
curl -X POST https://api.gelsomino.cn:444/api/friend/handle \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"request_id": 1, "status": 2}'
```

---

### 18. 获取好友列表

**接口地址：** `GET /api/friend/list`

**认证要求：** 是

**响应示例：**

```json
{
  "code": 200,
  "data": [
    {
      "id": 1,
      "user_id": 8,
      "friend_id": 9,
      "account": "9395046534",
      "name": "李四",
      "email": "friend@example.com",
      "avatar": "avatar_9_1776183103821.jpg",
      "gender": 1,
      "birthday": "2000-01-01",
      "location": "上海",
      "remark": "同事"
    }
  ],
  "message": "获取好友列表成功"
}
```

---

### 19. 检查好友关系

**接口地址：** `POST /api/friend/check`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `friend_id` | uint | 是 | 待检查的用户 ID |

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "is_friend": true
  },
  "message": "检查好友关系成功"
}
```

---

### 20. 删除好友

**接口地址：** `POST /api/friend/delete`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `friend_id` | uint | 是 | 好友用户 ID |

---

### 21. 修改好友备注

**接口地址：** `POST /api/friend/remark_update`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `friend_id` | uint | 是 | 好友用户 ID |
| `remark` | string | 否 | 备注名，空字符串表示清除备注 |

---

## 群聊相关 API

### 22. 创建群聊

**接口地址：** `POST /api/group/create`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 群聊名称 |
| `avatar` | string | 否 | 群头像文件名 |
| `member_ids` | []uint | 否 | 初始成员 ID 列表（必须是好友） |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/group/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "项目组",
    "avatar": "",
    "member_ids": [29, 30]
  }'
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "id": 3,
    "name": "项目组",
    "avatar": "",
    "owner_id": 28,
    "member_count": 3,
    "created_at": "2026-04-03T03:10:54+08:00",
    "updated_at": "2026-04-03T03:10:54+08:00"
  },
  "message": "创建群聊成功"
}
```

---

### 23. 获取群聊列表

**接口地址：** `GET /api/group/list`

**认证要求：** 是

**响应示例：**

```json
{
  "code": 200,
  "data": [
    {
      "id": 3,
      "name": "项目组",
      "avatar": "",
      "owner_id": 28,
      "member_count": 3,
      "created_at": "2026-04-03T03:10:54+08:00",
      "updated_at": "2026-04-03T03:10:54+08:00"
    }
  ],
  "message": "获取群聊列表成功"
}
```

---

### 24. 获取群成员列表

**接口地址：** `GET /api/group/members`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |

**响应示例：**

```json
{
  "code": 200,
  "data": [
    {
      "user_id": 28,
      "account": "4692092926",
      "name": "张三",
      "email": "user1@example.com",
      "avatar": "avatar_28_1776183103821.jpg",
      "role": "owner"
    },
    {
      "user_id": 29,
      "account": "9385705211",
      "name": "李四",
      "email": "user2@example.com",
      "avatar": "avatar_29_1776183103821.jpg",
      "role": "member"
    }
  ],
  "message": "获取群成员成功"
}
```

**角色说明：**

| 角色 | 说明 |
|------|------|
| `owner` | 群主 |
| `member` | 普通成员 |

---

### 25. 拉好友进群

**接口地址：** `POST /api/group/member/add`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |
| `member_ids` | []uint | 是 | 要拉入的好友 ID 列表 |

---

### 26. 踢出群成员

**接口地址：** `POST /api/group/member/remove`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |
| `member_id` | uint | 是 | 要踢出的成员 ID |

**说明：** 只有群主可以踢人

---

### 27. 退出群聊

**接口地址：** `POST /api/group/leave`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |

**说明：** 群主不能直接退出，需要先解散群聊

---

### 28. 删除群聊（解散）

**接口地址：** `POST /api/group/delete`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |

**说明：** 只有群主可以解散群聊

---

## WebSocket 实时聊天

### 29. 建立 WebSocket 连接

**连接地址：** `wss://api.gelsomino.cn:444/ws/chat?token=<token>`

**认证方式：** 通过 URL 参数传递 token

**JavaScript 连接示例：**

```javascript
const ws = new WebSocket('wss://api.gelsomino.cn:444/ws/chat?token=your_token');

ws.onopen = () => {
  console.log('WebSocket 连接成功');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('收到消息:', data);
};

ws.onclose = () => {
  console.log('WebSocket 连接关闭');
};

ws.onerror = (error) => {
  console.error('WebSocket 错误:', error);
};
```

**连接成功响应：**

```json
{
  "type": "connected",
  "user_id": 8
}
```

---

### 30. 发送单聊消息

**客户端发送：**

```json
{
  "type": "chat",
  "to_user_id": 9,
  "message_type": "text",
  "content": "你好"
}
```

**发送图片消息：**

```json
{
  "type": "chat",
  "to_user_id": 9,
  "message_type": "image",
  "content": "https://cdn.gelsomino.cn/chat/chat_8_1776183103821_0.jpg"
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `chat` |
| `to_user_id` | uint | 是 | 接收方用户 ID |
| `message_type` | string | 是 | `text` 或 `image` |
| `content` | string | 是 | 消息内容 |

---

### 31. 发送群聊消息

**客户端发送：**

```json
{
  "type": "chat",
  "group_id": 3,
  "message_type": "text",
  "content": "大家好"
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `chat` |
| `group_id` | uint | 是 | 群聊 ID（与 to_user_id 二选一） |
| `message_type` | string | 是 | `text` 或 `image` |
| `content` | string | 是 | 消息内容 |

---

### 32. 消息回执

**发送成功回执（发送方收到）：**

```json
{
  "type": "sent",
  "message": {
    "id": "1775157054611820070-8",
    "conversation_type": "single",
    "from_user_id": 8,
    "to_user_id": 9,
    "group_id": 0,
    "message_type": "text",
    "content": "你好",
    "created_at": "2026-04-03T03:10:54+08:00"
  }
}
```

**消息投递（接收方收到）：**

```json
{
  "type": "chat",
  "message": {
    "id": "1775157054611820070-8",
    "conversation_type": "single",
    "from_user_id": 8,
    "to_user_id": 9,
    "group_id": 0,
    "message_type": "text",
    "content": "你好",
    "created_at": "2026-04-03T03:10:54+08:00"
  },
  "offline": false
}
```

**离线消息：**

```json
{
  "type": "chat",
  "message": {
    "id": "1741170000000-1",
    "conversation_type": "single",
    "from_user_id": 8,
    "to_user_id": 9,
    "group_id": 0,
    "message_type": "text",
    "content": "离线消息",
    "created_at": "2026-03-25T01:23:45Z"
  },
  "offline": true
}
```

**字段说明：**

| 字段 | 说明 |
|------|------|
| `offline` | `true` 表示这是离线消息，`false` 表示实时消息 |

---

### 33. WebSocket 错误消息

```json
{
  "type": "error",
  "error": "只能给好友发送消息"
}
```

**常见错误：**

| 错误信息 | 说明 |
|----------|------|
| `缺少认证信息` | Token 无效或缺失 |
| `无效的token` | Token 格式错误或已过期 |
| `不支持的消息类型` | message_type 不是 text 或 image |
| `接收方或群聊不能为空` | 缺少 to_user_id 或 group_id |
| `消息内容不能为空` | content 为空 |
| `只能给好友发送消息` | 尝试给非好友发送消息 |
| `你不在该群聊中` | 尝试给未加入的群发送消息 |

---

### 34. WebSocket 心跳机制

**后端机制：**
- 服务端每 5 秒发送一次 Ping 帧
- 客户端需在 3 秒内响应 Pong 帧
- 超时未响应将断开连接

**前端建议：**
1. 标准 WebSocket API 会自动响应 Ping/Pong
2. 实现断线重连机制（指数退避：1s → 2s → 4s → 8s）
3. Token 失效时触发刷新流程后再重连

---

## WebSocket 在线状态

### 35. 建立在线状态 WebSocket 连接

**连接地址：** `wss://api.gelsomino.cn:444/ws/online?token=<token>`

**认证方式：** 通过 URL 参数传递 Access Token

**用途：** 前端用于查询指定用户当前是否在线。在线状态基于用户是否存在有效聊天 WebSocket 连接判断。

**JavaScript 连接示例：**

```javascript
const onlineWs = new WebSocket('wss://api.gelsomino.cn:444/ws/online?token=your_token');

onlineWs.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('在线状态消息:', data);
};
```

**连接成功响应：**

```json
{
  "type": "connected",
  "user_id": 8
}
```

---

### 36. 查询用户在线状态

**查询单个用户：**

```json
{
  "type": "check_online",
  "user_id": 9
}
```

**查询多个用户：**

```json
{
  "type": "check_online",
  "user_ids": [9, 10, 11]
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `check_online` |
| `user_id` | uint | 否 | 单个待查询用户 ID |
| `user_ids` | []uint | 否 | 多个待查询用户 ID |

**单个用户响应：**

```json
{
  "type": "online_status",
  "user_id": 9,
  "online": true
}
```

**多个用户响应：**

```json
{
  "type": "online_status",
  "statuses": [
    {
      "user_id": 9,
      "online": true
    },
    {
      "user_id": 10,
      "online": false
    }
  ]
}
```

---

### 37. 在线状态心跳与错误

**客户端心跳：**

```json
{
  "type": "ping"
}
```

**服务端响应：**

```json
{
  "type": "pong"
}
```

**错误响应：**

```json
{
  "type": "error",
  "error": "用户ID不能为空"
}
```

**说明：**
- 服务端每 5 秒发送一次 WebSocket Ping 帧
- 客户端需在 3 秒内响应 Pong 帧
- 查询消息中 `user_id` 和 `user_ids` 至少传一个

---

## E2EE 端到端加密 API

### 概述

系统支持端到端加密（End-to-End Encryption），使用 X25519 + ChaCha20-Poly1305 算法。

### 38. 发布用户公钥

**接口地址：** `POST /api/e2ee/keys/publish`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `key_type` | string | 是 | 密钥类型，如 `x25519` |
| `public_key` | string | 是 | Base64 编码的公钥 |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/e2ee/keys/publish \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "key_type": "x25519",
    "public_key": "base64_encoded_public_key"
  }'
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "user_id": 8,
    "key_type": "x25519",
    "updated_at": "2026-04-03T03:10:54Z"
  },
  "message": "ok"
}
```

---

### 39. 获取用户公钥

**接口地址：** `GET /api/e2ee/keys/public`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | uint | 是 | 目标用户 ID |

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "user_id": 9,
    "key_type": "x25519",
    "public_key": "base64_encoded_public_key",
    "updated_at": "2026-04-03T03:10:54Z"
  },
  "message": "ok"
}
```

---

### 40. 获取群聊当前密钥

**接口地址：** `GET /api/e2ee/group/key/current`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |

**响应示例（成功）：**

```json
{
  "code": 200,
  "data": {
    "group_id": 3,
    "key_version": 1,
    "wrapped_group_key": "base64_encoded_key",
    "wrap_nonce": "base64_encoded_nonce",
    "wrapped_by_user_id": 8,
    "key_wrap_alg": "x25519+aes256gcm"
  },
  "message": "ok"
}
```

**响应示例（需要上传密钥）：**

```json
{
  "code": 428,
  "message": "e2ee group key box not found, please upload key boxes",
  "data": {
    "group_id": 3,
    "key_version": 1,
    "need_publish": true
  }
}
```

---

### 41. 发布群聊密钥盒子

**接口地址：** `POST /api/e2ee/group/key/publish`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |
| `key_version` | int | 是 | 密钥版本 |
| `key_wrap_alg` | string | 否 | 密钥包装算法 |
| `boxes` | array | 是 | 密钥盒子数组 |

**boxes 数组元素：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | uint | 目标用户 ID |
| `wrapped_group_key` | string | 加密的群密钥 |
| `wrap_nonce` | string | 加密 nonce |

**请求示例：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/e2ee/group/key/publish \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": 3,
    "key_version": 1,
    "key_wrap_alg": "x25519+aes256gcm",
    "boxes": [
      {
        "user_id": 9,
        "wrapped_group_key": "base64_encoded_key",
        "wrap_nonce": "base64_encoded_nonce"
      }
    ]
  }'
```

---

### 42. 获取指定版本群密钥

**接口地址：** `GET /api/e2ee/group/key/by-version`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | uint | 是 | 群聊 ID |
| `key_version` | int | 是 | 密钥版本 |

---

## RTC 实时通话 API

### 概述

系统支持音视频实时通话，基于 WebRTC 技术。

### 通话类型

| 类型 | 说明 |
|------|------|
| `audio` | 语音通话 |
| `video` | 视频通话 |

### 43. 发起通话邀请

**接口地址：** `POST /api/rtc/call/invite`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `peer_id` | uint | 可选 | 被叫用户 ID（单聊） |
| `group_id` | uint | 可选 | 群聊 ID（群通话） |
| `call_type` | string | 是 | `audio` 或 `video` |

**请求示例（单聊）：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/rtc/call/invite \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "peer_id": 9,
    "call_type": "video"
  }'
```

**请求示例（群聊）：**

```bash
curl -X POST https://api.gelsomino.cn:444/api/rtc/call/invite \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": 3,
    "call_type": "audio"
  }'
```

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "call_id": "call_1776183103821_8_9",
    "room_id": "room_call_1776183103821_8_9",
    "call_type": "video",
    "peer_id": 9,
    "group_id": 0
  },
  "message": "发起呼叫成功"
}
```

---

### 44. 接受通话

**接口地址：** `POST /api/rtc/call/accept`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `call_id` | string | 是 | 通话 ID |

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "call_id": "call_1776183103821_8_9",
    "room_id": "room_call_1776183103821_8_9"
  },
  "message": "接听成功"
}
```

---

### 45. 拒绝通话

**接口地址：** `POST /api/rtc/call/reject`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `call_id` | string | 是 | 通话 ID |
| `reason` | string | 否 | 拒绝原因 |

---

### 46. 取消通话

**接口地址：** `POST /api/rtc/call/cancel`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `call_id` | string | 是 | 通话 ID |

**说明：** 主叫方在对方接听前取消通话

---

### 47. 挂断通话

**接口地址：** `POST /api/rtc/call/hangup`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `call_id` | string | 是 | 通话 ID |

**说明：** 通话中任意一方挂断

---

### 48. 获取 RTC Token

**接口地址：** `POST /api/rtc/token`

**认证要求：** 是

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `call_id` | string | 是 | 通话 ID |
| `room_id` | string | 否 | 房间 ID |
| `call_type` | string | 是 | `audio` 或 `video` |
| `peer_id` | uint | 否 | 对端用户 ID |
| `group_id` | uint | 否 | 群聊 ID |

**响应示例：**

```json
{
  "code": 200,
  "data": {
    "app_id": "agora_app_id",
    "room_id": "room_123",
    "uid": "8",
    "token": "rtc_token_string"
  },
  "message": "获取 RTC Token 成功"
}
```

---

## 附录

### 附录 A：完整错误码表

#### HTTP 状态码

| 状态码 | 含义 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 401 | 未授权，Token 无效或过期 |
| 403 | 禁止访问，权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 428 | 需要上传密钥（E2EE） |
| 500 | 服务器内部错误 |

#### 业务错误码

| 错误码 | 说明 | 建议处理 |
|--------|------|----------|
| 10001 | 用户已存在 | 提示用户直接登录 |
| 10002 | 用户不存在 | 提示用户先注册 |
| 10003 | 密码错误 | 提示重新输入密码 |
| 10004 | Token 生成失败 | 提示稍后重试 |
| 10005 | Token 解析失败 | 重新登录 |
| 10006 | Token 已过期 | 调用刷新接口或重新登录 |
| 20001 | 好友已存在 | 无需处理 |
| 20002 | 好友不存在 | 提示先添加好友 |
| 20003 | 好友请求处理失败 | 提示稍后重试 |
| 30001 | 文件上传失败 | 提示重新上传 |
| 30002 | 文件下载失败 | 提示稍后重试 |

### 附录 B：数据类型规范

| 字段 | 类型 | 格式/约束 |
|------|------|-----------|
| `gender` | int | 0-未知，1-男，2-女 |
| `birthday` | string | YYYY-MM-DD |
| `message_type` | string | `text`, `image` |
| `conversation_type` | string | `single`, `group` |
| `call_type` | string | `audio`, `video` |
| `created_at` | string | RFC3339 格式 |

### 附录 C：头像处理指南

#### 头像上传流程

```
1. 调用 GET /api/oss/upload-url?key=avatar_{user_id}_{timestamp}.jpg&type=avatar
2. 使用返回的 upload_url 上传图片（PUT 请求）
3. 调用 POST /api/user/avatar_update 更新头像字段
```

#### 头像显示流程

```
1. 检查 avatar 字段是否为完整 URL
   - 是：直接使用
   - 否：调用 GET /api/oss/download-url?key={avatar} 获取下载链接
2. 使用下载链接显示头像
```

### 附录 D：测试流程

#### 完整测试步骤

```bash
# 1. 注册用户
export REGISTER_RESULT=$(curl -s -X POST https://api.gelsomino.cn:444/api/user/register \
  -H "Content-Type: application/json" \
  -d '{"email": "test1@example.com", "password": "password123"}')

# 2. 登录获取 token
export LOGIN_RESULT=$(curl -s -X POST https://api.gelsomino.cn:444/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"account": "test1@example.com", "password": "password123"}')

export TOKEN=$(echo $LOGIN_RESULT | jq -r '.data.token')

# 3. 获取用户信息
curl -X POST https://api.gelsomino.cn:444/api/user/self \
  -H "Authorization: Bearer $TOKEN"

# 4. 搜索用户
curl -X GET "https://api.gelsomino.cn:444/api/user/search?keyword=test2@example.com"

# 5. 添加好友
curl -X POST https://api.gelsomino.cn:444/api/friend/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"account": "test2@example.com"}'

# 6. 获取好友列表
curl -X GET https://api.gelsomino.cn:444/api/friend/list \
  -H "Authorization: Bearer $TOKEN"
```

### 附录 E：WebSocket 消息格式汇总

#### 客户端发送消息

| 类型 | 格式 | 说明 |
|------|------|------|
| 单聊文本 | `{"type":"chat","to_user_id":9,"message_type":"text","content":"内容"}` | 发送文本消息 |
| 单聊图片 | `{"type":"chat","to_user_id":9,"message_type":"image","content":"url"}` | 发送图片消息 |
| 群聊文本 | `{"type":"chat","group_id":3,"message_type":"text","content":"内容"}` | 发送群消息 |
| 群聊图片 | `{"type":"chat","group_id":3,"message_type":"image","content":"url"}` | 发送群图片 |
| 查询单人在线状态 | `{"type":"check_online","user_id":9}` | 通过 `/ws/online` 查询单个用户在线状态 |
| 查询多人在线状态 | `{"type":"check_online","user_ids":[9,10]}` | 通过 `/ws/online` 批量查询在线状态 |
| 在线状态心跳 | `{"type":"ping"}` | 通过 `/ws/online` 发送应用层心跳 |

#### 服务端推送消息

| 类型 | 格式 | 说明 |
|------|------|------|
| 连接成功 | `{"type":"connected","user_id":8}` | WebSocket 连接建立 |
| 发送回执 | `{"type":"sent","message":{}}` | 消息发送成功 |
| 收到消息 | `{"type":"chat","message":{},"offline":false}` | 收到新消息 |
| 离线消息 | `{"type":"chat","message":{},"offline":true}` | 离线期间的消息 |
| 在线状态 | `{"type":"online_status","user_id":9,"online":true}` | 单个用户在线状态 |
| 批量在线状态 | `{"type":"online_status","statuses":[{}]}` | 多个用户在线状态 |
| 心跳响应 | `{"type":"pong"}` | 在线状态 WS 心跳响应 |
| 错误 | `{"type":"error","error":"错误信息"}` | 发生错误 |

---

**文档结束**
