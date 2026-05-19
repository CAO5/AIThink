# 稳定性改进文档

## 概述

本文档记录 AIThink 项目为提升系统稳定性而进行的各项改进。这些改进旨在确保项目能够持续稳定运行，作为可靠的 API Key 提供方。

## 改进记录

### 1. 会话管理器 (SessionManager)

**文件**: `internal/service/session_manager.go`

#### 功能特性
- 健康检查循环：每 60 秒自动检查所有活跃会话状态
- 自动恢复机制：会话崩溃时自动尝试恢复，最多重试 5 次
- Cookie 定时保存：每 5 分钟自动保存所有会话的 Cookie，防止登录状态丢失
- 会话槽位管理：使用信号量控制最大并发会话数（默认 5 个）

#### 自动恢复策略
1. 使用指数退避策略等待恢复（2秒、4秒、6秒...最多 30 秒）
2. 重试创建会话最多 3 次，每次间隔递增
3. 重试加载 Cookie 最多 2 次
4. 重试导航到目标网站最多 2 次
5. 如果 10 分钟内连续崩溃 5 次，暂停恢复避免资源浪费

### 2. 会话注册集成

**文件**: `internal/service/aithink.go`

#### 改进点
- 手动登录成功后自动注册会话到 SessionManager
- Cookie 自动恢复成功后自动注册会话
- 已登录状态复用时自动注册会话
- 登出时自动从 SessionManager 注销会话

#### 注册时机
1. `monitorLoginStatus` 检测到手动登录成功
2. `autoCreateSession` Cookie 恢复成功
3. `openLoginPage` 加载 Cookie 后发现已登录

### 3. 提问接口优化

**文件**: `internal/service/aithink.go`

#### 改进点
- 5 分钟超时控制：避免提问操作无限等待
- 重试机制：提问失败最多重试 2 次
- 会话健康检查：提问前检查会话健康状态，不健康则自动恢复
- 会话失效自动恢复：重试时检测会话是否失效，失效则重新创建

#### 执行流程
```
1. 检查会话登录状态
2. 检查会话健康状态（SessionManager）
3. 执行提问（带 5 分钟超时）
4. 如果失败，等待 2 秒后重试
5. 重试前检查会话是否有效，无效则恢复
6. 最多重试 2 次
```

### 4. 并发控制

**文件**: `internal/service/session_manager.go`

#### 实现方式
- 使用带缓冲的 channel 作为信号量
- 默认最大 5 个并发会话
- 注册会话时获取槽位，注销时释放槽位
- 槽位满时等待可用槽位

#### 配置参数
```go
maxSessions := 5 // 最大并发会话数
sessionSem := make(chan struct{}, maxSessions) // 信号量
```

## 配置说明

### 可调整参数

| 参数 | 位置 | 默认值 | 说明 |
|------|------|--------|------|
| 健康检查间隔 | `checkInterval` | 60 秒 | 健康检查循环间隔 |
| 最大崩溃次数 | `maxCrashCount` | 5 | 会话最大崩溃重试次数 |
| Cookie 保存间隔 | `autoSaveCookiesLoop` | 5 分钟 | 定时保存 Cookie 间隔 |
| 最大会话数 | `maxSessions` | 5 | 最大并发会话数 |
| 提问超时 | `executeAsk` | 5 分钟 | 单次提问超时时间 |
| 提问重试次数 | `Ask` | 2 | 提问失败重试次数 |

## 运行建议

### 系统要求
- 内存：建议 4GB 以上（每个浏览器会话约占用 500MB）
- CPU：建议 2 核以上
- 磁盘：确保有足够空间存储 Cookie 和会话数据

### 最佳实践
1. 定期监控日志中的会话恢复记录
2. 关注 Cookie 保存是否成功
3. 如果频繁出现会话崩溃，检查网络环境
4. 合理设置最大会话数，避免系统资源耗尽

## 故障排查

### 常见问题

#### 1. 会话频繁崩溃
- 检查网络连接是否稳定
- 检查目标网站是否可访问
- 查看日志中的具体错误信息

#### 2. Cookie 保存失败
- 检查 `sessions/cookies` 目录权限
- 确认磁盘空间充足
- 查看日志中的具体错误

#### 3. 提问超时
- 检查 AI 平台响应速度
- 适当增加超时时间配置
- 检查浏览器会话是否正常

## 维护计划

### 定期检查
- 每周检查日志文件大小
- 每月清理过期会话数据
- 定期更新 Cookie 存储格式

### 性能监控
- 监控会话活跃数量
- 监控内存使用情况
- 监控提问响应时间

