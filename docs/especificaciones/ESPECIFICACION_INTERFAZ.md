# Especificación: Interfaz visual del agente de copias (Windows)

> Documento de contexto para el asistente de programación (Cursor).
> Continúa el trabajo de `ESPECIFICACION_AGENTE.md`, que ya está implementado (versión 0.1.0).
> Léelo completo antes de empezar y respeta todo lo ya construido: no rehagas lo que funciona.

---

## 1. Objetivo

Añadir una **interfaz visual** al agente para que un técnico o el propio usuario puedan, sin usar la consola:

- Configurar todo: qué copiar, destino, contraseña, horarios, retención y verificación.
- Ver el estado de las copias de un vistazo.
- Lanzar una copia o una verificación al momento y ver su progreso.
- Consultar el historial y el detalle de cada copia.
- **Restaurar** ficheros y carpetas eligiendo la fecha.

Los comandos de consola actuales deben **seguir funcionando** igual.

---

## 2. Enfoque elegido

Un servicio de Windows no puede mostrar ventanas, por eso la interfaz se divide en dos piezas:

```
┌────────────────────────────┐
│ Icono de bandeja (tray.exe)│  sesión del usuario, arranca al iniciar sesión
│ estado por color, menú,    │
│ notificaciones             │
└──────────────┬─────────────┘
               │ abre el navegador con un token de un solo uso
               ▼
┌────────────────────────────┐        ┌──────────────────────────────┐
│ Panel web local            │ ◄────► │ Servicio BackupAgent (Go)     │
│ http://127.0.0.1:PUERTO    │  API   │ ya existente + servidor HTTP  │
│ HTML/CSS/JS embebido       │  JSON  │ local + API + eventos (SSE)   │
└────────────────────────────┘        └──────────────────────────────┘
```

1. **Panel web local servido por el propio servicio** (`agent.exe`), escuchando **solo en `127.0.0.1`**. Los ficheros de la interfaz van **embebidos en el ejecutable** (`embed`), sin depender de internet ni de CDNs.
2. **Icono en la bandeja del sistema** (`tray.exe`), un pequeño programa en Go que se ejecuta en la sesión del usuario, muestra el estado y abre el panel.

Motivo: todo sigue en Go, funciona aunque el panel esté cerrado, y la API local servirá más adelante como base para conectar con la plataforma web central (Yii2).

---

## 3. Tecnología

| Elemento | Elección |
|---|---|
| Servidor HTTP | `net/http` de la librería estándar de Go, dentro del servicio existente. |
| Frontend | HTML + CSS + JavaScript ligero, **sin build complejo**. Se permite una librería pequeña (por ejemplo Alpine.js o Preact) **incluida localmente** en el proyecto, nunca desde CDN. |
| Tiempo real | **Server-Sent Events (SSE)** para progreso de copias y cambios de estado. |
| Icono de bandeja | Librería de systray para Go (por ejemplo `fyne.io/systray`). |
| Comunicación bandeja ↔ servicio | **Named pipe** de Windows (para pedir tokens y estado de forma segura). |
| Idioma de la interfaz | Español. Textos centralizados en un solo fichero para poder traducir más adelante. |
| Diseño | Limpio, sobrio y profesional. Responsive. Modo claro y oscuro según el sistema. Sin dependencias externas de fuentes. |

---

## 4. Seguridad (obligatorio)

El panel permite cambiar la configuración y restaurar ficheros, así que debe estar bien protegido aunque sea local:

- El servidor escucha **solo en `127.0.0.1`**, nunca en `0.0.0.0`. Puerto por defecto configurable (por ejemplo `47800`); si está ocupado, error claro en el log.
- **Acceso solo con token**: `tray.exe` pide al servicio, por named pipe, un **token de un solo uso** de corta duración (por ejemplo 60 segundos) y abre el navegador con él. El servicio lo canjea por una cookie de sesión (`HttpOnly`, `SameSite=Strict`) con caducidad.
- El servicio identifica **quién pide el token** a través del named pipe (identidad del cliente del pipe) y asigna un **rol**:
  - **Administrador** (miembro del grupo Administradores): acceso completo.
  - **Usuario normal**: solo ver estado e historial y lanzar "Copiar ahora". No puede cambiar configuración, ni restaurar, ni ver la contraseña.
- Protección contra ataques desde otras webs: validar la cabecera `Host` (solo `127.0.0.1:PUERTO` / `localhost:PUERTO`), validar `Origin` en peticiones que modifican datos y usar token CSRF.
- La contraseña del repositorio **nunca** se devuelve por la API. Solo se puede establecer o cambiar.
- Toda acción que modifica algo (cambiar configuración, restaurar, iniciar copia) queda registrada en el log con fecha, usuario de Windows y acción.

---

## 5. Pantallas del panel

