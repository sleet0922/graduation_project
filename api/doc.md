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

### 11. 上传聊天图片（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/chat/upload/image \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./test.png"
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `file` (必填): 图片文件，支持常见 `image/*` 类型，大小不能超过 10MB

**响应示例:**

```json
{
  "code": 200,
  "data": {
    "url": "https://853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/sleet/chat/32/1775157055_test.png",
    "content": "https://853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/sleet/chat/32/1775157055_test.png",
    "filename": "test.png",
    "contentType": "image/png"
  },
  "message": "上传聊天图片成功"
}
```

说明：

- 返回的 `content` 字段可以直接作为 WebSocket 图片消息的 `content`

---

### 12. 删除用户/注销账号（需要认证）

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

### 13. 发送好友请求（需要认证）

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

### 14. 获取好友请求列表（需要认证）

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

### 15. 处理好友申请（需要认证）

```bash
# 接受好友申请
curl -X POST https://code.gelsomino.cn:8081/api/friend/handle \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "request_id": 1,
    "status": 1
  }'

# 拒绝好友申请
curl -X POST https://code.gelsomino.cn:8081/api/friend/handle \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "request_id": 1,
    "status": 2
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `request_id` (必填): 好友申请记录 ID，来自“获取好友请求列表”接口中的 `ID`
- `status` (必填): 处理结果
  - `1`: 接受申请
  - `2`: 拒绝申请

说明：

- 只有当前登录用户作为 `receiver_id` 时，才可以处理这条好友申请
- 前端应优先从“获取好友请求列表”接口中读取待处理申请，再调用本接口
- 如果一条申请已经被处理过，再次调用不会重复创建好友关系

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "处理好友申请成功"
}
```

**常见错误响应:**

```json
{
  "code": 400,
  "data": null,
  "message": "无效的好友申请处理状态"
}
```

```json
{
  "code": 403,
  "data": null,
  "message": "无权处理该好友申请"
}
```

```json
{
  "code": 404,
  "data": null,
  "message": "好友申请不存在"
}
```

**前端处理建议:**

1. 先调用“获取好友请求列表”接口，筛出 `status = 0` 的待处理申请
2. 用户点击“接受”时，传 `status = 1`
3. 用户点击“拒绝”时，传 `status = 2`
4. 处理成功后，重新拉取好友申请列表和好友列表，刷新页面状态

---

### 16. 获取好友列表（需要认证）

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

### 17. 检查好友关系（需要认证）

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

### 18. 删除好友（需要认证）

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

### 19. 修改好友备注（需要认证）

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

### 20. 创建群聊（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/group/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "项目组",
    "avatar": "",
    "member_ids": [29, 30]
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `name` (必填): 群聊名称
- `avatar` (可选): 群头像地址
- `member_ids` (可选): 初始拉入群聊的好友用户 ID 列表

**响应示例:**

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

### 21. 拉好友进群（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/group/member/add \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": 3,
    "member_ids": [31, 32]
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `group_id` (必填): 群聊 ID
- `member_ids` (必填): 要拉入群聊的好友用户 ID 列表

**响应示例:**

```json
{
  "code": 200,
  "data": [
    {
      "user_id": 28,
      "account": "4692092926",
      "name": "未命名用户",
      "email": "user1@example.com",
      "avatar": "",
      "role": "owner"
    },
    {
      "user_id": 29,
      "account": "9385705211",
      "name": "未命名用户",
      "email": "user2@example.com",
      "avatar": "",
      "role": "member"
    }
  ],
  "message": "拉群成功"
}
```

说明：

- 当前版本仅允许把自己的好友拉进群
- 当前版本拉人进群时不会额外发送系统通知消息，如果前端需要提示，可在成功拉群后自行刷新群成员列表或补充业务通知

---

### 22. 获取群聊列表（需要认证）

```bash
curl -X GET https://code.gelsomino.cn:8081/api/group/list \
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

### 23. 获取群成员列表（需要认证）

```bash
curl -X GET "https://code.gelsomino.cn:8081/api/group/members?group_id=3" \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `group_id` (必填): 群聊 ID

**响应示例:**

