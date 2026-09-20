@echo off
setlocal EnableExtensions
cd /d "%~dp0"

echo ============================================
echo   IT Operations Hub - Build
echo ============================================
echo.

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

echo Node: & node --version
echo npm:  & npm --version
echo.

rem The ZIP already contains a Windows backend executable. Rebuilding it requires
rem Go + a working GCC/cgo toolchain because the SQLite driver uses cgo.
where go >nul 2>nul
if errorlevel 1 goto USE_PREBUILT
where gcc >nul 2>nul
if errorlevel 1 goto USE_PREBUILT

echo [1/3] Building backend with Go + GCC...
cd /d "%~dp0backend"
go build -mod=vendor -o "%~dp0dist\ticketing-backend.exe" .\cmd\server
if errorlevel 1 (
  echo.
  echo WARNING: Backend compilation failed.
  echo The supplied prebuilt backend will be used instead.
  cd /d "%~dp0"
  if not exist "dist\ticketing-backend.exe" (
    echo ERROR: No usable backend executable exists.
    pause
    exit /b 1
  )
  goto FRONTEND
)
cd /d "%~dp0"
goto FRONTEND

:USE_PREBUILT
if not exist "%~dp0dist\ticketing-backend.exe" (
  echo ERROR: Go/GCC are unavailable and the supplied backend executable is missing.
  echo Install Go 1.22+ and GCC/MinGW, then run build.bat again.
  pause
  exit /b 1
)
echo [1/3] Using supplied Windows backend executable.
echo       Go/GCC not detected; backend compilation skipped.

goto FRONTEND

:FRONTEND
echo.
echo [2/3] Installing frontend dependencies...
cd /d "%~dp0frontend"
if exist package-lock.json (
  call npm ci --no-audit --no-fund
) else (
  call npm install --no-audit --no-fund
)
if errorlevel 1 (
  echo.
  echo ERROR: Frontend dependency installation failed.
  echo Check the npm error shown above. Do not close this window.
  pause
  exit /b 1
)

echo.
echo [3/3] Building frontend (production)...
if exist ".next" (
  echo Clearing previous Next.js build cache...
  rmdir /s /q ".next"
)
echo Turbopack root: %CD%
call npm run build
if errorlevel 1 (
  echo.
  echo ERROR: Frontend production build failed.
  echo The exact Next.js error is shown above.
  pause
  exit /b 1
)

if not exist ".next\BUILD_ID" (
  echo ERROR: Next.js reported success but .next\BUILD_ID was not created.
  pause
  exit /b 1
)

cd /d "%~dp0"
echo.
echo ============================================
echo   BUILD COMPLETE
echo  Backend:  dist\ticketing-backend.exe
echo  Frontend: frontend\.next
echo.
echo   Run run.bat to start the application.
echo ============================================
pause
exit /b 0