### 5.1 Asistente de primera configuración
Se muestra automáticamente si no hay configuración válida. Pasos:
1. **Destino**: elegir disco (USB/local) o carpeta de red (UNC) y probarlo.
2. **Contraseña** del repositorio: escribirla dos veces, aviso destacado de que si se pierde las copias son irrecuperables, y casilla obligatoria "He guardado la contraseña en un lugar seguro". Ofrecer **descargar o imprimir una hoja de recuperación** (nombre del equipo, destino, fecha y espacio para anotar la contraseña a mano; la contraseña no se imprime).
3. **Qué copiar**: carpetas.
4. **Cuándo**: horario.
5. **Resumen** y botón "Inicializar y hacer la primera copia".

### 5.2 Inicio (estado)
- Indicador grande con semáforo:
  - Verde: última copia correcta y reciente.
  - Amarillo: última copia con avisos, o retrasada.
  - Rojo: última copia fallida, o sin copia correcta en más de N días (configurable, por defecto 2).
  - Azul: copia en curso.
- Última copia (fecha, resultado, duración, datos añadidos) y próxima copia programada.
- Estado del destino: conectado o no, espacio libre y ocupado por las copias.
- Última verificación y su resultado.
- Botones: **Copiar ahora**, **Verificar ahora**, **Cancelar** (si hay copia en curso).
- Progreso en tiempo real durante la copia (porcentaje, ficheros, velocidad, tiempo restante, fichero actual), a partir de los mensajes `status` de restic.

### 5.3 Qué copiar
- **Accesos rápidos** para Documentos, Escritorio, Imágenes, Descargas, Vídeos y Música de cada usuario del equipo.
- **Explorador de carpetas** del equipo con casillas para marcar o desmarcar (el servicio lista directorios mediante la API; carga perezosa por niveles).
- **Exclusiones**: plantillas predefinidas activables (temporales de Office `~$*`, `*.tmp`, cachés de navegadores, `node_modules`, papelera, carpetas de OneDrive no descargadas…) y lista personalizada.
- Botón **"Calcular tamaño"** que estima cuánto ocupa la selección.

### 5.4 Destino
- Tipo: **disco local o USB**, o **carpeta de red (UNC)**.
- Para USB: identificar el disco por su **identificador de volumen** (GUID/serie), no solo por la letra. Si Windows le asigna otra letra al reconectarlo, el agente debe encontrarlo igualmente. Añadir a la configuración un campo opcional `repository.volume_id` y resolver la letra actual antes de cada copia.
- Para red: explicar que el servicio no ve unidades mapeadas (`Z:`) y que hay que usar ruta UNC con permisos para la cuenta del equipo.
- Botón **"Probar destino"**: accesible, escribible, espacio libre, y si ya existe repositorio, que la contraseña sea correcta.
- **Cambiar contraseña** del repositorio (con `restic key`), con los mismos avisos que el asistente.

### 5.5 Programación
- Horas de copia (añadir y quitar varias) y **días de la semana**.
- Copiar al arrancar si se perdió una copia (sí/no).
- Reintentos y espera entre reintentos.
- Días sin copia correcta para considerar estado rojo.

### 5.6 Retención
- Ajustes predefinidos fáciles de entender:
  - **Básica**: 7 diarias.
  - **Recomendada**: 7 diarias, 4 semanales, 12 mensuales.
  - **Larga**: 14 diarias, 8 semanales, 24 mensuales, 5 anuales.
- Modo **avanzado** con todos los valores editables.
- Explicación en una frase de lo que significa la opción elegida.

### 5.7 Verificación
- Cada cuántos días y qué porcentaje de datos se comprueba.

### 5.8 Historial
- Lista de ejecuciones (copias y verificaciones) a partir de los informes JSON: fecha, tipo, resultado, duración, datos añadidos.
- Filtros por resultado y por fechas.
- **Detalle** de cada ejecución: todos los datos del informe, ficheros omitidos con su motivo y errores. Botón para copiar el detalle al portapapeles (útil para soporte).

### 5.9 Restaurar (solo administradores)
1. Elegir la **copia por fecha** (lista de snapshots de restic).
2. **Explorar** su contenido como un árbol de carpetas (`restic ls --json`), con **buscador** de ficheros por nombre (`restic find --json`).
3. Marcar ficheros o carpetas.
4. Elegir destino de la restauración:
   - **Otra carpeta** (opción por defecto, por ejemplo `Escritorio\Restaurado_FECHA`).
   - **Ubicación original**: con aviso claro; si el fichero existe, **no sobrescribir** por defecto (renombrar el restaurado), y opción explícita para sobrescribir.
5. Progreso y resultado final (ficheros restaurados, errores).

### 5.10 Ajustes y diagnóstico
- Versión del agente y de restic, ruta de datos, puerto del panel.
- Visor de las últimas líneas del log, con filtro por nivel.
- Botón **"Generar paquete de diagnóstico"**: ZIP con configuración (**sin contraseña**), últimos informes y logs, para enviar a soporte.

---