```json
{
  "code": 200,
  "data": [
    {
      "user_id": 28,
      "account": "4692092926",
      "name": "未命名用户",
      "email": "user1@example.com",
      "avatar": "",
      "role": "owner"
    },
    {
      "user_id": 29,
      "account": "9385705211",
      "name": "未命名用户",
      "email": "user2@example.com",
      "avatar": "",
      "role": "member"
    }
  ],
  "message": "获取群成员成功"
}
```

---

### 24. 踢出群成员（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/group/member/remove \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": 3,
    "member_id": 29
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `group_id` (必填): 群聊 ID
- `member_id` (必填): 要踢出的群成员用户 ID

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "踢出群成员成功"
}
```

说明：

- 只有群主可以踢人
- 不能踢出群主自己

---

### 25. 退出群聊（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/group/leave \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": 3
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `group_id` (必填): 群聊 ID

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "退出群聊成功"
}
```

说明：

- 普通群成员可以主动退出群聊
- 群主不能直接退出群聊，当前版本需要先解散群聊

---

### 26. 删除群聊（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/group/delete \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": 3
  }'
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `group_id` (必填): 群聊 ID

**响应示例:**

```json
{
  "code": 200,
  "data": null,
  "message": "删除群聊成功"
}
```

说明：

- 只有群主可以删除群聊

---

### 27. 获取云端聊天记录（需要认证）

```bash
# 获取当前用户所有可见的聊天记录
curl -X GET "https://code.gelsomino.cn:8081/api/chat/history" \
  -H "Authorization: Bearer $TOKEN"

# 获取指定好友的聊天记录
curl -X GET "https://code.gelsomino.cn:8081/api/chat/history?friend_id=1" \
  -H "Authorization: Bearer $TOKEN"

# 获取指定群聊的聊天记录
curl -X GET "https://code.gelsomino.cn:8081/api/chat/history?group_id=3" \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (可选): 指定好友的用户 ID
- `group_id` (可选): 指定群聊 ID

说明：

- `friend_id` 与 `group_id` 二选一即可
- 如果都不传，则返回当前用户所有可见的聊天记录（单聊 + 群聊）
- 这里的“可见”指：
  - 单聊：当前用户作为发送方或接收方，且未被自己删除的消息
  - 群聊：当前用户当前所属群聊中的消息，且未被自己删除的消息
- 不会返回当前用户未加入群聊的群消息

**响应示例:**

```json
{
  "code": 200,
  "data": [
    {
      "id": "1738221800123456789-1",
      "conversation_type": "single",
      "from_user_id": 8,
      "to_user_id": 9,
      "group_id": 0,
      "message_type": "text",
      "content": "你好",
      "created_at": "2026-03-31T13:10:20Z",
      "updated_at": "2026-03-31T13:10:20Z"
    },
    {
      "id": "1775157056220740834-13",
      "conversation_type": "group",
      "from_user_id": 32,
      "to_user_id": 0,
      "group_id": 3,
      "message_type": "image",
      "content": "https://853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/sleet/chat/32/1775157055_test.png",
      "created_at": "2026-04-03T03:10:56+08:00",
      "updated_at": "2026-04-03T03:10:56+08:00"
    }
  ],
  "message": "获取成功"
}
```

---

### 28. 删除云端聊天记录（需要认证）

```bash
# 删除当前用户所有可见聊天记录（仅删除自己视角，不影响其他用户）
curl -X DELETE "https://code.gelsomino.cn:8081/api/chat/history" \
  -H "Authorization: Bearer $TOKEN"

# 删除与指定好友的聊天记录（仅删除自己视角，不影响对方）
curl -X DELETE "https://code.gelsomino.cn:8081/api/chat/history?friend_id=1" \
  -H "Authorization: Bearer $TOKEN"

# 删除指定群聊的聊天记录（仅删除自己视角，不影响其他群成员）
curl -X DELETE "https://code.gelsomino.cn:8081/api/chat/history?group_id=3" \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**

- `Authorization`: Bearer Token

**请求参数:**

- `friend_id` (可选): 指定好友的用户 ID
- `group_id` (可选): 指定群聊 ID

说明：