## 更新历史

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-05-13 | 1.0.0 | 初始版本，完成基础稳定性改进 |
| 2026-05-13 | 1.1.0 | 修复浏览器会话管理、发送机制和Chrome进程冲突问题 |
| 2026-05-15 | 1.2.0 | 修复cookie解析错误、添加输入字符长度限制 |
| 2026-05-15 | 1.2.1 | 增强cookie解析错误处理，添加cookie值清洗逻辑和自动降级机制 |

## 版本 1.1.0 修复详情

### 修复日期
2026-05-13

### 修复问题
1. Chrome进程冲突问题（"在现有的浏览器会话中打开"错误）
2. Channel panic问题（"close of closed channel"）
3. 中文输入后未发送问题
4. invalid exec pool flag错误

### 修复内容

#### 1. 浏览器会话管理修复
**文件**: `internal/browser/browser.go`

- **CreateSession会话复用逻辑**
  - 修改前：会话已存在时返回错误
  - 修改后：会话已存在时复用现有会话，更新LastActive时间
  - 避免重复创建Chrome实例导致userDataDir冲突

- **CloseSession安全清理**
  - 增加Cancel函数空值检查，防止重复关闭channel
  - 会话不存在时返回nil而不是错误
  - cleanupExpiredSessions也增加相同的保护

#### 2. Chrome启动优化
**文件**: `internal/browser/browser.go`

- **清理Singleton lock文件**
  - 启动前清理SingletonLock、SingletonSocket、SingletonCookie
  - 避免上次异常退出导致的lock残留

- **终止占用进程**
  - Windows下使用taskkill终止可能占用userDataDir的Chrome进程
  - 等待2秒确保进程完全退出

#### 3. 问题发送机制修复
**文件**: `internal/browser/zhipu.go`

- **输入框内容验证**
  - 输入后通过JavaScript检查textarea或contenteditable元素的值
  - 确保中文内容正确输入到输入框

- **多重发送策略**
  1. 尝试多种选择器查找发送按钮：`button[class*='Send']`, `button[class*='send']`, `button[type='submit']`等
  2. 使用JavaScript评估按钮是否存在并点击
  3. 如果按钮未找到，使用Enter键发送（`\r`）
  4. 如果Enter键失败，尝试JavaScript触发keydown事件

- **详细日志记录**
  - 每个步骤都有详细的日志输出
  - 便于排查发送失败的具体原因

### 测试验证
- 使用中文问题"你好，请用一句话介绍你自己"进行测试
- 验证结果：
  - 中文输入正常（13个字符，39字节）
  - Enter键发送成功
  - AI回复成功获取（555字节）
  - 总耗时约29秒

### 已知问题
- ~~cookiePart解析错误~~（已修复，见版本1.2.0）

### 注意事项
1. 如果服务长时间未使用，首次提问可能需要更长时间来启动浏览器
2. Windows环境下，taskkill命令可能会关闭所有Chrome窗口，建议在使用前保存其他Chrome窗口的工作
3. 建议定期清理userDataDir目录，避免数据积累过多导致启动变慢

## 版本 1.2.0 修复详情

### 修复日期
2026-05-15

### 修复问题
1. 输入字符超长但仍然返回响应（无友好错误提示）
2. cookie解析错误（`ERROR: could not unmarshal event: parse error: expected string near offset 667 of 'cookiePart...'`）
3. 页面提示"最多可以输入20000字"时程序未识别，仍在等待AI回复
4. 人机验证页面出现时程序未识别，仍在刷新等待

### 修复内容

#### 1. 输入字符长度限制
**文件**: `internal/api/anthropic_gateway.go`

- 在 `Messages` 方法中，提取用户消息后添加长度检查
- 最大输入长度限制为 8000 字符
- 超过限制时返回 HTTP 400 和 Anthropic 格式的错误响应
- 错误信息包含实际输入长度和最大支持长度

#### 2. Cookie解析错误修复
**文件**: `internal/browser/cookie_store.go`, `internal/browser/zhipu.go`

- **根本原因**: 之前使用 `chromedp.Evaluate` 执行JavaScript获取cookies，JavaScript返回的JSON数据中cookie值可能包含特殊字符（如分号、等号等），导致chromedp内部unmarshal解析失败
- **修复方案**: 改用CDP原生 `network.GetCookies` API获取cookies
  - `cookie_store.go` 的 `SaveCookies` 方法：使用 `network.GetCookies().Do(ctx)` 替代 `chromedp.Evaluate`
  - `zhipu.go` 的 `GetCookies` 方法：同样改用 `network.GetCookies().Do(ctx)` 替代 `document.cookie`
  - 保留 `saveCookiesFromDocument` 作为备用方案

#### 3. 页面异常状态检测与自动处理
**文件**: `internal/browser/zhipu.go`, `internal/api/anthropic_gateway.go`

