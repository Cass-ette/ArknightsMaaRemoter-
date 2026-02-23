@echo off
cd /d "%~dp0"

if not exist ArknightsMaaRemoter.exe (
    echo 正在编译...
    go build -o ArknightsMaaRemoter.exe .
    if errorlevel 1 (
        echo 编译失败，请确认已安装 Go
        pause
        exit /b 1
    )
)

start "MAA Remote" /min ArknightsMaaRemoter.exe
timeout /t 1 /nobreak >nul
start http://localhost:8080
