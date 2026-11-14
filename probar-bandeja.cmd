@echo off
rem Prueba el icono de la bandeja. Doble clic para ejecutarlo.
rem   1 = Solo icono y menu (rapido; el icono sale gris porque no hay agente conectado).
rem   2 = Todo: sustituye el agente y el icono REALES de bin\ y los arranca (colores y avisos reales).
cd /d "%~dp0"

echo.
echo  PRUEBA DE LA BANDEJA DE BACKUPAMG
echo  ---------------------------------
echo  1) Solo ver el icono y el menu (no toca tu agente)
echo  2) Probarlo todo (para y sustituye el agente y el icono de bin\)
echo.
choice /c 12 /n /m "Elige 1 o 2: "
if errorlevel 2 goto todo
goto solo

:solo
echo.
echo [1/3] Cerrando el icono anterior...
taskkill /f /im tray.exe >nul 2>&1
echo [2/3] Compilando el icono...
if not exist "bin\dev" mkdir "bin\dev"
go build -o "bin\dev\tray.exe" .\cmd\tray
if errorlevel 1 goto error
echo [3/3] Arrancando el icono...
start "" "bin\dev\tray.exe"
echo.
echo Listo. Busca el escudo en la bandeja (puede estar en la flecha ^ de iconos ocultos).
echo Saldra GRIS porque no esta conectado a ningun agente. Prueba el menu y "Ocultar el icono".
goto fin

:todo
echo.
echo ATENCION: se va a PARAR y sustituir el agente y el icono que estan en marcha.
echo Si usas ese agente para copias de verdad, se interrumpe un momento.
choice /c SN /n /m "Quieres continuar? (S/N): "
if errorlevel 2 goto fin
echo.
echo [1/5] Parando agente e icono...
taskkill /f /im tray.exe >nul 2>&1
taskkill /f /im agent.exe >nul 2>&1
timeout /t 2 /nobreak >nul
echo [2/5] Compilando agent.exe...
go build -o "bin\agent.exe" .\cmd\agent
if errorlevel 1 goto error
echo [3/5] Compilando tray.exe...
go build -o "bin\tray.exe" .\cmd\tray
if errorlevel 1 goto error
echo [4/5] Arrancando el agente en otra ventana...
start "Agente BackupAMG" "bin\agent.exe" run
timeout /t 3 /nobreak >nul
echo [5/5] Arrancando el icono...
start "" "bin\tray.exe"
echo.
echo Listo. El escudo debe tomar el color del estado (verde, azul, ambar, rojo o gris).
echo Para parar el agente, cierra su ventana "Agente BackupAMG".
echo Si el agente no arranca, prueba a ejecutar este fichero como administrador.
goto fin

:error
echo.
echo ERROR: no se pudo compilar. Revisa los mensajes de arriba.

:fin
echo.
pause
