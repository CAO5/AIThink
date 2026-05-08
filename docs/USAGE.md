# AIThink 使用指南

## 服务已启动

服务当前运行在: `http://localhost:8081`

## API 接口测试

### 1. 健康检查

```powershell
Invoke-RestMethod -Uri http://localhost:8081/health -Method Get
```

预期返回:
```json
{
  "code": 0,
  "message": "服务正常运行"
}
```

### 2. 登录智谱清言

```powershell
$loginBody = @{
    platform = "zhipu"
    username = "你的手机号"
    password = "验证码"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/api/v1/login `
    -Method Post `
    -Body $loginBody `
    -ContentType "application/json"
```

**注意**: 
- 首次运行会打开 Chrome 浏览器窗口
- 需要手动输入验证码完成登录
- 登录成功后会返回 `session_id`，请保存备用

预期返回:
```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "session_id": "zhipu_1234567890",
    "message": "登录成功"
  }
}
```

### 3. 向智谱清言提问

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

预期返回:
```json
{
  "code": 0,
  "message": "提问成功",
  "data": {
    "answer": "我是智谱清言，是由智谱AI公司开发的...",
    "session_id": "zhipu_1234567890"
  }
}
```

### 4. 登出

```powershell
Invoke-RestMethod -Uri "http://localhost:8081/api/v1/logout?session_id=你的session_id" `
    -Method Post
```

## 使用流程总结

```
1. 启动服务 (已完成)
   ./bin/aithink.exe

2. 调用登录接口
   POST /api/v1/login
   -> 获得 session_id

3. 使用 session_id 提问
   POST /api/v1/ask
   -> 获得 AI 回答

4. 使用完毕登出
   POST /api/v1/logout
```

## 停止服务

在运行服务的终端按 `Ctrl + C` 停止服务。

## 常见问题

### 1. 浏览器没有自动打开
- 检查是否安装了 Chrome 浏览器
- 检查程序是否有足够权限

### 2. 登录时验证码如何处理
- 程序会打开浏览器窗口
- 在浏览器中手动完成验证码输入
- 程序会自动检测登录状态

### 3. 提问没有响应
- 检查 session_id 是否正确
- 检查网络连接
- 查看服务日志输出

### 4. 端口被占用
修改 `cmd/server/main.go` 中的默认端口，或设置环境变量:
```powershell
$env:PORT="8082"
./bin/aithink.exe
```

## 下一步

- 完善智谱清言的登录流程（自动处理验证码）
- 添加更多 AI 平台支持（ChatGPT、Claude 等）
- 实现会话池管理，提高性能
- 添加 API 认证和限流

## 项目文档

- [README.md](file:///d:/wan_workspase/AIThink/README.md) - 项目概述
- [INSTALL.md](file:///d:/wan_workspase/AIThink/INSTALL.md) - 安装指南
- [docs/API.md](file:///d:/wan_workspase/AIThink/docs/API.md) - API 接口文档
- [docs/DEVELOPMENT.md](file:///d:/wan_workspase/AIThink/docs/DEVELOPMENT.md) - 开发文档