- `friend_id` 与 `group_id` 二选一即可
- 不传时删除当前用户所有可见聊天记录
- 删除范围只包含：
  - 当前用户自己的单聊记录视角
  - 当前用户当前所属群聊中的群消息视角
- 不会影响其他用户，也不会删除当前用户未加入群聊的消息

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

### 29. 建立聊天连接

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

### 30. 发送聊天消息

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

客户端发送群文本消息：

```json
{
  "type": "chat",
  "group_id": 3,
  "message_type": "text",
  "content": "大家好"
}
```

客户端发送群图片消息：

```json
{
  "type": "chat",
  "group_id": 3,
  "message_type": "image",
  "content": "https://853c9e9e83f1baa03bf8f17686060e5c.r2.cloudflarestorage.com/sleet/sleet/chat/32/1775157055_test.png"
}
```

说明：

- 单聊时传 `to_user_id`
- 群聊时传 `group_id`
- 图片消息建议先调用“上传聊天图片”接口，再把返回的 `content` 作为消息内容发送

发送成功后，发送方会收到：

```json
{
  "type": "sent",
  "message": {
    "id": "1775157054611820070-10",
    "conversation_type": "group",
    "from_user_id": 8,
    "to_user_id": 0,
    "group_id": 3,
    "message_type": "text",
    "content": "大家好",
    "created_at": "2026-04-03T03:10:54+08:00"
  }
}
```

接收方会收到：

```json
{
  "type": "chat",
  "message": {
    "id": "1775157054611820070-10",
    "conversation_type": "group",
    "from_user_id": 8,
    "to_user_id": 0,
    "group_id": 3,
    "message_type": "text",
    "content": "大家好",
    "created_at": "2026-04-03T03:10:54+08:00"
  },
  "offline": false
}
```

### 31. 离线消息

- 如果接收方不在线，服务端会把消息暂存在内存里
- 接收方下次建立 WebSocket 连接后，服务端会立即投递这些消息
- 服务端成功投递后，就会从内存中删除对应离线消息
- 这些消息会写入数据库，支持后续查询云端聊天记录

离线消息投递时格式如下：

```json
{
  "type": "chat",
  "message": {
    "id": "1741170000000-1",
    "conversation_type": "group",
    "from_user_id": 8,
    "to_user_id": 0,
    "group_id": 3,
    "content": "我离线给你发了一条消息",
    "created_at": "2026-03-25T01:23:45Z"
  },
  "offline": true
}
```

### 32. 错误消息

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
- `接收方或群聊不能为空`
- `消息内容不能为空`
- `只能给好友发送消息`
- `你不在该群聊中`

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

# 上传聊天图片
curl -X POST https://code.gelsomino.cn:8081/api/chat/upload/image \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@./test.png"

# 测试好友 API（通过邮箱或账号）
curl -X POST https://code.gelsomino.cn:8081/api/friend/request -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"account": "sleet0528@outlook"}'

# 获取好友列表（返回好友详细信息）
curl -X GET https://code.gelsomino.cn:8081/api/friend/list -H "Authorization: Bearer $TOKEN"

# 创建群聊
curl -X POST https://code.gelsomino.cn:8081/api/group/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"项目组","member_ids":[2,3]}'
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

3. **目标用户获取待处理申请列表**（需要认证）

```bash
curl -X GET https://code.gelsomino.cn:8081/api/friend/requests -H "Authorization: Bearer $TOKEN"
```

从返回结果中拿到申请记录的 `ID`

4. **目标用户处理好友申请**（需要认证）

```bash
curl -X POST https://code.gelsomino.cn:8081/api/friend/handle -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"request_id":1,"status":1}'
```

处理成功后，双方即可在好友列表中看到彼此

### 注意事项：

