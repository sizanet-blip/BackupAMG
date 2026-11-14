# Especificación: Agente de copias de seguridad para Windows

> Documento de contexto para el asistente de programación (Cursor).
> Léelo completo antes de empezar y síguelo como referencia durante todo el desarrollo.

---

## 1. Objetivo

Desarrollar un **agente de copias de seguridad para Windows** que se instala como servicio, está activo desde que se enciende el equipo hasta que se apaga, hace copias automáticas de las carpetas configuradas (incluidos **ficheros abiertos**, mediante VSS) y genera un **informe detallado** de cada ejecución.

En esta fase **solo se desarrolla el agente**. Más adelante el agente enviará sus informes a una plataforma web (un módulo de una web existente en Yii2/PHP), pero eso **no forma parte de esta fase**. Aun así, el diseño debe dejarlo preparado: los informes deben contener desde el principio todos los datos que necesitará la web.

Prioridad absoluta: **que las copias se hagan correctamente, se puedan restaurar siempre y que cualquier fallo quede registrado**. La fiabilidad está por encima de cualquier otra cosa.

---

## 2. Tecnología

| Elemento | Elección |
|---|---|
| Lenguaje | **Go** (versión estable actual) |
| Motor de copia | **restic** (`restic.exe`, distribuido junto al agente). El agente lo ejecuta como proceso externo y lee su salida JSON (`--json`). No reimplementar el motor de copia. |
| Ficheros abiertos | VSS de Windows mediante la opción de restic `--use-fs-snapshot` (requiere privilegios de administrador / LocalSystem). |
| Servicio de Windows | Librería `github.com/kardianos/service` (o `golang.org/x/sys/windows/svc` si se justifica). |
| Configuración | Fichero **YAML**. |
| Contraseña del repositorio | Cifrada con **DPAPI** de Windows (ámbito máquina). Nunca en texto plano en disco ni en logs. |
| Logs | Logging estructurado (`log/slog`) con rotación de ficheros. |
| Instalador | **Inno Setup** (fase final). |

Convenciones de código: identificadores y nombres en inglés; comentarios y mensajes al usuario en español. Código idiomático de Go, con manejo explícito de errores (nunca ignorar un `error`).

---

## 3. Arquitectura

```
┌─────────────────────────────────────────────┐
│  AGENTE (Go) – servicio de Windows          │
│                                             │
│  config      → lee y valida config.yaml     │
│  scheduler   → decide cuándo toca copiar    │
│  runner      → ejecuta restic y lee JSON    │
│  retention   → aplica política (forget)     │
│  verify      → comprobación periódica       │
│  report      → genera informe de cada copia │
│  secrets     → DPAPI para la contraseña     │
│  service     → integración con Windows      │
│                                             │
│        ┌───────────────────────────┐        │
│        │ restic.exe (motor)        │        │
│        └───────────────────────────┘        │
└─────────────────────────────────────────────┘
          │ (fase futura)
          ▼
   Plataforma web Yii2 (NO en esta fase)
```

Estructura de proyecto propuesta:

```
backup-agent/
├── cmd/agent/main.go          # punto de entrada (consola y servicio)
├── internal/config/           # carga y validación de YAML
├── internal/scheduler/        # programación y copias atrasadas
├── internal/restic/           # wrapper de restic.exe + parseo JSON
├── internal/retention/        # política de retención
├── internal/verify/           # restic check periódico
├── internal/report/           # modelo y escritura de informes
├── internal/secrets/          # DPAPI
├── internal/service/          # servicio de Windows
├── internal/system/           # espacio en disco, evitar suspensión, info del equipo
├── bin/restic.exe             # binario de restic (no versionar si es grande)
├── configs/config.example.yaml
└── docs/
```

Rutas en el equipo:
- Configuración: `C:\ProgramData\BackupAgent\config.yaml`
- Informes: `C:\ProgramData\BackupAgent\reports\`
- Logs: `C:\ProgramData\BackupAgent\logs\`
- Estado interno (última copia, última verificación…): `C:\ProgramData\BackupAgent\state.json`

(El nombre `BackupAgent` es provisional y debe ser fácil de cambiar en un solo sitio.)

---

## 4. Configuración (ejemplo)

```yaml
agent:
  name: "PC-OFICINA-01"          # por defecto, nombre del equipo

