# API 测试文档

## 用户相关 API

### 1. 用户注册

```bash
curl -X POST http://localhost:8081/api/user/register \
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
curl -X POST http://localhost:8081/api/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "account": "sleet0528@outlook",
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
    "user": {
      "id": 8,
      "account": "0762353747",
      "name": "未命名用户",
      "avatar": "",
      "email": "sleet0528@outlook",
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

**保存 token 用于后续测试:**
```bash
export TOKEN="你的token"
```

---

### 3. 获取用户信息（需要认证）

```bash
curl -X POST http://localhost:8081/api/user/self \
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

### 4. 搜索用户（可选认证）

```bash
curl -X GET "http://localhost:8081/api/user/search?keyword=sleet0528@outlook"
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

### 5. 更新用户名（需要认证）

```bash
curl -X POST http://localhost:8081/api/user/name_update \
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

### 5. 更新密码（需要认证）

```bash
curl -X POST http://localhost:8081/api/user/password_update \
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

## OSS 相关 API

### 6. 获取上传 URL

```bash
curl -X GET "http://localhost:8081/api/oss/upload-url?key=test.jpg"
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

### 7. 获取下载 URL

```bash
curl -X GET "http://localhost:8081/api/oss/download-url?key=test.jpg"
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

## 好友相关 API

### 9. 发送好友请求（需要认证）

```bash
curl -X POST http://localhost:8081/api/friend/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \  -d '{
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

**使用流程:**
1. 前端先调用搜索用户接口，获取用户详细信息
2. 前端展示用户信息（名字、邮箱、账号、头像等）
3. 用户点击"发送好友请求"按钮
4. 前端调用发送好友请求接口，携带目标用户的邮箱或账号

---

### 10. 获取好友请求列表（需要认证）

```bash
curl -X GET http://localhost:8081/api/friend/requests \
  -H "Authorization: Bearer $TOKEN"
```

**请求头:**
- `Authorization`: Bearer Token

**响应示例:**
```json
{
  "code": 200,
  "data": [],
  "message": "获取好友申请列表成功"
}
```

---

### 11. 获取好友列表（需要认证）

```bash
curl -X GET http://localhost:8081/api/friend/list \
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
curl -X POST http://localhost:8081/api/friend/check \
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
}Q
```

---

## 测试流程

### 快速测试步骤：

1. **注册用户**
```bash
curl -X POST http://localhost:8081/api/user/register -H "Content-Type: application/json" -d '{"email": "sleet0528@outlook.com", "password": "Zyz20050922!"}'
```

2. **登录获取 token**
```bash
curl -X POST http://localhost:8081/api/user/login -H "Content-Type: application/json" -d '{"account": "sleet0528@outlook.com", "password": "Zyz20050922!"}'
```

3. **设置 token 变量**
```bash
export TOKEN="你的token"
```

4. **测试其他 API**
```bash
# 获取用户信息
curl -X POST http://localhost:8081/api/user/self -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN"

# 搜索用户（通过邮箱或账号，不需要认证）
curl -X GET "http://localhost:8081/api/user/search?keyword=sleet0528@outlook"

# 测试 OSS API
curl -X GET "http://localhost:8081/api/oss/upload-url?key=test.jpg"

# 测试好友 API（通过邮箱或账号）
curl -X POST http://localhost:8081/api/friend/request -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"account": "sleet0528@outlook"}'

# 获取好友列表（返回好友详细信息）
curl -X GET http://localhost:8081/api/friend/list -H "Authorization: Bearer $TOKEN"
```

### 完整的好友添加流程：

1. **搜索用户**（不需要认证）
```bash
curl -X GET "http://localhost:8081/api/user/search?keyword=目标用户邮箱"
```
返回目标用户的详细信息（名字、邮箱、账号、头像、生日等）

2. **发送好友请求**（需要认证）
```bash
curl -X POST http://localhost:8081/api/friend/request -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"account":"目标用户邮箱"}'
```

### 注意事项：
- 确保服务运行在 `localhost:8081`
- 注册时账号由系统自动生成 10 位随机数字
- 邮箱用于登录，账号用于搜索好友
- 搜索用户接口不需要认证，可以公开调用
- 需要先注册并登录获取 token 才能测试需要认证的 API
- 好友功能需要至少注册两个用户进行测试
- 好友列表接口返回好友的详细信息（名字、账号、邮箱、头像等）