## 6. Icono de bandeja (`tray.exe`)

- Se instala junto al agente y arranca al iniciar sesión de cualquier usuario (registro `Run` de la máquina).
- Si el servicio no está en marcha, icono gris y menú con aviso.
- **Color del icono** según el semáforo (verde, amarillo, rojo, azul en curso, gris sin conexión con el servicio).
- Texto al pasar el ratón: estado y última copia.
- **Menú**:
  - Estado (texto, no clicable).
  - Abrir panel.
  - Copiar ahora.
  - Ver historial.
  - Salir del icono (solo cierra el icono; **no** detiene el servicio ni las copias).
- **Notificaciones de Windows**: copia fallida (siempre), copia con avisos (configurable), copia correcta (desactivado por defecto), destino no conectado a la hora de la copia.
- Comunicación con el servicio solo por **named pipe** con permisos adecuados (no por HTTP sin token).

---

## 7. Cambios necesarios en el servicio

- Servidor HTTP local y API JSON versionada (`/api/v1/...`) para todo lo que necesita el panel.
- Canal **SSE** (`/api/v1/events`) para progreso y cambios de estado.
- **Recarga de configuración sin reiniciar** el servicio al guardar desde el panel.
- Escritura de la configuración **atómica** (escribir en fichero temporal y renombrar) y **validación** antes de guardar; si no es válida, no se guarda y se devuelven los errores por campo.
- **Cancelar** una copia en curso de forma limpia.
- Endpoint de estado resumido (el mismo que usa la bandeja).
- Resolver el destino USB por `volume_id` (ver 5.4).
- Named pipe para la bandeja: estado, "copiar ahora" y emisión de tokens del panel.
- El instalador debe incluir `tray.exe`, registrarlo para el inicio de sesión y quitarlo al desinstalar.

---

## 8. Fases de desarrollo

Avanzar **fase por fase**, comprobando cada una antes de seguir.

1. **Servidor HTTP local + seguridad**: escucha en 127.0.0.1, tokens, sesión, roles, validación de Host/Origin, CSRF.
2. **API de estado y SSE** + pantalla **Inicio** con progreso en tiempo real y Copiar/Verificar/Cancelar.
3. **Icono de bandeja** con named pipe, colores, menú y notificaciones.
4. **Configuración**: pantallas Qué copiar, Programación, Retención y Verificación, con guardado atómico y recarga sin reinicio.
5. **Destino**: USB por `volume_id`, UNC, probar destino, cambiar contraseña.
6. **Asistente de primera configuración** y hoja de recuperación.
7. **Historial** y detalle de ejecuciones.
8. **Restaurar**: snapshots, explorador, buscador, restauración segura.
9. **Ajustes y diagnóstico**.
10. **Instalador** actualizado y pruebas completas.

---

## 9. Pruebas obligatorias

De la interfaz:
- El panel **no** es accesible desde otro equipo de la red.
- Sin token válido no se puede acceder; un token usado o caducado no sirve.
- Un usuario normal no puede cambiar configuración ni restaurar (comprobar también llamando a la API directamente).
- Guardar una configuración inválida no estropea la que está en uso.
- Cerrar el navegador o el icono durante una copia no la interrumpe.
- Progreso en tiempo real correcto en una copia grande.
- Restaurar a otra carpeta y a la ubicación original sin sobrescribir.
- USB reconectado con otra letra: el agente lo encuentra por `volume_id`.

Pendientes del agente, a realizar ahora con la interfaz:
- Copia con **Word, Excel y Outlook abiertos**, restauración y apertura de los ficheros (sobre todo el `.pst`).
- **USB desconectado físicamente** a la hora de copia y reconexión.
- **Apagado brusco** del equipo durante una copia.
- **Reinicio** del equipo: el servicio arranca solo y copia a su hora; el icono aparece al iniciar sesión.
- Destino en **ruta UNC**.

---

## 10. Fuera de alcance en esta fase

- Plataforma web central (Yii2), envío de informes, latidos y alertas por email.
- Copias a la nube.
- macOS y Linux (evitar decisiones que lo impidan: lo específico de Windows, como named pipes, DPAPI y systray, debe quedar aislado).
- Multiidioma (solo dejar los textos centralizados).

---

## 11. Cómo debe trabajar el asistente

- Antes de cada fase, explica brevemente qué vas a hacer y qué ficheros vas a crear o modificar.
- No rompas los comandos de consola existentes ni el formato de los informes (`schema_version: 1`); si hay que ampliarlo, hazlo de forma compatible.
- Si una decisión importante no está cubierta aquí (nueva dependencia, cambio de formato, cambio de arquitectura), **pregunta antes**.
- Al terminar cada fase, indica cómo probarla manualmente en Windows y qué has probado tú realmente y qué no.
- Prioriza que la interfaz sea **clara para alguien no técnico**: mensajes en lenguaje sencillo, sin términos de restic a la vista del usuario salvo en el detalle técnico.