- 确保服务运行在 `code.gelsomino.cn:8081`
- 注册时账号由系统自动生成 10 位随机数字
- 邮箱用于登录，账号用于搜索好友
- 搜索用户接口不需要认证，可以公开调用
- 加好友时如果目标用户是自己，会返回 `不能添加自己为好友`
- 加好友时如果双方已经是好友，会返回 `你们已经是好友了`
- 加好友时如果存在未处理的申请（任一方向），会返回 `好友申请已存在`
- 处理好友申请时，只有申请接收者本人可以操作；其他用户会收到 `无权处理该好友申请`
- 需要先注册并登录获取 token 才能测试需要认证的 API
- 好友功能需要至少注册两个用户进行测试
- 好友列表接口返回好友的详细信息（名字、账号、邮箱、头像等）
- 获取全部聊天记录 / 删除全部聊天记录时，只会处理当前用户自己可见的单聊和所在群聊消息
- 群聊相关接口需要先建立好友关系，当前版本只支持拉好友进群
- 删除群聊仅允许群主操作

---

## 附录

### 附录 A: 全局错误码对照表
前端需要根据响应的 `code` 字段进行全局拦截或提示。

| HTTP 状态码 | 业务错误码 (code) | 说明 | 前端建议处理方式 |
| :--- | :--- | :--- | :--- |
| 200 | 200 | 成功 | 继续执行业务逻辑 |
| 400 | 400 | 请求参数错误 | 提示用户检查输入表单 |
| 401 | 401 | 未授权 | 跳转到登录页 |
| 403 | 403 | 禁止访问 | 提示无权限 |
| 404 | 404 | 资源不存在 | 提示资源找不到 |
| 500 | 500 | 服务器内部错误 | 提示系统繁忙，稍后重试 |
| 200/400 | 10001 | 用户已存在 | 注册时提示账号已占用 |
| 200/404 | 10002 | 用户不存在 | 登录/搜索时提示无此用户 |
| 200/400 | 10003 | 密码错误 | 登录时提示密码错误 |
| 200/500 | 10004 | Token生成失败 | 提示登录失败，请重试 |
| 200/401 | 10005 | Token解析失败 | 强制登出，重新登录 |
| 200/401 | 10006 | Token已过期 | 尝试无感刷新 Token 或重新登录 |
| 200/400 | 20001 | 好友已存在 | 提示已经是好友 |
| 200/404 | 20002 | 好友不存在 | 提示非好友关系 |
| 200/400 | 20003 | 好友请求处理失败 | 提示操作失败，请重试 |
| 200/500 | 30001 | 文件上传失败 | 提示上传失败，请重试 |
| 200/500 | 30002 | 文件下载失败 | 提示下载失败，请重试 |

### 附录 B: 通用字段类型及约束规范
表单提交和数据渲染时，请遵守以下字段规范：

| 字段名 | 类型 | 说明及约束 |
| :--- | :--- | :--- |
| `gender` | `int` | 性别枚举：`0` 未知/保密，`1` 男，`2` 女 |
| `birthday` | `string` | 格式必须为 `YYYY-MM-DD`（如：`2000-01-01`） |
| `password` | `string` | 注册/修改时，建议前端增加长度和复杂度限制（如 6-20 位，包含字母数字） |
| `message_type` | `string` | 聊天消息类型枚举：`text` (文本), `image` (图片) |
| `conversation_type` | `string` | 会话类型枚举：`single` (单聊), `group` (群聊) |

### 附录 C: WebSocket 心跳保活机制 💓
为了防止 WebSocket 长连接被网关（如 Nginx、云服务商防火墙）自动掐断，**前端必须实现心跳机制**。

**后端机制：**
- 服务端会每隔 **5 秒** 向客户端发送一次 Ping 帧。
- 客户端如果在 **3 秒** 内没有响应 Pong 帧，服务端将主动断开连接。

**前端开发建议（心跳与重连）：**
1. **自动响应**：标准的浏览器 WebSocket API (如 JS 的 `WebSocket` 对象) 底层会自动响应服务端的 Ping 帧（回复 Pong 帧），通常不需要前端手动写代码回 Ping。
2. **断线重连**：网络波动会导致连接意外断开，前端必须监听 `onclose` 和 `onerror` 事件，并实现**指数退避重连机制**（如 1s -> 2s -> 4s -> 8s 尝试重连）。
3. **Token 失效处理**：如果连接因为 `401 Unauthorized` 被拒绝，应触发 Token 刷新流程后再重新建立 WS 连接。
