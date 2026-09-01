# 毕业项目全栈代码问题报告

**生成时间**: 2026-09-01  
**检查范围**: 
- 后端 Go 代码 (~8000行)
- 前端 Flutter/Dart 代码 (~23823行)

---

## 📋 问题概览

| 优先级 | 后端问题数 | 前端问题数 | 总计 |
|--------|-----------|-----------|------|
| P0 严重 | 3 | 2 | 5 |
| P1 重要 | 6 | 4 | 10 |
| P2 一般 | 12 | 8 | 20 |
| P3 优化 | 9 | 6 | 15 |
| **总计** | **30** | **20** | **50** |

---

## 🔴 P0 严重问题 (必须立即修复)

### 后端问题

#### P0-1: WebSocket连接泄漏
**文件**: `internal/handler/websocket_handler.go:45-120`  
**问题**: 
- `defer conn.Close()` 在多个错误路径缺失
- Redis订阅未正确清理
- 可能导致文件描述符耗尽

```go
// 当前代码 - 缺少清理
if err := h.redis.Subscribe(ctx, userChannel); err != nil {
    return // ❌ 连接和订阅泄漏
}
```

**影响**: 高并发下服务器崩溃  
**修复**: 添加完整的资源清理逻辑

---

#### P0-2: JWT token并发安全问题
**文件**: `internal/middleware/auth_middleware.go:25-50`  
**问题**:
- `claims` 存入 `gin.Context` 但可能被并发读写
- 无互斥锁保护

**影响**: 数据竞争，可能导致认证错误  
**修复**: 使用 `sync.RWMutex` 或改为请求级别存储

---

#### P0-3: 数据库连接池未正确配置
**文件**: `internal/db/postgres.go:15-30`  
**问题**:
```go
sqlDB.SetMaxOpenConns(100)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(time.Hour)
```
- `MaxIdleConns` 过低，频繁创建销毁连接
- `ConnMaxLifetime` 1小时过长，可能遇到数据库端超时

**影响**: 连接池耗尽，请求超时  
**建议**: `MaxIdleConns: 25`, `ConnMaxLifetime: 10分钟`

---

### 前端问题

#### P0-4: 内存泄漏 - Timer未清理
**文件**: `lib/providers/chat_provider.dart:33-43`  
**问题**:
```dart
final Map<String, Timer> _decryptRetryTimers = {};
final Map<String, int> _decryptRetryAttempts = {};
```
- Timer在dispose时清理，但如果Provider异常释放可能泄漏
- `_decryptRetryTimers` 可能无限增长

**影响**: 应用卡顿，内存溢出  
**修复**: 添加Timer上限和老化机制

---

#### P0-5: StreamController未正确关闭
**文件**: `lib/services/websocket_service.dart:54-59`  
**问题**:
```dart
final StreamController<Message> _messageController =
    StreamController<Message>.broadcast();
```
- 在某些异常路径下可能未关闭
- `dispose()` 是 Future 但调用方未 await

**影响**: 资源泄漏，事件丢失  
**修复**: 确保所有路径都await dispose

---

## 🟠 P1 重要问题

### 后端问题

#### P1-1: SQL注入风险
**文件**: `internal/repo/user_repo.go:55`  
**问题**:
```go
query := fmt.Sprintf("SELECT * FROM users WHERE account LIKE '%%%s%%'", keyword)
```
- 使用字符串拼接构建SQL
- 即使后续使用了预编译，格式化字符串仍存在风险

**修复**: 始终使用参数化查询
```go
query := "SELECT * FROM users WHERE account LIKE ?"
db.Raw(query, "%"+keyword+"%")
```

---

#### P1-2: 密码哈希算法不够强
**文件**: `pkg/utils/crypto.go:10-20`  
**问题**:
```go
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
```
- `DefaultCost` 是 10，现代标准推荐 12-14
- 无盐值管理

**修复**: 使用 `bcrypt.Cost = 12` 或更高

---

#### P1-3: 错误处理信息泄露
**文件**: 多处 handler  
**问题**:
```go
c.JSON(500, gin.H{"error": err.Error()})
```
- 直接返回内部错误消息给客户端
- 可能暴露数据库结构、文件路径

**修复**: 记录详细错误到日志，返回通用消息给客户端

---

#### P1-4: LiveKit配置硬编码
**文件**: `configs/livekit.yaml`  
**问题**:
- API密钥直接写在配置文件
- 无环境变量覆盖机制
- 提交到Git会泄露密钥

**修复**: 使用环境变量或密钥管理服务

---

