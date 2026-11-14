# Antivirus

BackupAMG cifra archivos y arranca `restic.exe`. Un antivirus con defensa por comportamiento puede puntuar eso como un programa dañino, sobre todo si el ejecutable no está firmado y se acaba de compilar. No es un virus conocido: es la misma alerta que reciben otros programas de copias.

## Cómo arranca

El agente solo debe quedar en marcha como servicio de Windows, registrado con `agent.exe install` desde el instalador. El icono no usa PowerShell. El instalador escribe en `HKLM\Software\Microsoft\Windows\CurrentVersion\Run` la ruta directa:

`C:\Program Files\BackupAgent\BackupAgentTray.exe`

Esa carpeta es `{autopf}\BackupAgent`, el nombre interno del servicio y de `C:\ProgramData\BackupAgent`. No se usa `C:\Program Files\BackupAMG`.

El instalador también borra, si existiera, el valor `BackupAMG` de `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. Ese valor era un PowerShell oculto de las pruebas y no debe volver.

El panel se abre con `ShellExecute` (`open` y la dirección local). El agente y el icono no lanzan `cmd`, `powershell` ni `rundll32`. El aviso del icono sale del propio icono de la bandeja (`Shell_NotifyIcon`). El menú de la bandeja es una ventana propia dibujada con GDI, no un menú nativo ni un script.

`restic.exe` se ejecuta desde la misma carpeta que `agent.exe`, con sus argumentos normales y sin una ventana oculta añadida.

## Manifiesto y versión

`agent.exe` y `tray.exe` llevan nombre de producto BackupAMG, empresa, descripción, versión 0.1.0, icono y manifiesto. Los dos piden `asInvoker`.

`requireAdministrator` en `agent.exe` haría fallar el arranque del servicio: Windows lo inicia sin el diálogo de elevación y puede devolver el error 740. Los comandos que necesitan administrador (`install`, `start`, `stop`, `secure-data`) ya lo comprueban y lo dicen si falta.

No se empaqueta con UPX ni con ningún otro compresor.

Para regenerar el recurso de Windows, con Go en el PATH y desde cada carpeta `cmd\agent` y `cmd\tray`:

```
go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64
```

Eso reescribe `rsrc_windows_amd64.syso`. Hay que volver a compilar después.

## Dónde probar

En el equipo de desarrollo, la carpeta `D:\Proyectos\app\BackupAMG\bin` se excluye solo ahí, en Advanced Threat Defense, porque cada compilación genera un archivo nuevo. Esas excepciones no forman parte del programa ni se instalan en otros equipos.

La prueba con el antivirus tal como viene de fábrica se hace instalando `dist\BackupAgent-Setup.exe` en una máquina virtual. No vale compilar en `bin` y dar esa carpeta por instalación.

## Si un cliente lo necesita

Cuando Bitdefender, u otro antivirus, borre o bloquee la copia ya instalada, la exclusión recomendada es la carpeta del programa y la de los datos:

- `C:\Program Files\BackupAgent\`
- `C:\ProgramData\BackupAgent\`

En Bitdefender, Advanced Threat Defense solo admite archivos sueltos. En ese caso los archivos son `BackupAgentTray.exe`, `agent.exe` y `restic.exe` dentro de `C:\Program Files\BackupAgent\`.
