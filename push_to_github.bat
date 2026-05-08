@echo off
chcp 65001 >nul
echo ================================================
echo        AIThink 项目推送到 GitHub 脚本
echo ================================================
echo.

REM 检查是否在正确的目录
if not exist ".git" (
    echo [错误] 未找到 .git 目录，请在项目根目录运行此脚本
    pause
    exit /b 1
)

echo [1/4] 检查 Git 配置...
git config user.name >nul 2>&1
if errorlevel 1 (
    echo [警告] Git 用户名未配置
    set /p username="请输入您的 GitHub 用户名: "
    git config user.name "%username%"
)

git config user.email >nul 2>&1
if errorlevel 1 (
    echo [警告] Git 邮箱未配置
    set /p email="请输入您的邮箱 (建议使用GitHub绑定邮箱): "
    git config user.email "%email%"
)

echo.
echo [2/4] 检查 SSH 密钥...
if not exist "%USERPROFILE%\.ssh\id_rsa.pub" (
    echo [警告] 未找到 SSH 密钥
    echo 请先运行以下命令生成 SSH 密钥:
    echo ssh-keygen -t rsa -b 4096 -C "你的邮箱"
    echo.
    echo 然后将公钥添加到 GitHub:
    echo 1. 访问 https://github.com/settings/keys
    echo 2. 点击 "New SSH key"
    echo 3. 粘贴 %USERPROFILE%\.ssh\id_rsa.pub 的内容
    pause
    exit /b 1
)

echo.
echo [3/4] 检查远程仓库配置...
git remote -v | find "origin" >nul 2>&1
if errorlevel 1 (
    echo [设置] 添加远程仓库...
    git remote add origin git@github.com:caochengjian/AIThink.git
) else (
    echo [信息] 远程仓库已配置:
    git remote -v
)

echo.
echo [4/4] 推送代码到 GitHub...
echo 正在推送到 master 分支...
git push -u origin master

if errorlevel 1 (
    echo.
    echo [失败] 推送失败，可能的原因:
    echo 1. GitHub 仓库不存在，请先创建:
    echo    - 访问 https://github.com/new
    echo    - 仓库名: AIThink
    echo    - 选择 Private (私有)
    echo    - 不要勾选 "Initialize this repository with a README"
    echo 2. SSH 密钥未添加到 GitHub
    echo 3. 网络连接问题
    echo.
    echo 请检查后重试
    pause
    exit /b 1
) else (
    echo.
    echo ================================================
    echo [成功] 代码已成功推送到 GitHub!
    echo 仓库地址: https://github.com/caochengjian/AIThink
    echo ================================================
)

pause
