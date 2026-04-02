## 用户相关 API

### 1. 用户注册

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "sleet0528@outlook.com",
    "password": "Zyz20050922!"
  }'
```

**请求参数:**

- `email` (必填): 邮箱地址（用于登录）
- `password` (必填): 密码

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "id": 8,
    "account": "0762353747",
    "email": "sleet0528@outlook.com",
    "name": "未命名用户"
  },
  "message": "注册成功"
}
```

---

### 2. 用户登录

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "account": "sleet0528@outlook.com",
    "password": "Zyz20050922!"
  }'
```

**请求参数:**

- `account` (必填): 账号（支持邮箱或10位随机数字账号）
- `password` (必填): 密码

**响应示例:**

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
      "email": "sleet0528@outlook.com",
      "gender": 0,
      "birthday": "",
      "location": ""
    }
  },
  "message": "登录成功"
}
```

**说明:**

- 用户注册时，系统会自动生成一个10位随机数字账号作为唯一标识
- 登录时可以使用注册时的邮箱或生成的账号
- 账号主要用于好友搜索，邮箱主要用于登录
- `token` 为 access token，默认有效期 1 天
- `refresh_token` 用于续期 access token，默认有效期 30 天

**保存 token 用于后续测试:**

```bash
export TOKEN="你的token"
export REFRESH_TOKEN="你的refresh_token"
```

---

### 3. 刷新 token

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "'"$REFRESH_TOKEN"'"
  }'
```

**请求参数:**

- `refresh_token` (必填): 登录时返回的 refresh token

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "token": "新的access token",
    "refresh_token": "新的refresh token",
    "expires_in": 86400,
    "refresh_expires_in": 2592000
  },
  "message": "刷新token成功"
}
```

**说明:**

- 当 access token 过期时，客户端应调用该接口换取新 token
- 刷新成功后，客户端需要同时更新本地保存的 `token` 和 `refresh_token`
- 如果 refresh token 也过期了，才需要重新登录

```
---

### 4. 获取用户信息（需要认证）

**注意**: 如果服务器配置为 `mode: "debug"`，请将 `https://` 改为 `http://`

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/self \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "ID": 6,
    "name": "test@example.com",
    "account": "6158726193",
    "email": "test@example.com",
    "avatar": "",
    "gender": 0,
    "birthday": "",
    "location": "",
    "user_status": 0
  },
  "message": "获取用户信息成功"
}
```

---

### 5. 搜索用户（可选认证）

```bash
curl -X GET "https://code.gelsomino.cn:8081/api/user/search?keyword=sleet0528@outlook"
```

**请求参数:**

- `keyword` (必填): 要搜索的账号（10位数字）或邮箱地址

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "id": 8,
    "account": "0762353747",
    "name": "未命名用户",
    "avatar": "",
    "email": "sleet0528@outlook",
    "gender": 0,
    "birthday": "",
    "location": ""
  },
  "message": "搜索用户成功"
}
```

**返回字段说明:**

- `id`: 用户ID
- `account`: 用户账号
- `name`: 用户名字
- `avatar`: 用户头像
- `email`: 用户邮箱
- `gender`: 用户性别 (0:未知, 1:男, 2:女)
- `birthday`: 用户生日
- `location`: 用户位置

**使用流程:**

1. 前端调用搜索用户接口，输入邮箱或账号
2. 后端返回用户详细信息（名字、邮箱、账号、头像、生日等）
3. 前端展示用户信息，用户点击"发送好友请求"按钮
4. 前端调用发送好友请求接口，携带目标用户的邮箱或账号

---

### 6. 更新用户名（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/name_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "张三"
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `name` (必填): 新用户名

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "id": 6,
    "name": "张三"
  },
  "message": "更新用户名成功"
}
```

---

### 7. 更新密码（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/password_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "password": "123456",
    "new_password": "newpassword123"
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `password` (必填): 原密码
- `new_password` (必填): 新密码

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "更新密码成功"
}
```

---

### 8. 更新用户资料（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/profile_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "gender": 1,
    "birthday": "2000-01-01",
    "location": "北京"
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `gender` (可选): 性别 (0:未知, 1:男, 2:女)
- `birthday` (可选): 生日字符串，如 "2000-01-01"
- `location` (可选): 地区字符串

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "id": 6,
    "gender": 1,
    "birthday": "2000-01-01",
    "location": "北京"
  },
  "message": "更新资料成功"
}
```

---

## OSS 相关 API

### 9. 获取上传 URL

```bash
curl -X GET "https://code.gelsomino.cn:8081/api/oss/upload-url?key=test.jpg"
```

**请求参数:**

- `key`: 上传的文件键

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "expires_in": "1小时",
    "upload_url": "https://sleet.853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/test.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&..."
  },
  "message": "获取上传URL成功"
}
```

---

### 10. 获取下载 URL

```bash
curl -X GET "https://code.gelsomino.cn:8081/api/oss/download-url?key=test.jpg"
```

