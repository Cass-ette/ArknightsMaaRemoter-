@echo off
cd /d "%~dp0"

if not exist ArknightsMaaRemoter.exe (
    echo 未找到可执行文件，正在从 GitHub 下载最新版本...
    powershell -Command "Invoke-WebRequest -Uri 'https://github.com/Cass-ette/ArknightsMaaRemoter-/releases/latest/download/ArknightsMaaRemoter.exe' -OutFile 'ArknightsMaaRemoter.exe'"
    if not exist ArknightsMaaRemoter.exe (
        echo 下载失败，尝试从源码编译（需要已安装 Go）...
        go build -o ArknightsMaaRemoter.exe .
        if errorlevel 1 (
            echo.
            echo 启动失败，请选择以下方式之一：
            echo 1. 从 GitHub Releases 手动下载 exe 放到本目录
            echo    https://github.com/Cass-ette/ArknightsMaaRemoter-/releases/latest
            echo 2. 安装 Go 后重新运行此脚本
            echo    https://go.dev/dl/
            pause
            exit /b 1
        )
    )
)

start "MAA Remote" /min ArknightsMaaRemoter.exe
timeout /t 1 /nobreak >nul
start http://localhost:8080