#### P1-5: Redis操作无超时
**文件**: `internal/service/chat_service.go:多处`  
**问题**:
```go
h.redis.Publish(ctx, channel, message)
```
- 使用 `context.Background()` 无超时
- Redis阻塞会导致goroutine泄漏

**修复**: 所有Redis操作使用带超时的context

---

#### P1-6: 文件上传无大小限制
**文件**: `internal/handler/oss_handler.go:25-40`  
**问题**:
- 无文件大小验证
- 无文件类型白名单
- 可能被滥用上传大文件

**修复**: 添加大小限制（如50MB）和类型检查

---

### 前端问题

#### P1-7: 未处理的Future
**文件**: 多个文件  
**问题**:
```dart
unawaited(VideoThumbnailService.generateAndCacheThumbnail(displayMsg.content));
```
- 使用 `unawaited` 但未捕获错误
- Future失败时无任何提示

**修复**: 添加 `.catchError()` 或使用 `try-catch`

---

#### P1-8: 状态更新过于频繁
**文件**: `lib/providers/chat_provider.dart`  
**问题**:
- `notifyListeners()` 被调用 50+ 次
- 每次消息到达都触发全局重建
- UI可能卡顿

**修复**: 使用 `ChangeNotifierProvider.value` 或分片Provider

---

#### P1-9: 图片未压缩直接上传
**文件**: `lib/services/api/oss_api.dart`  
**问题**:
- 图片直接上传原始字节
- 无压缩、无尺寸调整
- 浪费带宽和存储

**修复**: 使用 `image` 包压缩图片

---

#### P1-10: 加密失败时阻止明文发送但体验差
**文件**: `lib/providers/chat_provider.dart:1225-1254`  
**问题**:
```dart
if (_e2eeService.isIdentityKeyMissing) {
  throw StateError('E2EE 身份密钥缺失，无法安全发送消息');
}
```
- 用户输入消息后才抛出错误
- 应该在输入框提前提示

**修复**: 在UI层检查加密状态，禁用发送按钮

---

## 🟡 P2 一般问题

### 后端问题

#### P2-1: 日志级别不规范
**文件**: 全局  
**问题**: 
- 混用 `fmt.Println` 和 `log.Printf`
- 无结构化日志
- 无日志级别区分

**修复**: 统一使用 `zerolog` 或 `zap`

---

#### P2-2: 配置文件读取无容错
**文件**: `internal/config/config.go:20-30`  
**问题**:
```go
viper.ReadInConfig() // panic if file not found
```
- 配置文件缺失直接panic
- 无默认值fallback

**修复**: 提供合理默认值

---

#### P2-3: HTTP超时未设置
**文件**: `cmd/server/main.go`  
**问题**:
```go
srv := &http.Server{
    Addr:    ":8080",
    Handler: router,
}
```
- 无 `ReadTimeout`、`WriteTimeout`
- 慢客户端攻击风险

**修复**: 设置30秒读超时，60秒写超时

---

#### P2-4: 群组成员数无上限
**文件**: `internal/service/group_service.go`  
**问题**: 无群组成员数量限制

**修复**: 限制为500人

---

#### P2-5: 好友请求无过期机制
**文件**: `internal/repo/friend_repo.go`  
**问题**: 待处理好友请求永久存在

**修复**: 30天自动过期

---

#### P2-6: OSS签名URL无过期时间
**文件**: `internal/handler/oss_handler.go`  
**问题**: URL永久有效，可被滥用

**修复**: 设置1小时过期

---

#### P2-7: 无API限流
**文件**: 全局  
**问题**: 同一用户可无限请求

**修复**: 添加令牌桶或滑动窗口限流

---

#### P2-8: 群消息无已读状态
**文件**: `internal/model/message.go`  
**问题**: 只有单聊已读，群聊未实现

**影响**: 功能不完整

---

#### P2-9: 在线状态无心跳
**文件**: `internal/handler/websocket_handler.go`  
**问题**: 用户在线状态依赖WebSocket连接，但无定期更新

**修复**: 每5分钟更新在线状态

---

#### P2-10: 数据库索引不足
**文件**: 数据库迁移文件（未找到）  
**问题**: 
- `messages` 表无 `created_at` 索引
- `friend_requests` 无 `created_at` 索引
- 查询可能很慢

**修复**: 添加复合索引

---

#### P2-11: 事务使用不当
**文件**: `internal/service/group_service.go:50-80`  
**问题**:
```go
db.Create(&group)
db.Create(&member1)
db.Create(&member2)
```
- 非原子操作，可能部分成功
- 无事务包裹

**修复**: 使用 `db.Transaction()`

---

#### P2-12: 缓存雪崩风险
**文件**: Redis缓存使用  
**问题**: 所有缓存使用相同过期时间

**修复**: 添加随机过期时间偏移