backup:
  sources:
    - "C:\\Users\\Juan\\Documents"
    - "C:\\Users\\Juan\\Desktop"
  excludes:
    - "*.tmp"
    - "~$*"
    - "**\\node_modules"
    - "**\\AppData\\Local\\Temp"
  use_vss: true

repository:
  path: "E:\\Copias"             # USB, disco local o ruta UNC (\\\\nas\\copias)
  password_encrypted: "..."      # cifrada con DPAPI; nunca en claro

schedule:
  times: ["13:30", "20:00"]      # horas de copia diarias
  run_missed_on_startup: true    # si el equipo estaba apagado, copiar al arrancar
  max_retries: 2
  retry_delay_minutes: 15

retention:
  keep_daily: 7
  keep_weekly: 4
  keep_monthly: 12
  prune: true

verify:
  every_days: 7
  read_data_subset: "5%"         # restic check --read-data-subset
```

Requisitos:
- Validar la configuración al arrancar y dar errores claros en español.
- Comando de consola para establecer la contraseña: la pide por teclado, la cifra con DPAPI y la guarda.

---

## 5. Comportamiento del agente

### 5.1 Modos de ejecución
- `agent run` → ejecuta en consola (para pruebas), con logs por pantalla.
- `agent backup-now` → lanza una copia inmediata y termina.
- `agent install` / `agent uninstall` / `agent start` / `agent stop` → gestión del servicio.
- `agent set-password` → configura la contraseña del repositorio.
- `agent init-repo` → inicializa el repositorio de restic si no existe.
- `agent status` → muestra última copia, resultado y próxima copia.

### 5.2 Ciclo de una copia
1. Comprobar que el destino existe y es accesible (USB conectado, ruta de red disponible). Si no, registrar el error en el informe con un mensaje claro ("Destino no disponible: conecta el disco USB").
2. Comprobar espacio libre en el destino y registrarlo.
3. Evitar que el equipo entre en suspensión durante la copia (`SetThreadExecutionState`).
4. Ejecutar `restic backup` con `--json`, `--use-fs-snapshot` (si `use_vss`), las carpetas y exclusiones. Pasar la contraseña por variable de entorno al proceso hijo, nunca por argumento de línea de comandos.
5. Leer la salida JSON de restic (mensajes `status`, `error` y `summary`) y construir el informe.
6. Interpretar los códigos de salida de restic, en particular:
   - `0` → correcto.
   - `3` → copia hecha pero algunos ficheros no se pudieron leer → resultado **"con avisos"**, listar los ficheros afectados.
   - Otros → **fallido**, guardar el mensaje de error.
7. Si falla, reintentar según `max_retries`.
8. Aplicar la retención (`restic forget` con la política y `--prune` si está activado).
9. Guardar el informe y actualizar `state.json`.

### 5.3 Programación
- Copias a las horas configuradas.
- Si a la hora prevista el equipo estaba apagado o suspendido, hacer la copia al arrancar o reanudar (`run_missed_on_startup`).
- Nunca lanzar dos copias a la vez sobre el mismo repositorio.
- Si restic deja un bloqueo antiguo (por ejemplo, tras un apagado brusco), detectarlo y gestionarlo de forma segura (`restic unlock` solo para bloqueos obsoletos).

### 5.4 Verificación periódica
- Cada `verify.every_days`, ejecutar `restic check` (con `--read-data-subset` según configuración) y generar un informe de tipo "verificación".

### 5.5 Apagado
- Al recibir la orden de parada del servicio (apagado del equipo), registrar un evento "apagado normal". Si hay una copia en curso, intentar detenerla de forma limpia.

---

## 6. Informe de cada ejecución

Un fichero JSON por ejecución en `reports\`, con nombre basado en fecha/hora. Este formato será el que se envíe más adelante a la web, así que debe ser estable y versionado.

```json
{
  "schema_version": 1,
  "type": "backup",
  "agent_version": "0.1.0",
  "restic_version": "x.y.z",
  "hostname": "PC-OFICINA-01",
  "started_at": "2026-11-13T13:30:00+01:00",
  "finished_at": "2026-11-13T13:34:12+01:00",
  "duration_seconds": 252,
  "status": "success | warning | failed",
  "trigger": "scheduled | missed_on_startup | manual | retry",
  "attempt": 1,
  "sources": ["C:\\Users\\Juan\\Documents"],
  "repository": "E:\\Copias",
  "vss_used": true,
  "files_new": 12,
  "files_changed": 30,
  "files_unmodified": 15400,
  "data_added_bytes": 52428800,
  "total_files_processed": 15442,
  "total_bytes_processed": 8589934592,
  "snapshot_id": "abc123...",
  "destination_free_bytes": 107374182400,
  "skipped_files": [
    { "path": "C:\\...\\archivo.pst", "reason": "..." }
  ],
  "errors": [],
  "retention": { "applied": true, "snapshots_removed": 2 },
  "message": "Copia completada correctamente"
}
```

Además, mantener un log de texto legible con los eventos del agente (arranque, parada, copias, errores).

---

## 7. Casos que deben funcionar (pruebas obligatorias)

- Copia con ficheros abiertos: Word, Excel y Outlook (`.pst`) abiertos durante la copia.
- Disco USB desconectado a la hora de la copia → informe "fallido" con mensaje claro y reintentos.
- Destino lleno.
- Apagado brusco en mitad de una copia → la siguiente copia funciona correctamente.
- Equipo apagado a la hora prevista → la copia se hace al encender.
- Rutas largas (más de 260 caracteres), nombres con tildes, eñes y espacios.
- Ficheros sin permisos de lectura → aparecen en `skipped_files`.
- Cientos de miles de ficheros y ficheros muy grandes.
- **Restauración real** de ficheros y carpetas, comprobando que se abren bien.
- Destino en ruta de red (UNC). Ojo: el servicio se ejecuta como LocalSystem y **no ve unidades de red mapeadas** (Z:, etc.); usar rutas UNC y documentar cómo dar acceso.

Incluir tests unitarios para: carga y validación de configuración, parseo de la salida JSON de restic (con ejemplos reales guardados como fixtures), interpretación de códigos de salida, lógica del programador (incluidas copias atrasadas) y construcción del informe.

---

## 8. Fases de desarrollo

Avanzar **fase por fase**. No pasar a la siguiente sin que la anterior funcione y esté probada.

1. **Esqueleto del proyecto**: estructura de carpetas, `go.mod`, logging, comando `agent run`.
2. **Configuración**: carga, validación y `config.example.yaml`.
3. **Secretos**: DPAPI y comando `set-password`.
4. **Wrapper de restic**: `init-repo` y `backup-now` con VSS, lectura del JSON y códigos de salida.
5. **Informe**: generación del JSON y log legible.
6. **Retención y verificación**.
7. **Programador**: horas, copias atrasadas, reintentos, bloqueo de concurrencia, evitar suspensión.
8. **Servicio de Windows**: install/uninstall/start/stop, evento de apagado.
9. **Pruebas** de la sección 7.
10. **Instalador** con Inno Setup (incluye `restic.exe`, crea carpetas en ProgramData, registra el servicio).

---

## 9. Fuera de alcance en esta fase

- Plataforma web, API, envío de informes, latidos, alertas por email.
- Interfaz gráfica o icono en la bandeja del sistema.
- Soporte para macOS y Linux (pero evitar decisiones que lo impidan: aislar lo específico de Windows en `internal/system`, `internal/secrets` e `internal/service`).
- Motor de copia propio.
- Copia de imagen completa del sistema.

---

## 10. Cómo debe trabajar el asistente

- Antes de cada fase, explica brevemente qué vas a hacer y qué ficheros vas a crear o modificar.
- Escribe código completo y funcional, no fragmentos con "TODO" sin resolver.
- Si una decisión no está cubierta por este documento y es importante (formato de datos, dependencia nueva, cambio de arquitectura), **pregunta antes** de decidir.
- No añadas dependencias innecesarias.
- Al terminar cada fase, indica cómo probarla manualmente en Windows.
- Recuerda que restic se distribuye bajo licencia BSD: incluir su aviso de copyright y licencia en `docs/THIRD_PARTY_LICENSES.md`.
