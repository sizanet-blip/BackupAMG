# BackupAMG

Programa de copias de seguridad para Windows (más adelante también macOS y Linux) pensado para que un técnico informático lo instale en los equipos de sus clientes y, en una fase futura, los controle desde una plataforma web central.

Idioma: hablar con el usuario en **español**, en lenguaje claro. El usuario no es programador de Go; explica las decisiones sin jerga innecesaria.

## Estado actual

- Agente versión **0.1.0**, implementado según `docs/especificaciones/ESPECIFICACION_AGENTE.md` y `docs/especificaciones/ESPECIFICACION_INTERFAZ.md`.
- En curso: **rediseño visual** del panel y del icono de bandeja según `docs/especificaciones/ESPECIFICACION_DISENO.md`.
- Rama de trabajo actual: `feature/...` (consultar `git branch`). No hacer commit ni push salvo que el usuario lo pida.
- Pendiente: pruebas reales (ver más abajo), uso piloto y, después, la plataforma web.

## Arquitectura

```
tray.exe (sesión del usuario) ──named pipe──► agent.exe (servicio Windows, LocalSystem)
                                                 ├─ panel web local 127.0.0.1 (HTML/CSS/JS embebido)
navegador ◄──── token de un solo uso ───────────┤
                                                 └─ restic.exe (motor de copia, proceso externo, --json)
```

- **Lenguaje:** Go. Módulo `backup-agent`.
- **Motor:** `restic.exe` 0.19.1 junto a `agent.exe`. No reimplementar el motor. Licencia BSD en `docs/THIRD_PARTY_LICENSES.md`.
- **Ficheros abiertos:** VSS con `--use-fs-snapshot`.
- **Configuración:** YAML en `C:\ProgramData\BackupAgent\config.yaml` (ejemplo en `configs/config.example.yaml`). Guardado atómico y recarga sin reiniciar.
- **Contraseña del repositorio:** DPAPI de máquina, nunca en claro, ni en logs ni en argumentos (se pasa a restic por `RESTIC_PASSWORD`). Si se pierde, las copias son irrecuperables.
- **Datos:** `C:\ProgramData\BackupAgent\` (config, `reports\` JSON con `schema_version: 1`, logs, `state.json`), con permisos solo para SYSTEM y Administradores. Retención de informes y logs ~90 días.
- **Destino USB:** identificado por `repository.volume_id`, no solo por la letra. Destinos de red solo por ruta UNC (el servicio no ve unidades mapeadas).
- **Panel:** solo en `127.0.0.1`, token de un solo uso → cookie de sesión, validación de Host/Origin y CSRF. Rol administrador (todo) y usuario normal (ver estado e historial y "Copiar ahora").
- **Instalador:** Inno Setup, `installer/BackupAgent.iss` → `dist/BackupAgent-Setup.exe`. Instala en `C:\Program Files\BackupAgent\`, registra el servicio (no lo arranca) y el icono en `HKLM\...\Run`.
- **Nombres:** la marca visible es **BackupAMG**; el nombre interno del servicio, carpetas e instalador es `BackupAgent` (definido en `internal/app`).

### Carpetas del código

- `cmd/agent`, `cmd/tray`: ejecutables (con recurso de Windows `rsrc_windows_amd64.syso`, generado con go-winres).
- `internal/`: `config`, `scheduler`, `restic`, `restore`, `verify`, `report`, `secrets`, `service`, `state`, `status`, `system`, `panel` (con `panel/web` embebido), `diagnose`, `housekeeping`, `logging`, `paths`, `app`.
- `docs/`: especificaciones, `ANTIVIRUS.md`, licencias.

## Reglas que hay que respetar siempre

1. **La fiabilidad de las copias va primero.** Una copia correcta no se marca como fallida por un fallo posterior (retención, limpieza). Cancelar no debe tumbar el agente. Una configuración inválida nunca sustituye a la que está en uso.
2. **No romper** los comandos de consola existentes ni el formato de los informes. Si hay que ampliarlo, de forma compatible.
3. **Antivirus** (ver `docs/ANTIVIRUS.md`): nada de PowerShell, `cmd` ni `rundll32` lanzados por el programa; nada de arranques ocultos en el registro del usuario; el panel se abre con `ShellExecute`; ejecutables con información de versión, icono y manifiesto `asInvoker`; **nunca** UPX ni empaquetadores. El programa no está firmado (no hay presupuesto para certificado por ahora).
4. **Probar antes de decir que algo está hecho** (también está en `.cursor/rules/probar-antes-de-decir-hecho.mdc`): compilar no basta. Reinicia el binario afectado y comprueba el síntoma concreto. Si no puedes comprobarlo, dilo claramente y explica cómo probarlo.
5. En el icono de bandeja, el hilo de interfaz no hace I/O del pipe ni llamadas Win32 de foco desde goroutines.
6. Nada se carga desde internet en el panel: fuentes, iconos y librerías van embebidos.
7. Textos de la interfaz centralizados en `internal/panel/web/textos.js`, en español natural y sin términos técnicos (snapshot, repositorio, prune, restic) salvo en el detalle técnico y en Ajustes.
8. Si una decisión importante no está cubierta (dependencia nueva, cambio de formato o de arquitectura), **preguntar antes**.

## Desarrollo y pruebas

- Compilar en `bin\` (`agent.exe`, `tray.exe`, `restic.exe`). Esa carpeta está excluida en Bitdefender **solo en el PC de desarrollo**.
- Las pruebas como las verá un cliente se hacen **instalando `dist\BackupAgent-Setup.exe` en una máquina virtual** con el antivirus por defecto, no desde `bin\`.
- Variable `BACKUP_AGENT_DATA_DIR` solo para desarrollo (el servicio no la lee).

### Pruebas reales pendientes

- Word, Excel y Outlook abiertos con VSS activo → restaurar y **abrir** los ficheros (sobre todo el `.pst`).
- USB desconectado físicamente a la hora de la copia, y reconexión con otra letra.
- Apagado brusco durante una copia.
- Reinicio: el servicio arranca solo y copia a su hora; el icono aparece al iniciar sesión.
- Usuario sin permisos de administrador: no puede configurar ni restaurar.
- Destino en ruta UNC.
- Instalación y desinstalación con el instalador.

## Hoja de ruta

1. Rediseño visual (`ESPECIFICACION_DISENO.md`) e integración del logotipo.
2. Pruebas reales y uso piloto unas semanas.
3. Firma de código cuando haya ingresos (Azure Artifact Signing si hay empresa; Certum estándar si no).
4. **Plataforma web central** como módulo de la web Yii2 existente del usuario: API v1 (alta de equipo, latido, informes, descarga de configuración), panel con semáforo por cliente, alertas por fallo y por silencio (email, Telegram), configuración remota, roles y portal de cliente. El agente envía estado e informes, nunca los ficheros.
5. Después: destinos en la nube (S3/Backblaze), actualización automática, macOS y Linux.

## Marca

Logotipo con el estilo de SaludAMG: escudo azul `#19ABFF` con flecha circular y check blancos, punto naranja `#E06903`; texto "Backup" gris oscuro `#333D4B` + "AMG" azul en cursiva. Iconos de bandeja por estado: verde (al día), ámbar (avisos), rojo (fallo), azul (copiando), gris (servicio parado). Paleta y componentes del panel en `ESPECIFICACION_DISENO.md`.
