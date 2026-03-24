# API 测试文档

## 用户相关 API

### 1. 用户注册

```bash
curl -X POST http://localhost:8081/api/user/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "123456"
  }'
```

**请求参数:**
- `email` (必填): 邮箱地址
- `password` (必填): 密码

**响应示例:**
```json
{
  "code": 200,
  "data": {
    "id": 6,
    "account": "6158726193",
    "name": "test@example.com",
    "email": "test@example.com"
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
    "account": "6158726193",
    "password": "123456"
  }'
```

**请求参数:**
- `account` (必填): 账号（10位随机数字）
- `password` (必填): 密码

**响应示例:**
```json
{
  "code": 200,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 6,
      "account": "6158726193",
      "name": "test@example.com",
      "avatar": "",
      "email": "test@example.com",
      "gender": 0,
      "birthday": "",
      "location": ""
    }
  },
  "message": "登录成功"
}
```

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

### 4. 更新用户名（需要认证）

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

### 8. 发送好友请求（需要认证）

```bash
curl -X POST http://localhost:8081/api/friend/request \
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
  "message": "好友申请已发送"
}
```

---

### 9. 获取好友请求列表（需要认证）

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

### 10. 获取好友列表（需要认证）

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
  "data": [],
  "message": "获取好友列表成功"
}
```

---

### 11. 检查好友关系（需要认证）

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
}
```

---

## 测试流程

### 快速测试步骤：

1. **注册用户**
```bash
curl -X POST http://localhost:8081/api/user/register -H "Content-Type: application/json" -d '{"email": "test@example.com", "password": "123456"}'
```

2. **登录获取 token**
```bash
curl -X POST http://localhost:8081/api/user/login -H "Content-Type: application/json" -d '{"account": "6158726193", "password": "123456"}'
```

3. **设置 token 变量**
```bash
export TOKEN="你的token"
```

4. **测试其他 API**
```bash
# 获取用户信息
curl -X POST http://localhost:8081/api/user/self -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN"

# 测试 OSS API
curl -X GET "http://localhost:8081/api/oss/upload-url?key=test.jpg"

# 测试好友 API
curl -X POST http://localhost:8081/api/friend/request -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d '{"friend_id": 7}'
```

### 注意事项：
- 确保服务运行在 `localhost:8081`
- 注册时账号由系统自动生成 10 位随机数字
- 需要先注册并登录获取 token 才能测试需要认证的 API
- 好友功能需要至少注册两个用户进行测试