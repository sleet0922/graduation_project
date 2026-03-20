# API 文档

## 1. 添加用户

### 接口信息
- **路径**：`/api/user/add`
- **方法**：`POST`
- **内容类型**：`application/json`

### 接受的参数
| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| name | string | 是 | 用户名 |
| account | string | 是 | 账号 |
| password | string | 是 | 密码 |
| phone | string | 是 | 手机号 |
| avatar | string | 否 | 头像 |
| gander | int | 否 | 性别 |
| birthday | string | 否 | 生日 |
| location | string | 否 | 位置 |
| user_status | int | 否 | 用户状态 |

### 返回的结果
- **成功**：
  ```json
  {
    "code": 200,
    "message": "添加用户成功",
    "data": {
      "ID": 0,
      "CreatedAt": "2026-03-20T08:59:52.116+08:00",
      "UpdatedAt": "2026-03-20T08:59:52.116+08:00",
      "DeletedAt": null,
      "name": "张三",
      "account": "zhangsan",
      "password": "$2a$10$...", // 加密后的密码
      "phone": "13800138000",
      "avatar": "",
      "gander": 1,
      "birthday": "2000-01-01",
      "location": "北京",
      "user_status": 1
    }
  }
  ```

- **失败**：
  ```json
  {
    "code": 400,
    "message": "参数错误",
    "data": null
  }
  ```

  或

  ```json
  {
    "code": 500,
    "message": "添加用户失败",
    "data": null
  }
  ```

### 示例请求
```bash
curl -X POST http://localhost:8081/api/user/add \
  -H "Content-Type: application/json" \
  -d '{
    "name": "张三",
    "account": "zhangsan",
    "password": "123456",
    "phone": "13800138000",
    "avatar": "",
    "gander": 1,
    "birthday": "2000-01-01",
    "location": "北京",
    "user_status": 1
  }'
```
