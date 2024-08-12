@echo off
setlocal enabledelayedexpansion
protoc -I . --go_out=. --go-triple_out=. *.proto

REM 使用for循环遍历.pb.go文件
for %%f in (*.pb.go) do (
    REM 使用powershell来执行字符串替换操作，类似于sed
    @REM powershell -Command "(gc '%%f') -replace ',omitempty' | sc '%%f.tmp'"
    powershell -Command "& {Get-Content '%%f' -Encoding UTF8 | ForEach-Object { $_ -replace ',omitempty' } | Set-Content '%%f.tmp' -Encoding UTF8}"
    REM 移动临时文件替换原文件，类似于mv
    move /Y "%%f.tmp" "%%f"
)

REM 设置数组（在批处理中使用for /f和dir命令来模拟数组功能）
for /f "delims=" %%i in ('dir /b *.pb.go') do (
    set "file=%%i"
    REM 调用protoc-go-inject-tag工具，注意路径和命令格式可能需要根据实际情况调整
    call protoc-go-inject-tag -input=!file!
)

endlocal