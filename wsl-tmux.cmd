@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0dialtone.ps1" tmux %*
endlocal
