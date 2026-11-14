@echo off
rem Compila una copia de prueba del agente y abre el panel, sin tocar el agente real.
rem Datos de prueba: %TEMP%\backupamg-dev  (borra esa carpeta para empezar de cero)
rem Doble clic para ejecutarlo. Para parar el agente de prueba, cierra su ventana.
cd /d "%~dp0"

echo [1/4] Compilando...
if not exist "bin\dev" mkdir "bin\dev"
go build -o "bin\dev\agent.exe" .\cmd\agent
if errorlevel 1 (
  echo.
  echo ERROR: no se pudo compilar. Revisa los mensajes de arriba.
  pause
  exit /b 1
)
copy /y "bin\restic.exe" "bin\dev\restic.exe" >nul

set "BACKUP_AGENT_DATA_DIR=%TEMP%\backupamg-dev"
set "BACKUP_AGENT_PIPE=\\.\pipe\backupamg-dev"

rem El agente real usa el puerto 47800: el de prueba usa otro para no chocar.
if not exist "%BACKUP_AGENT_DATA_DIR%" mkdir "%BACKUP_AGENT_DATA_DIR%"
if not exist "%BACKUP_AGENT_DATA_DIR%\config.yaml" (
  >"%BACKUP_AGENT_DATA_DIR%\config.yaml" echo panel:
  >>"%BACKUP_AGENT_DATA_DIR%\config.yaml" echo   port: 47811
)

echo [2/4] Parando un agente de prueba anterior, si lo hay...
taskkill /f /fi "WINDOWTITLE eq Agente de prueba BackupAMG*" >nul 2>&1

echo [3/4] Arrancando el agente de prueba en otra ventana...
start "Agente de prueba BackupAMG" "bin\dev\agent.exe" run
timeout /t 3 /nobreak >nul

echo [4/4] Abriendo el panel en el navegador...
"bin\dev\agent.exe" panel

echo.
echo Listo. El agente de prueba sigue en su ventana: cierrala cuando termines.
pause