**请求参数:**

- `key`: 下载的文件键

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "download_url": "https://sleet.853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/test.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&...",
    "expires_in": "1小时"
  },
  "message": "获取下载URL成功"
}
```

---

### 11. 删除用户/注销账号（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/delete \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "删除用户成功"
}
```

---

## 好友相关 API

### 12. 发送好友请求（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/friend/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "account": "sleet0528@outlook"
  }'
```

（或者使用 `"friend_id": 8`）

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `account` (可选): 好友的用户账号或邮箱地址
- `friend_id` (可选): 好友用户 ID，与 `account` 选填其一即可

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "好友申请已发送"
}
```

**边界场景响应:**

```json
{
  "code": 400,
  "data": null,
  "message": "不能添加自己为好友"
}
```

```json
{
  "code": 400,
  "data": null,
  "message": "你们已经是好友了"
}
```

```json
{
  "code": 400,
  "data": null,
  "message": "好友申请已存在"
}
```

**使用流程:**

1. 前端先调用搜索用户接口，获取用户详细信息
2. 前端展示用户信息（名字、邮箱、账号、头像等）
3. 用户点击"发送好友请求"按钮
4. 前端调用发送好友请求接口，携带目标用户的邮箱或账号

---

### 13. 获取好友请求列表（需要认证）

```bash
curl -X GET https://code.gelsomino.cn:8081/api/friend/requests \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**响应示例:**

```json
{
  "code": 200,
  "data": [
    {
      "ID": 1,
      "CreatedAt": "2026-03-25T01:23:45Z",
      "UpdatedAt": "2026-03-25T01:23:45Z",
      "DeletedAt": null,
      "sender_id": 8,
      "receiver_id": 9,
      "status": 0
    }
  ],
  "message": "获取好友申请列表成功"
}
```

**返回字段说明:**

- `ID`: 申请记录ID
- `sender_id`: 发起申请的用户ID
- `receiver_id`: 接收申请的用户ID
- `status`: 申请状态 (0: 待处理, 1: 已接受, 2: 已拒绝)

---

### 11. 获取好友列表（需要认证）

```bash
curl -X GET https://code.gelsomino.cn:8081/api/friend/list \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**响应示例:**

```json
{
  "code": 200,
  "data": [
    {
      "id": 1,
      "user_id": 8,
      "friend_id": 9,
      "account": "9395046534",
      "name": "未命名用户",
      "email": "friend@example.com",
      "avatar": "",
      "gender": 0,
      "birthday": "",
      "location": "",
      "remark": ""
    }
  ],
  "message": "获取好友列表成功"
}
```

**返回字段说明:**

- `id`: 好友关系记录ID
- `user_id`: 当前用户ID
- `friend_id`: 好友用户ID
- `account`: 好友账号
- `name`: 好友名字
- `email`: 好友邮箱
- `avatar`: 好友头像
- `gender`: 好友性别 (0:未知, 1:男, 2:女)
- `birthday`: 好友生日
- `location`: 好友位置
- `remark`: 好友备注

---

### 12. 检查好友关系（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/friend/check \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "friend_id": 7
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (必填): 好友用户 ID

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "is_friend": false
  },
  "message": "检查好友关系成功"
}
```

---

### 13. 删除好友（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/friend/delete \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "friend_id": 7
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (必填): 好友用户 ID

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "删除好友成功"
}
```

---

### 14. 修改好友备注（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/friend/remark_update \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "friend_id": 9,
    "remark": "张三"
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (必填): 好友用户 ID
- `remark` (可选): 新的备注名，传空字符串表示清除备注

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "修改好友备注成功"
}
```

---

## 聊天相关 API

### 15. 获取云端聊天记录（需要认证）

```bash
# 获取所有好友的聊天记录
curl -X GET "https://code.gelsomino.cn:8081/api/chat/history" \
  -H "Authorization: Bearer $TOKEN"

# 获取指定好友的聊天记录
curl -X GET "https://code.gelsomino.cn:8081/api/chat/history?friend_id=1" \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (可选): 指定好友的用户 ID。如果不传则返回当前用户与所有好友的聊天记录。

**响应示例:**

```json
{
  "code": 200,
  "data": [
    {
      "id": "1738221800123456789-1",
      "from_user_id": 8,
      "to_user_id": 9,
      "message_type": "text",
      "content": "你好",
      "created_at": "2026-03-31T13:10:20Z",
      "updated_at": "2026-03-31T13:10:20Z"
    }
  ],
  "message": "获取成功"
}
```

---

### 16. 删除云端聊天记录（需要认证）

```bash
# 删除所有好友的聊天记录（仅删除自己视角，不影响对方）
curl -X DELETE "https://code.gelsomino.cn:8081/api/chat/history" \
  -H "Authorization: Bearer $TOKEN"

# 删除与指定好友的聊天记录（仅删除自己视角，不影响对方）
curl -X DELETE "https://code.gelsomino.cn:8081/api/chat/history?friend_id=1" \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (可选): 指定好友的用户 ID。如果不传则删除当前用户与所有好友的聊天记录。

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "删除成功"
}
```