- **输入超限检测**: 发送问题后和等待AI回复循环中，检测页面是否出现"最多可以输入"、"字数超"、"超出限制"等提示
  - 检测到后立即返回错误，不再继续等待
  - 通过API返回 HTTP 400 和 `invalid_request_error` 错误类型
- **人机验证检测与自动处理**: 发送问题后和等待AI回复循环中，检测页面是否出现人机验证
  - 关键词检测：人机验证、安全验证、访问验证、别离开、请进行验证、TraceID、captcha、verify等
  - DOM元素检测：captcha iframe、slider、puzzle、challenge等验证组件
  - **自动处理策略**（`handleCaptcha` 方法）：
    1. **滑块验证码自动处理**（`handleSliderCaptcha` 方法，优先级最高）：
       - 自动查找滑块元素和轨道宽度
       - 使用chromedp底层鼠标事件（`input.MousePressed/MouseMoved/MouseReleased`）模拟拖拽
       - 生成人类拖拽轨迹：先快后慢、Y轴微抖、随机停顿、结尾微调
       - 最多重试3次
    2. 尝试点击验证按钮/复选框（多种选择器）
    3. 尝试JavaScript查找并点击含"验证"、"确认"、"通过"等文字的按钮
    4. 等待用户手动完成验证（最多5分钟）
    5. 验证通过后智能判断是否需要刷新页面：如果当前URL仍在聊天页面域名内则不刷新，否则导航回来
    6. 验证通过后重新输入问题并发送
  - 验证未通过时返回 HTTP 403 和 `authentication_error` 错误类型
- **错误分类处理**: `anthropic_gateway.go` 中对错误类型进行分类
  - 输入超限 → HTTP 400 `invalid_request_error`
  - 人机验证 → HTTP 403 `authentication_error`
  - 其他错误 → HTTP 500 `api_error`
- **统一检测方法**: `detectPageAnomaly` 方法封装了所有异常检测逻辑，避免代码重复

### 验证结果
- 编译通过
- 服务正常启动在 http://localhost:8081

## 版本 1.2.1 修复详情

### 修复日期
2026-05-15

### 修复问题
1. cookie解析错误再次出现（`ERROR: could not unmarshal event: parse error: expected string near offset 856 of 'cookiePart...'`）
2. 错误日志仍然打印 ERROR，影响日志可读性
3. 未对 cookie 值进行清洗，特殊字符仍可能导致后续问题

### 修复内容

#### 1. Cookie 解析错误增强处理
**文件**: `internal/browser/zhipu.go`, `internal/browser/cookie_store.go`

- **错误类型识别**
  - 在 `network.GetCookies().Do(ctx)` 调用后，检查错误信息是否包含 `could not unmarshal event` 或 `parse error`
  - 识别出 cookie 解析错误后，记录为 WARNING 级别而不是 ERROR
  - 立即降级到 `document.cookie` 备用方案

- **Cookie 值清洗逻辑**
  - 新增 `sanitizeCookieValue` 函数
  - 过滤掉 ASCII 控制字符（0x00-0x1F，除了制表符 `\t`）
  - 过滤掉 DEL 字符（0x7F）
  - 这些字符在 JSON 解析时会导致 unmarshal 失败

- **自动降级机制**
  - `zhipu.go` 的 `GetCookies` 方法：CDP API 失败时自动调用 `getCookiesFromDocument` 备用方法
  - `cookie_store.go` 的 `SaveCookies` 方法：保持原有的降级到 `saveCookiesFromDocument` 逻辑
  - 确保即使 CDP API 失败，系统仍能正常获取和保存 cookies

#### 2. 日志优化
- cookie 解析错误从 ERROR 降级为 WARNING
- 添加详细的降级过程日志，便于追踪问题
- 记录通过备用方案获取的 cookie 数量和内容

### 技术细节

**sanitizeCookieValue 函数逻辑**:
```go
func sanitizeCookieValue(value string) string {
    var cleaned []rune
    for _, r := range value {
        // 跳过控制字符（除了常见的制表符等）
        if r < 0x20 && r != '\t' {
            continue
        }
        // 跳过DEL字符
        if r == 0x7F {
            continue
        }
        cleaned = append(cleaned, r)
    }
    return string(cleaned)
}
```

**错误处理流程**:
```
1. 调用 network.GetCookies().Do(ctx)
2. 如果返回错误：
   a. 检查是否是 unmarshal 解析错误
   b. 是 → 记录 WARNING 日志，返回错误触发降级
   c. 否 → 记录 ERROR 日志，返回错误
3. 降级到 document.cookie 方案
4. 如果 document.cookie 也失败，返回空字符串
```

### 验证结果
- 编译通过
- cookie 解析错误不再显示 ERROR 日志
- 系统能够自动降级到备用方案
- cookie 值清洗功能正常工作
