# AI视觉识别功能使用指南

## 概述

AIThink现在支持使用AI视觉识别技术来智能分析网页，解决网页动态变更导致选择器失效的问题。

## 工作原理

传统的网页自动化依赖固定的CSS选择器或XPath来定位元素。当网页结构变化时，这些选择器就会失效。

AI视觉识别通过以下方式解决此问题：

1. **截图分析**：对当前页面进行截图
2. **AI识别**：将截图发送给AI视觉模型（如OpenAI GPT-4V）进行分析
3. **智能定位**：AI返回页面元素的描述和建议选择器
4. **自动操作**：使用AI提供的选择器进行点击、输入等操作

## 配置AI视觉服务

### 1. 支持的AI服务

系统支持以下AI视觉服务：

- **OpenAI GPT-4V**：推荐使用，识别准确度高
- **百度OCR**：国内访问快，但需要自行识别元素位置
- **腾讯云OCR**：类似百度OCR
- **自定义API**：接入任何支持图片分析的API

### 2. 配置方法

使用配置管理API进行配置：

#### 获取当前配置

```bash
curl http://localhost:8081/api/v1/config
```

#### 配置OpenAI GPT-4V（推荐）

```bash
curl -X POST http://localhost:8081/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "openai": {
      "api_key": "sk-your-openai-key",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-4-vision-preview"
    }
  }'
```

#### 配置百度OCR

```bash
curl -X POST http://localhost:8081/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "baidu",
    "baidu": {
      "api_key": "your-baidu-api-key",
      "secret_key": "your-baidu-secret-key"
    }
  }'
```

#### PowerShell示例

```powershell
$body = @{
    provider = "openai"
    openai = @{
        api_key = "sk-your-openai-key"
        base_url = "https://api.openai.com/v1"
        model = "gpt-4-vision-preview"
    }
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/api/v1/config -Method POST -ContentType "application/json" -Body $body
```

## 工作流程

### 在登录流程中的应用

当系统执行登录操作时：

1. **常规方法优先**：首先尝试使用JavaScript和DOM查询查找元素（手机号输入框、登录按钮等）
2. **AI后备方案**：如果常规方法失败，自动使用AI视觉识别
3. **截图分析**：对当前页面截图并发送给配置的AI服务
4. **智能操作**：使用AI返回的选择器进行操作

### 代码示例

登录流程会自动使用AI视觉识别：

```go
// 在zhipu.go的Login方法中
analyer := NewPageAnalyier(ctx, true, screenshotDir)

// 查找手机号输入框（会先尝试常规方法，失败后使用AI）
phoneSelector := analyer.FindPhoneInput()
if phoneSelector == "" {
    return fmt.Errorf("无法找到手机号输入框")
}

// 输入手机号
analyer.SmartInput(phoneSelector, username)

// 查找登录按钮（同样会尝试AI识别）
loginBtn := analyer.FindButtonByText([]string{"登录", "登 录"})
```

## 优势

### 1. 适应网页变化
- 不依赖固定的选择器
- AI可以识别页面元素的语义，而不是结构
- 即使页面改版，只要功能相同，AI仍能识别

### 2. 提高健壮性
- 多重保障：常规方法 + AI后备
- 自动截图保存，便于调试
- 详细的日志记录

### 3. 易于扩展
- 支持多种AI服务
- 可以轻松添加新的AI服务商
- 配置简单，无需修改代码

## 注意事项

### 1. API Key安全
- 配置文件中不返回完整的API Key（只告知是否已配置）
- 建议在生产环境使用环境变量或密钥管理服务

### 2. 性能考虑
- AI视觉识别比传统方法慢（需要网络请求和AI处理）
- 建议只对关键操作使用AI识别
- 常规方法优先，AI作为后备

### 3. 成本考虑
- OpenAI GPT-4V按token计费
- 建议设置合理的截图频率
- 可以考虑缓存识别结果

### 4. 准确性
- AI识别不是100%准确
- 建议保存截图和AI响应，便于调试
- 可以设置人工审核机制

## 测试

### 1. 检查配置

```bash
curl http://localhost:8081/api/v1/config
```

### 2. 测试登录流程

```bash
curl -X POST http://localhost:8081/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "username": "13800138000",
    "password": ""
  }'
```

观察日志输出，查看是否使用了AI视觉识别。

## 调试

### 截图保存位置

截图默认保存在 `screenshots/` 目录：

- `01_initial_page.png` - 初始页面
- `ai_find_phone.png` - AI查找手机号输入框时的截图
- `ai_find_button.png` - AI查找按钮时的截图

### 日志分析

启用debug模式后，会输出详细的日志：

```
正在分析页面...
========== 页面分析 ==========
URL: https://chatglm.cn/...
页面状态: login_page
==============================
常规方法未找到手机号输入框，尝试使用AI视觉识别...
截图已保存: screenshots/ai_find_phone.png
AI找到手机号输入框: #phone-input (文本: 手机号)
```

## 未来计划

- [ ] 支持更多AI视觉服务
- [ ] 实现识别结果缓存
- [ ] 添加人工审核界面
- [ ] 支持自定义提示词模板
- [ ] 实现批量页面训练功能

## 常见问题

### Q: AI识别太慢怎么办？
A: 可以调整策略，只在必要时使用AI识别。或者考虑使用更快的OCR服务。

### Q: 如何提高识别准确率？
A: 使用更先进的AI模型（如GPT-4V），优化提示词，提供更多上下文信息。

### Q: 可以同时使用多个AI服务吗？
A: 当前版本只支持配置一个provider。未来可以扩展为多种服务组合使用。

### Q: 百度OCR能识别元素位置吗？
A: 百度OCR主要做文字识别，不太适合直接定位元素。建议使用OpenAI GPT-4V这类多模态模型。