---

## WebSocket 聊天

### 17. 建立聊天连接

**注意**: WebSocket 连接使用 `ws://` 或 `wss://` 协议

```javascript
const ws = new WebSocket("wss://code.gelsomino.cn:8081/ws/chat?token=你的token")
```

连接成功后，服务端会先返回：

```json
{
  "type": "connected",
  "user_id": 8
}
```

### 15. 发送聊天消息

客户端发送文本消息：

```json
{
  "type": "chat",
  "to_user_id": 9,
  "message_type": "text",
  "content": "你好"
}
```

客户端发送图片消息：

```json
{
  "type": "chat",
  "to_user_id": 9,
  "message_type": "image",
  "content": "https://sleet.853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/test.jpg"
}
```

发送成功后，发送方会收到：

```json
{
  "type": "sent",
  "message": {
    "id": "1741170000000-1",
    "from_user_id": 8,
    "to_user_id": 9,
    "message_type": "text",
    "content": "你好",
    "created_at": "2026-03-25T01:23:45Z"
  }
}
```

接收方会收到：

```json
{
  "type": "chat",
  "message": {
    "id": "1741170000000-1",
    "from_user_id": 8,
    "to_user_id": 9,
    "message_type": "text",
    "content": "你好",
    "created_at": "2026-03-25T01:23:45Z"
  },
  "offline": false
}
```

### 16. 离线消息

- 如果接收方不在线，服务端会把消息暂存在内存里
- 接收方下次建立 WebSocket 连接后，服务端会立即投递这些消息
- 服务端成功投递后，就会从内存中删除对应离线消息
- 这些消息不会写入数据库，也不会做云端同步

离线消息投递时格式如下：

```json
{
  "type": "chat",
  "message": {
    "id": "1741170000000-1",
    "from_user_id": 8,
    "to_user_id": 9,
    "content": "我离线给你发了一条消息",
    "created_at": "2026-03-25T01:23:45Z"
  },
  "offline": true
}
```

### 17. 错误消息

```json
{
  "type": "error",
  "error": "只能给好友发送消息"
}
```

常见错误：

- `缺少认证信息`
- `无效的token`
- `不支持的消息类型`
- `接收方不能为空`
- `消息内容不能为空`
- `只能给好友发送消息`

---

## 测试流程

### 快速测试步骤：

1. **注册用户**

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/register -H "Content-Type: application/json" -d '{"email": "sleet0528@outlook.com", "password": "Zyz20050922!"}'
```

2. **登录获取 token**

```bash
curl -X POST https://code.gelsomino.cn:8081/api/user/login -H "Content-Type: application/json" -d '{"account": "sleet0528@outlook.com", "password": "Zyz20050922!"}'
```

3. **设置 token 变量**

```bash
export TOKEN="你的token"
```

4. **测试其他 API**

```bash
# 获取用户信息
curl -X POST https://code.gelsomino.cn:8081/api/user/self -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN"

# 搜索用户（通过邮箱或账号，不需要认证）
curl -X GET "https://code.gelsomino.cn:8081/api/user/search?keyword=sleet0528@outlook"

# 测试 OSS API
curl -X GET "https://code.gelsomino.cn:8081/api/oss/upload-url?key=test.jpg"

# 测试好友 API（通过邮箱或账号）
curl -X POST https://code.gelsomino.cn:8081/api/friend/request -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"account": "sleet0528@outlook"}'

# 获取好友列表（返回好友详细信息）
curl -X GET https://code.gelsomino.cn:8081/api/friend/list -H "Authorization: Bearer $TOKEN"
```

### 完整的好友添加流程：

1. **搜索用户**（不需要认证）

```bash
curl -X GET "https://code.gelsomino.cn:8081/api/user/search?keyword=目标用户邮箱"
```

返回目标用户的详细信息（名字、邮箱、账号、头像、生日等）

2. **发送好友请求**（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/friend/request -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"account":"目标用户邮箱"}'
```

### 注意事项：

- 确保服务运行在 `code.gelsomino.cn:8081`
- 注册时账号由系统自动生成 10 位随机数字
- 邮箱用于登录，账号用于搜索好友
- 搜索用户接口不需要认证，可以公开调用
- 加好友时如果目标用户是自己，会返回 `不能添加自己为好友`
- 加好友时如果双方已经是好友，会返回 `你们已经是好友了`
- 加好友时如果存在未处理的申请（任一方向），会返回 `好友申请已存在`
- 需要先注册并登录获取 token 才能测试需要认证的 API
- 好友功能需要至少注册两个用户进行测试
- 好友列表接口返回好友的详细信息（名字、账号、邮箱、头像等）