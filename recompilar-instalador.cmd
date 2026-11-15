@echo off
rem Recompila agent.exe, tray.exe y el instalador (dist\BackupAgent-Setup.exe).
rem Doble clic para ejecutarlo. Necesita Go e Inno Setup 6 instalados.
cd /d "%~dp0"

echo.
echo  RECOMPILAR EL INSTALADOR DE BACKUPAMG
echo  -------------------------------------
echo  Se va a PARAR el agente y el icono que esten en marcha, porque se
echo  sustituyen los ficheros de bin\ por los compilados ahora.
echo.
choice /c SN /n /m "Quieres continuar? (S/N): "
if errorlevel 2 goto fin

set "ISCC="
if exist "%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe" set "ISCC=%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if exist "%ProgramFiles%\Inno Setup 6\ISCC.exe" set "ISCC=%ProgramFiles%\Inno Setup 6\ISCC.exe"
if exist "%LocalAppData%\Programs\Inno Setup 6\ISCC.exe" set "ISCC=%LocalAppData%\Programs\Inno Setup 6\ISCC.exe"
if not defined ISCC (
  echo.
  echo ERROR: no encuentro Inno Setup 6. Instalalo desde https://jrsoftware.org/isinfo.php
  goto error
)
if not exist "bin\restic.exe" (
  echo.
  echo ERROR: falta bin\restic.exe. El instalador lo necesita.
  goto error
)

echo.
echo [1/4] Parando agente e icono...
taskkill /f /im tray.exe >nul 2>&1
taskkill /f /im agent.exe >nul 2>&1
timeout /t 2 /nobreak >nul

echo [2/4] Compilando agent.exe...
go build -o "bin\agent.exe" .\cmd\agent
if errorlevel 1 goto error

echo [3/4] Compilando tray.exe...
go build -o "bin\tray.exe" .\cmd\tray
if errorlevel 1 goto error

echo [4/4] Compilando el instalador...
"%ISCC%" "installer\BackupAgent.iss"
if errorlevel 1 goto error

echo.
echo Listo: dist\BackupAgent-Setup.exe
echo Recuerda probarlo en una maquina virtual con el antivirus por defecto.
if exist "dist\BackupAgent-Setup.exe" explorer /select,"%~dp0dist\BackupAgent-Setup.exe"
goto fin

:error
echo.
echo ERROR: no se pudo completar. Revisa los mensajes de arriba.

:fin
echo.
pause
