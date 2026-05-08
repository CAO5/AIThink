# 自动登录功能说明

## 实现方案

AIThink 使用 **Chrome 用户数据目录（User Data Directory）** 方案实现自动登录。

### 工作原理

1. **首次登录**：用户输入手机号后，程序打开浏览器，用户手动完成登录（包括验证码）
2. **保存状态**：登录成功后，Chrome 会自动将 Cookie 和登录状态保存到指定的用户数据目录
3. **自动登录**：下次使用相同的 `user_data_dir` 启动，Chrome 会自动加载之前的登录状态，无需重新登录

### 优势

- ✅ 无需手动处理 Cookie
- ✅ Chrome 原生支持，稳定可靠
- ✅ 支持多个账号（不同 user_data_dir）
- ✅ 更接近真实用户行为

## 使用方法

### 1. 首次登录（需要手动输入验证码）

```powershell
$loginBody = @{
    platform = "zhipu"
    username = "你的手机号"
    password = "123456"  # 实际登录时不需要密码，这里只是占位
    user_data_dir = "sessions/zhipu_main"  # 指定用户数据目录
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/api/v1/login `
    -Method Post `
    -Body $loginBody `
    -ContentType "application/json"
```

**注意**：
- 首次登录会打开浏览器窗口
- 需要手动输入验证码完成登录
- 登录成功后，Cookie 会自动保存到 `sessions/zhipu_main` 目录

### 2. 后续自动登录

使用相同的 `user_data_dir` 再次调用登录接口：

```powershell
$loginBody = @{
    platform = "zhipu"
    username = "你的手机号"
    password = "123456"
    user_data_dir = "sessions/zhipu_main"  # 与首次登录使用相同的目录
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/api/v1/login `
    -Method Post `
    -Body $loginBody `
    -ContentType "application/json"
```

**效果**：
- Chrome 会自动加载 `sessions/zhipu_main` 目录中的登录状态
- 无需重新输入验证码
- 实现"自动登录"

### 3. 提问（登录后）

```powershell
$askBody = @{
    platform = "zhipu"
    session_id = "上一步返回的session_id"
    question = "你好，请介绍一下自己"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/api/v1/ask `
    -Method Post `
    -Body $askBody `
    -ContentType "application/json"
```

## 多账号支持

可以为不同账号使用不同的 `user_data_dir`：

```json
// 账号1
{
  "platform": "zhipu",
  "username": "13800138000",
  "user_data_dir": "sessions/zhipu_account1"
}

// 账号2
{
  "platform": "zhipu",
  "username": "13900139000",
  "user_data_dir": "sessions/zhipu_account2"
}
```

## 清除登录状态

如需重新登录，删除对应的用户数据目录即可：

```powershell
Remove-Item -Recurse -Force "sessions/zhipu_main"
```

## 限制说明

### 为什么不能全自动登录？

智谱清言等AI平台通常有以下防护措施：
1. **验证码**：短信验证码或滑块验证码
2. **设备指纹**：检测浏览器指纹
3. **行为分析**：检测自动化行为

要实现完全自动登录（无需任何手动操作），需要：
- 接入打码平台（处理验证码）
- 使用真实浏览器指纹
- 模拟人类操作行为

这些实现复杂且可能违反平台服务条款。

### 当前方案的折中

- ✅ 首次登录后，后续全自动登录
- ✅ 符合平台规范（模拟真实用户）
- ✅ 实现简单，稳定可靠

## 技术细节

### Chrome 用户数据目录

Chrome 浏览器的用户数据目录包含：
- Cookies（登录状态）
- LocalStorage
- Cache
- 其他浏览数据

指定 `user_data_dir` 后，chromedp 会启动一个使用该目录的 Chrome 实例，自动加载所有保存的状态。

### 代码示例

```go
// 创建浏览器会话时指定用户数据目录
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", false),
    chromedp.UserDataDir(userDataDir),  // 指定用户数据目录
)

allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
ctx, cancel := chromedp.NewContext(allocCtx)
```

## 反检测措施

AIThink 已实现多重反检测机制，避免在提问时触发人机验证：

### 1. 浏览器启动参数优化

程序在启动 Chrome 时，已添加以下反检测参数：

```go
// 禁用自动化控制特征（关键）
chromedp.Flag("disable-blink-features", "AutomationControlled"),
// 排除自动化开关
chromedp.Flag("exclude-switches", "enable-automation"),
// 禁用自动化扩展
chromedp.Flag("disable-extensions", true),
chromedp.Flag("disable-automation-extension", true),
// 使用真实 User-Agent
chromedp.Flag("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
// 其他反检测参数
chromedp.Flag("disable-infobars", true),
chromedp.Flag("ignore-certificate-errors", true),
chromedp.Flag("disable-popup-blocking", true),
```

### 2. JavaScript 注入反检测

每次页面操作前，程序会自动注入反检测脚本，覆盖以下检测点：

- **navigator.webdriver**：设置为 `undefined`（最关键）
- **Chrome 自动化标记**：删除 `cdc_adoQpoasnfa76pfcZLmcfl_` 等特征变量
- **CDP 特征**：删除 `__driver_evaluate`、`__webdriver_evaluate` 等可能暴露自动化的属性
- **插件伪装**：模拟真实的 Chrome 插件列表
- **语言伪装**：设置真实的浏览器语言列表
- **平台伪装**：伪装为 Win32 平台
- **硬件信息伪装**：伪装硬件并发数和内存大小
- **权限查询伪装**：伪装通知权限等查询

### 3. 操作行为模拟

- **真人输入速度**：逐字输入，随机延迟 50-150ms
- **操作间隔**：点击、输入等操作间有合理延迟
- **页面导航后重新注入**：确保页面切换后反检测依然生效

### 4. 如何验证反检测效果

1. **检查浏览器控制台**：注入成功会输出 `"反检测脚本已注入（增强版）"`
2. **检查 navigator.webdriver**：在浏览器控制台输入 `navigator.webdriver`，应返回 `undefined`
3. **测试提问功能**：登录后多次提问，观察是否触发人机验证

### 5. 注意事项

- 反检测措施不能 100% 保证不触发验证，取决于目标网站的检测强度
- 如果仍触发验证，可以尝试：
  - 增加操作间隔（修改代码中的 `Sleep` 时间）
  - 使用更真实的 User-Agent（模拟不同设备）
  - 定期更换用户数据目录（模拟新用户）

## 故障排查

### 1. 自动登录失败

**原因**：Cookie 过期或被清除

**解决**：
```powershell
# 删除旧的用户数据目录
Remove-Item -Recurse -Force "sessions/zhipu_main"

# 重新手动登录一次
# 然后后续就可以自动登录了
```

### 2. 用户数据目录权限问题

**原因**：程序没有读写权限

**解决**：确保程序对 `sessions` 目录有读写权限

### 3. 多个实例冲突

**原因**：同时使用相同的 `user_data_dir` 启动多个浏览器实例

**解决**：确保每个会话使用独立的 `user_data_dir`，或等待前一个实例关闭

## 总结

AIThink 的自动登录方案：
- **首次**：手动登录一次（输入验证码）
- **后续**：全自动登录（使用保存的 Cookie）
- **实现**：利用 Chrome 用户数据目录机制
- **优势**：简单、稳定、符合规范

这是一个实用的折中方案，既保证了自动化程度，又避免了复杂的验证码处理。