---

### 前端问题

#### P2-13: 图片缓存无上限
**文件**: `lib/services/file_cache_service.dart`  
**问题**: 缓存可能无限增长

**修复**: LRU淘汰策略，限制100MB

---

#### P2-14: 视频缩略图同步生成
**文件**: `lib/providers/chat_provider.dart:917-921`  
**问题**:
```dart
unawaited(
  VideoThumbnailService.generateAndCacheThumbnail(displayMsg.content),
);
```
- 虽然使用了unawaited，但生成操作仍可能阻塞
- 应该后台队列处理

---

#### P2-15: 定位权限未正确处理
**文件**: `lib/services/location_service.dart`  
**问题**: 权限被拒后未提示用户

---

#### P2-16: 生物识别锁定逻辑复杂
**文件**: `lib/providers/biometric_lock_provider.dart`  
**问题**: 状态机过于复杂，难以维护

---

#### P2-17: 深色模式颜色对比度不足
**文件**: `lib/main.dart:206-220`  
**问题**: 某些颜色组合可读性差

---

#### P2-18: 国际化字符串硬编码
**文件**: 多处  
**问题**: 大量中文硬编码，无法国际化

**修复**: 提取到 `l10n/app_localizations.dart`

---

#### P2-19: 无网络错误重试机制
**文件**: API调用  
**问题**: 网络错误直接失败，不重试

---

#### P2-20: 聊天输入框无草稿保存
**文件**: `lib/screens/chat_screen.dart`  
**问题**: 切换对话时输入内容丢失

---

## 🔵 P3 优化建议

### 后端优化

#### P3-1: 添加分布式追踪
**建议**: 集成 OpenTelemetry  
**收益**: 性能问题定位

---

#### P3-2: 添加Prometheus监控
**建议**: 暴露 `/metrics` 端点  
**收益**: 可观测性

---

#### P3-3: 使用连接池优化HTTP客户端
**文件**: OSS上传  
**问题**: 每次请求新建连接

---

#### P3-4: 消息分页加载
**问题**: 一次加载所有历史消息

---

#### P3-5: WebSocket消息批量发送
**问题**: 每条消息单独发送

---

#### P3-6: 添加全文搜索
**建议**: 集成 Elasticsearch

---

#### P3-7: 图片CDN加速
**建议**: 使用CloudFlare或阿里云CDN

---

#### P3-8: 数据库读写分离
**建议**: 主从复制

---

#### P3-9: Redis持久化策略
**问题**: 未配置AOF

---

### 前端优化

#### P3-10: 使用Isolate处理图片
**问题**: 图片解码在主线程

---

#### P3-11: 消息列表虚拟滚动
**问题**: 长对话加载所有消息

---

#### P3-12: 预加载下一页消息
**建议**: 滚动到80%时预加载

---

#### P3-13: 使用WebP格式
**问题**: PNG/JPEG占用空间大

---

#### P3-14: 音视频通话添加回声消除
**建议**: 启用AEC

---

#### P3-15: 添加消息搜索
**功能**: 搜索聊天历史

---

---

## 📊 代码质量指标

### 后端
- **测试覆盖率**: 0% (❌ 无单元测试)
- **代码重复率**: 估计15%
- **平均函数长度**: 50行
- **循环复杂度**: 中等

### 前端
- **测试覆盖率**: 0% (❌ 无单元测试)
- **Widget复杂度**: 高 (某些Widget超过500行)
- **状态管理**: Provider (合理)
- **代码规范**: 通过 `flutter analyze`

---

## 🎯 修复优先级建议

### 第一阶段 (本周)
1. 修复所有P0问题
2. 添加基本单元测试
3. 修复SQL注入和密码强度问题

### 第二阶段 (下周)
1. 修复所有P1问题
2. 添加API限流
3. 优化数据库索引

### 第三阶段 (两周内)
1. 修复所有P2问题
2. 添加监控和日志
3. 性能优化

### 第四阶段 (一个月内)
1. 实现P3优化建议
2. 完善文档
3. 压力测试

---

## 📝 总结

当前代码库质量：**中等偏下**

**优点**:
- ✅ 整体架构清晰
- ✅ 代码风格统一
- ✅ 端到端加密实现完整
- ✅ Flutter代码通过静态分析

**缺点**:
- ❌ 无测试覆盖
- ❌ 资源泄漏风险高
- ❌ 错误处理不完善
- ❌ 缺少性能优化

**关键风险**:
1. WebSocket连接泄漏可能导致生产事故
2. 并发安全问题可能导致数据错乱
3. 无限流机制容易被DDoS

**建议**: 在上线前必须修复所有P0和P1问题。
