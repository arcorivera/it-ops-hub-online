@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

echo ============================================
echo   IT Operations Hub
echo ============================================
echo.

if not exist "dist\ticketing-backend.exe" (
  echo ERROR: dist\ticketing-backend.exe not found.
  echo Run build.bat first.
  pause
  exit /b 1
)

where node >nul 2>nul
if errorlevel 1 (
  echo ERROR: Node.js is not installed or not on PATH.
  echo Install Node.js 20.9 or newer, then reopen this window.
  pause
  exit /b 1
)
where npm >nul 2>nul
if errorlevel 1 (
  echo ERROR: npm is not installed or not on PATH.
  pause
  exit /b 1
)

rem Make run.bat self-healing: if node_modules is missing, install the locked dependencies.
if not exist "frontend\node_modules\.bin\next.cmd" (
  echo.
  echo Frontend dependencies are missing. Installing them now...
  cd /d "%~dp0frontend"
  if exist package-lock.json (
    call npm ci --no-audit --no-fund
  ) else (
    call npm install --no-audit --no-fund
  )
  if errorlevel 1 (
    echo.
    echo ERROR: npm dependency installation failed.
    echo Check the npm error above.
    pause
    exit /b 1
  )
  cd /d "%~dp0"
)

if not exist "frontend\node_modules\.bin\next.cmd" (
  echo ERROR: Next.js was not installed correctly.
  pause
  exit /b 1
)

rem Start from a clean Next.js development cache so a previous broken build
rem cannot be reused after replacing the source ZIP.
if exist "frontend\.next" (
  echo Clearing previous Next.js cache...
  rmdir /s /q "frontend\.next"
)

rem Always run the current source in development mode. This prevents a stale
rem .next production build from masking source fixes during local operation.
set FRONTEND_CMD=npm run dev -- -p 3000 -H 0.0.0.0

set "LAN_IP="
for /f "tokens=2 delims=:" %%A in ('ipconfig ^| findstr /R /C:"IPv4 Address" /C:"IPv4 Address\. \.\. \.\."') do if not defined LAN_IP (
  set "LAN_IP=%%A"
  set "LAN_IP=!LAN_IP: =!"
)

if defined LAN_IP (
  echo LAN IP: !LAN_IP!
  set "ITOPS_ALLOWED_ORIGINS=http://!LAN_IP!:3000"
) else (
  echo LAN IP could not be detected; localhost will still work.
  set "ITOPS_ALLOWED_ORIGINS="
)

echo.
echo Starting backend on http://localhost:8080 ...
start "IT Operations Hub - Backend" /D "%~dp0" cmd /k "set ITOPS_PORT=8080&&set ITOPS_ALLOWED_ORIGINS=!ITOPS_ALLOWED_ORIGINS!&&dist\ticketing-backend.exe"

call :WAIT_HTTP "http://localhost:8080/health" 30
if errorlevel 1 (
  echo.
  echo ERROR: Backend did not become ready on port 8080.
  echo Check the Backend window for the actual error.
  pause
  exit /b 1
)

echo Backend is ready.
echo.
echo Frontend root: %~dp0frontend
echo Starting frontend on http://localhost:3000 ...
start "IT Operations Hub - Frontend" /D "%~dp0frontend" cmd /k "!FRONTEND_CMD!"

call :WAIT_HTTP "http://localhost:3000" 30
if errorlevel 1 (
  echo.
  echo ERROR: Frontend did not become ready on port 3000.
  echo Check the Frontend window for the actual Next.js error.
  pause
  exit /b 1
)

echo.
echo ============================================
echo   IT Operations Hub is READY
echo  http://localhost:3000
echo  http://localhost:8080/health
if defined LAN_IP echo  LAN: http://!LAN_IP!:3000
echo ============================================
echo.
start "" "http://localhost:3000"
pause
exit /b 0

:WAIT_HTTP
set "URL=%~1"
set "TRIES=%~2"
for /L %%N in (1,1,%TRIES%) do (
  powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $r=Invoke-WebRequest -UseBasicParsing -Uri '%URL%' -TimeoutSec 2; if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 500) { exit 0 } } catch {} ; exit 1" >nul 2>nul
  if not errorlevel 1 exit /b 0
  timeout /t 1 /nobreak >nul
)
exit /b 1
