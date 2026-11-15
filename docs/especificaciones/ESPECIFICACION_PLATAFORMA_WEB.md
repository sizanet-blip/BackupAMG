# Especificación: Plataforma web central de BackupAMG (módulo del panel Yii2 AMG)

> Documento para el asistente de programación. Cubre **dos proyectos**: el agente (este repositorio, Go) y el panel web existente en `D:\Proyectos\yii2\amg` (Yii2 Advanced, PHP 8.x).
> Continúa `ESPECIFICACION_AGENTE.md`, `ESPECIFICACION_INTERFAZ.md`, `ESPECIFICACION_DISENO.md` y `ESPECIFICACION_MULTIDESTINO.md`.
> **Estado: borrador de diseño (hoja de ruta, punto 4).** No empezar sin confirmación del usuario. Redactado a partir de una lectura del panel Yii2 (migraciones, `AppAuthService`, módulo `Licencias`, notificadores). La **sección 16** recoge la verificación posterior contra el código real (permisos, licencias, entorno) y **prevalece** sobre el resto donde haya diferencias.
> **Objetivo:** que el técnico (usuario) vea de un vistazo si las copias de todos sus clientes se hacen bien, sepa por qué fallan y se entere solo cuando algo va mal, y que cada empresa pueda ver el estado de sus equipos.

---

## 1. Principios

1. **El agente nunca depende de la web.** Si la web no responde, la licencia caduca o se supera el límite de equipos, **las copias siguen haciéndose**. Solo se deja de informar y se muestra un aviso claro. Una licencia nunca apaga una copia.
2. **El agente envía estado e informes, nunca los ficheros.** Solo conexiones salientes desde el equipo (HTTPS). La web **no** manda órdenes al agente en las primeras fases.
3. **Lo más importante es avisar de lo que NO llega**: una copia que falla y un equipo que se queda en silencio.
4. **Reutilizar lo que ya existe** en el panel (cuentas, apps, licencias, avisos), sin inventar un sistema paralelo.
5. **Datos mínimos** (RGPD): enviar solo lo necesario, con retención definida.
6. **Lenguaje sencillo** para el cliente; los términos técnicos solo en el detalle del técnico.

---

## 2. Modelo de negocio y de datos

**Una empresa = una cuenta con un usuario principal**, como en las demás apps AMG. Bajo la empresa están sus equipos. El límite de equipos lo marca la licencia (1, 10…).

```
Empresa (cuenta app_account, con acceso a la app «BackupAMG»)
  ├─ Licencia (plan, máximo de equipos, fecha de fin)
  └─ Equipos (uno por ordenador)
        ├─ Latidos (estado actual)
        ├─ Ejecuciones (cada copia / verificación / copia adicional)
        │     └─ Incidencias (ficheros omitidos, errores)
        └─ Destinos (principal y copias adicionales) y su estado
```

### Reutilización de lo existente en el panel
| Necesidad | Ya existe | Uso |
|---|---|---|
| Registrar la app | `amg_app` (SaludAMG, TiroAMG, MotoAMG, Simulador112) | Añadir `BackupAMG` |
| Usuario de la empresa | `app_account` + `app_account_app` | Cuenta principal con acceso habilitado a BackupAMG |
| Licencia y límite de equipos | `license` (`max_devices`, `fecha_fin`, `grace_days_offline`) y `license_device` | Licencia de la empresa; cada equipo ocupa un dispositivo |
| API de activación/validación | módulo `Licencias` → `licencias/api` (`activate`, `validate`, límite de peticiones con `RequestRateLimit`) | Patrón a seguir para el alta de equipos |
| Sesión/token | `app_session` (`token_hash`, `device_id`, `app_version`) | Patrón para la clave de cada equipo (guardar solo el hash) |
| Avisos | `TelegramNotifier`, `SystemTelegramNotifier`, `BrandedMail`, cola `tenant_job_queue` | Alertas por Telegram y correo |
| Autenticación de administradores | usuarios del panel, 2FA (`Enable2faForm`) | Acceso del técnico a todas las empresas |

> Decidir, tras leer `AccessCatalog`/`user_access`, cómo se enlaza la empresa con permisos (rol «cliente» de solo lectura) sin tocar lo que ya funciona en otras apps.

### Entidades nuevas (nombres orientativos, prefijo `bk_`)
- **`bk_company`** (ficha de la empresa): `app_account_id`, nombre comercial, contacto, correo de avisos, zona horaria, notas, activa.
- **`bk_machine`**: `company_id`, `license_device_id` (o equivalente), `machine_uid` (identificador estable del PC), `hostname`, SO, `agent_version`, `restic_version`, `enrolled_at`, `last_seen_at`, `status` (activo/revocado), `token_hash`, destino principal (resumen), `next_backup_at`, `stale_after_days`.
- **`bk_run`** (una por informe recibido): `machine_id`, `report_uid` (idempotencia), `type` (backup, verify, replica), `status` (success, warning, failed), `trigger`, `started_at`, `finished_at`, `duration_seconds`, cifras (`files_new`, `files_changed`, `data_added_bytes`, `total_bytes_processed`…), `destination_free_bytes`, `snapshot_id`, `message`, `cause_code` (causa normalizada, ver §6), `schema_version`, y `payload_json` (informe original comprimido, con retención propia).
- **`bk_run_issue`**: `run_id`, `kind` (skipped_file, error), `path_hash` o ruta recortada según política de privacidad (§9), `reason`.
- **`bk_destination_status`**: `machine_id`, `destination_id` (principal o id de copia adicional), nombre, tipo, estado (al día, pendiente, sin conexión, error), última réplica correcta, espacio usado.
- **`bk_alert`**: `company_id`, `machine_id`, `type`, `severity`, `opened_at`, `closed_at`, `last_notified_at`, `dedupe_key`, `acknowledged_by`.
- **`bk_enrollment_log`**: altas, revocaciones y rechazos por límite (auditoría).

Migraciones de Yii2 con `safeUp()`/`safeDown()`, InnoDB, claves foráneas con `CASCADE` donde corresponda, índices por `machine_id` + `started_at`.

---

## 3. Alta de equipos (enrolamiento) y licencias

**Flujo normal (lo hace la empresa al instalar):**
1. El administrador crea la empresa y su licencia en el panel (plan, número de equipos, fecha de fin). Se genera la **clave de licencia** de la empresa.
2. Al instalar el agente, en el asistente (o con un comando de consola) se escribe la clave de licencia de la empresa.
3. El agente llama a la web (`enroll`) con: clave de licencia, `machine_uid`, nombre del equipo, versiones y SO.
4. La web comprueba licencia vigente y plazas libres:
   - Hay plaza: crea el equipo, ocupa un dispositivo y devuelve una **clave propia del equipo**. El agente la guarda cifrada (DPAPI de máquina), nunca en claro.
   - No hay plaza o la licencia caducó: respuesta clara («límite de equipos alcanzado», «licencia caducada»). El agente **sigue copiando**.
5. Cada alta nueva envía un correo a la empresa y al técnico.

**Reglas:**
- **Reinstalar en el mismo PC no gasta plaza:** el `machine_uid` identifica el ordenador (valor estable derivado de un identificador de máquina de Windows, con hash; nunca datos personales). Si ya existe, se reactiva y se emite clave nueva.
- **Liberar plaza:** el usuario principal (o el técnico) quita un equipo del panel. Un equipo sin señales durante X días se marca «Sin señales» para que lo revisen, sin liberarse solo.
- **Revocar:** el usuario principal o el técnico pueden revocar un equipo; el agente recibe «revocado» en su siguiente conexión y deja de enviar (sigue copiando).
- **Clave de licencia comprometida:** se puede **regenerar** sin tocar los equipos ya dados de alta (estos usan su clave propia).
- **Licencia caducada o límite superado:** el agente muestra un aviso en el panel local y en la bandeja, **nunca se detiene**. Respetar `grace_days_offline` como margen sin conexión.
- Límite de peticiones por IP en `enroll` (como ya hace `licencias/api`) y registro de intentos fallidos.

---

## 4. API agente → web (v1)

Base: `https://<panel>/backup/v1/...` (ruta propia, estilo de las existentes `licenses/*`/`installations/*`). Solo POST con JSON, solo HTTPS. Respuestas con `ok`, `code` (máquina) y `message` (humano). Sin CSRF (clientes no navegador), con límite de peticiones.

| Endpoint | Función |
|---|---|
| `POST enroll` | Alta de equipo con la clave de licencia (§3). Devuelve la clave del equipo. |
| `POST heartbeat` | Latido: estado actual (`healthy/warning/failed/busy/stopped`), versiones, espacio libre del destino, próxima copia, número de copias adicionales y su estado, aviso de «verificación toca». Cada 5–15 min y tras cada copia. |
| `POST reports` | Envío de uno o varios informes (§5). **Idempotente** por `report_uid`. |
| `POST license/status` | Estado de la licencia del equipo (vigente, caducada, límite, revocado). Puede ir dentro del latido. |
| `GET/POST config` | (Fase 4) descarga de configuración remota. |

**Autenticación:** cabecera `Authorization: Bearer <clave del equipo>`; la web guarda solo el hash (como `app_session.token_hash`). 401 si está revocada. La hora del servidor manda; el agente envía su hora para detectar desfases de reloj.

**Respuesta del latido:** incluye lo que el agente debe saber: licencia, «revocado», versión mínima recomendada y, en el futuro, cambios de configuración pendientes.

### Cambios en el agente (Go)
- Módulo `internal/cloud` (o similar): cliente HTTPS con tiempos límite, reintentos con espera creciente y **cola local persistente** (carpeta de datos) de informes pendientes, sin límite de antigüedad razonable (ver retención).
- Estado «Conectado a la plataforma / Sin conexión / Licencia caducada / Límite de equipos» visible en Ajustes y en la bandeja, con texto sencillo.
- Clave del equipo cifrada con DPAPI, igual que la contraseña del repositorio. Nunca en logs ni en informes.
- Envío en un hilo/actividad aparte: **un fallo de red nunca afecta a la copia** (misma regla que las copias adicionales).
- Validar el certificado TLS normalmente; opción de fijar el dominio esperado. Sin dependencias nuevas sin preguntar.
- Antivirus: solo conexiones HTTPS salientes del agente; nada de scripts externos (`docs\ANTIVIRUS.md`).

---

## 5. Informes que se envían

Los informes ya existen (`schema_version: 1`, `type: backup | verify | replica`). Se envían **tal cual**, más `report_uid` (identificador único, estable por ejecución). Reglas:
- Un informe enviado correctamente se marca como enviado; el reenvío con el mismo `report_uid` no duplica.
- Campos ampliados siempre como opcionales; la web **tolera campos desconocidos** y versiones futuras de `schema_version`.
- Rutas de ficheros: ver §9 (privacidad). Por defecto enviar recuento de omitidos y, como mucho, una muestra recortada; el detalle completo se queda en el equipo.

---

## 6. Causas de fallo en lenguaje claro

La web guarda una **causa normalizada** (`cause_code`) para cada fallo y la traduce a texto para el técnico y para el cliente, con qué hacer. Ejemplos: destino no conectado, destino lleno, contraseña incorrecta, fichero abierto sin poder leer, sin permisos, red caída (destino de red), copia cancelada, apagado durante la copia, error del repositorio (necesita verificación), servicio parado.
- El agente envía el mensaje técnico y, si puede, la causa normalizada; si no, la web la deduce de los códigos de salida y de los textos conocidos y marca «desconocida» sin inventarla.
- El panel muestra la causa y la acción recomendada («Conecta el disco USB y pulsa Reintentar»), y el detalle técnico en un desplegable.

---

## 7. Panel web (técnico y cliente)

Diseño coherente con el panel AMG existente. Módulo `backend\modules\BackupAmg` (como `Licencias`, `AppAccounts`): controladores delgados, modelos en `common\models`, lógica en `common\services`, vistas con `Html::encode()`, widgets `GridView`/`DetailView`/`ActiveForm`, assets con `AssetBundle`.

### Vista del técnico (administrador)
- **Hoy requiere atención:** lista priorizada de empresas y equipos con problemas (fallo, silencio, poco espacio, verificación fallida, copia adicional atrasada), con la causa y un enlace.
- **Empresas:** tabla con semáforo (verde, ámbar, rojo, gris), nº de equipos / plazas, última copia correcta, licencia y fecha de fin. Filtros y búsqueda.
- **Detalle de empresa:** datos, licencia, usuario principal, equipos, alertas abiertas, historial.
- **Detalle de equipo:** línea de tiempo de ejecuciones (estado, hora, duración, tamaño, causa), tendencia de datos y espacio libre, estado de cada destino, versiones, última verificación, botones «Revocar» y «Quitar».
- **Alertas:** abiertas y cerradas, con reconocimiento (marcar como atendida).
- **Licencias:** altas, plazas ocupadas/libres, regenerar clave, caducidades próximas.

### Semáforo (reglas)
- **Verde:** última copia correcta dentro de lo esperado (según horario y `stale_after_days`) y latidos recientes.
- **Ámbar:** copia con avisos, destino adicional atrasado, poco espacio, verificación pendiente, agente desactualizado, licencia próxima a caducar.
- **Rojo:** copia fallida o sin copia correcta desde hace más de `stale_after_days`.
- **Gris:** el equipo no da señales (sin latidos más de X minutos/horas; ver §8).
- Nunca verde si falta algo; siempre con texto, no solo color.

### Vista del cliente (usuario principal de la empresa)
- Solo lectura de **sus** equipos: estado, últimas copias, próxima copia, resumen sencillo («Tus archivos están protegidos»).
- Alta/baja de sus equipos dentro del límite de la licencia, y regeneración de su clave.
- Sin detalles técnicos innecesarios. Aislamiento total entre empresas (§9).

---

## 8. Alertas

**Tipos:**
1. **Copia fallida** (cualquier equipo).
2. **Sin señales:** el equipo no envía latidos. Umbral configurable por equipo (por defecto, p. ej., 24 h para PC de oficina, para evitar falsos avisos por equipos apagados fuera de horario). Es la alerta más importante.
3. **Sin copia correcta** en más de `stale_after_days`.
4. **Poco espacio** en el destino.
5. **Verificación fallida** o muy atrasada.
6. **Copia adicional atrasada o con error.**
7. **Licencia próxima a caducar / límite de equipos alcanzado / alta nueva de equipo.**
8. **Agente desactualizado** (versión inferior a la mínima).

**Reglas de funcionamiento:**
- Una alerta se abre una vez por causa (`dedupe_key`), **no** se repite en bucle: recordatorio cada X horas mientras siga abierta y aviso de «resuelta» al cerrarse sola.
- Horario tranquilo configurable; severidad que determina el canal.
- Canales: **correo** (`BrandedMail`) y **Telegram** (`SystemTelegramNotifier`), usando la cola existente. Resumen diario por correo al técnico (qué ha pasado en cada empresa).
- Destinatarios: técnico (todas) y, opcionalmente, contacto de la empresa (solo las suyas, texto sencillo).
- Un comando de consola programado (cron) revisa el silencio y los umbrales cada pocos minutos.

**Informe periódico al cliente:** correo semanal o mensual por empresa («esta semana: 14 copias correctas, 1 con avisos; espacio disponible; próxima verificación»), con marca AMG. Pensado también como prueba de valor del servicio.

---

## 9. Seguridad, privacidad y retención

**Seguridad**
- Solo HTTPS. Claves de equipo y de licencia almacenadas con hash; nunca en logs, informes ni URLs.
- Límite de peticiones por IP y por equipo (`RequestRateLimit`); respuestas que no revelan si existe una clave.
- Aislamiento entre empresas: toda consulta filtra por `company_id`; pruebas específicas de que una empresa no ve datos de otra (incluidas las rutas de descarga o exportación).
- Acceso de administración con 2FA; registro de auditoría de altas, revocaciones, regeneraciones de clave y reconocimiento de alertas.
- La web **no ejecuta nada** en los equipos en las primeras fases. La configuración remota (fase 4) exige aprobación y firma, y nunca cambia contraseñas ni destinos sin confirmación local.
- Validación de entradas con `rules()`, parámetros enlazados en SQL, escape en vistas.

**Privacidad (RGPD)**
- Los informes pueden contener **rutas de ficheros**, que a veces incluyen nombres de personas o de clientes. Política por defecto: no guardar rutas completas; guardar recuentos y causas, y, a lo sumo, la extensión y una ruta recortada. Opción por empresa para guardar más detalle, informando al cliente.
- Ampliar `docs/registro-actividades-tratamiento.md` del panel con este tratamiento (finalidad, base, conservación, encargado).
- **No se envían contenidos de ficheros ni contraseñas del repositorio.**

**Retención**
- Latidos: conservar solo el último más un resumen diario.
- Ejecuciones y sus cifras: p. ej. 13 meses; el JSON original comprimido: p. ej. 90 días; incidencias detalladas: 90 días. **Decidir los plazos con el usuario.**
- Tarea programada de purga que no afecta a los datos de alertas abiertas.

---

## 10. Entregas (fases)

1. **Núcleo y recepción:** migraciones; alta de la app `BackupAMG`; empresa y licencia; `enroll`, `heartbeat`, `reports` con clave por equipo e idempotencia; cliente HTTPS y cola local en el agente; lista básica de empresas y equipos. Tests de API y de roles.
2. **Semáforo, detalle y alertas:** reglas de semáforo, causas normalizadas, pantalla «Hoy requiere atención», alertas de fallo y de silencio por correo y Telegram, resumen diario, recordatorios y cierre automático.
3. **Portal del cliente e informes periódicos:** vista de solo lectura, alta/baja de equipos dentro del límite, correo semanal/mensual.
4. **Configuración remota y actualizaciones** (lo más delicado, al final): descarga de configuración firmada, aviso y control de versiones. **Preguntar antes** de diseñar la actualización automática del agente.

---

## 11. Casos que deben funcionar (pruebas obligatorias)

1. Alta de un equipo con licencia vigente y plaza libre: queda en la lista de la empresa y recibe su clave.
2. Alta con límite alcanzado y con licencia caducada: respuesta clara; **el agente sigue copiando**.
3. Reinstalar el agente en el mismo PC: no ocupa una plaza nueva.
4. Revocar un equipo: deja de enviar y sigue copiando.
5. Sin conexión durante horas: la cola local guarda los informes; al volver se envían todos **sin duplicados** y en orden.
6. Mismo informe enviado dos veces: una sola ejecución en la web.
7. Copia fallida: aparece con causa en lenguaje claro y genera una alerta única, no un bucle de avisos.
8. Equipo apagado o sin señales más del umbral: alerta de silencio; al volver, se cierra sola.
9. Copia adicional atrasada: semáforo ámbar, nunca rojo.
10. Una empresa no ve datos de otra (API y panel); tokens y claves nunca en claro en base de datos ni logs.
11. Informe con `schema_version` futuro o campos desconocidos: se acepta sin fallar.
12. Reloj del equipo desfasado: se detecta y se avisa.
13. Carga: 100 equipos enviando latidos cada 10 minutos y copias diarias sin degradar el panel.
14. Purga de retención: borra lo previsto y respeta alertas abiertas.

---

## 12. Decisiones pendientes (para el usuario)

- Plazos de retención (§9) y política de rutas de ficheros (qué detalle se guarda).
- Umbral de «sin señales» por defecto y horario tranquilo.
- Precio/planes y qué distingue un plan de otro (nº de equipos, ¿servidor frente a PC?).
- Si el cliente puede ver solo su estado o también el historial detallado.
- Si habrá otros técnicos con permisos distintos al administrador.
- Capacidad y requisitos del servidor (versión de PHP, base de datos, cron, correo) y dominio/ruta pública de la API.
- Nombre público de la app y de la ruta (`/backup/v1`).

---

## 13. Cómo debe trabajar el asistente

- **No empezar hasta que el usuario lo confirme.** La fiabilidad de las copias va primero: la web no puede, por ningún fallo, hacer que el agente deje de copiar.
- Antes de implementar, **leer a fondo** `AccessCatalog`, `user_access`, `AppAuthService` y el módulo `Licencias` del panel y **corregir este documento** donde la realidad sea distinta, como se hizo con la especificación multidestino.
- Seguir las reglas de `AGENTS.md` del panel: PSR-12, `declare(strict_types=1)`, tipado, controladores delgados, modelos en `common/models`, migraciones con `safeUp()`/`safeDown()`, `Html::encode()`, AssetBundles.
- Dependencia nueva, cambio de formato o de arquitectura no cubierto: **preguntar antes**.
- Probar antes de decir que algo está hecho: agente real contra un panel de pruebas, con un equipo real y, al menos, dos empresas para verificar el aislamiento.
- Textos en español natural y sin términos técnicos para el cliente; sin commit ni push salvo que el usuario lo pida.

---

## 16. Verificación contra el panel real (15-nov-2026)

Lectura de `D:\Proyectos\yii2\amg` (solo lectura). **Esta sección prevalece sobre el resto.**

### Entorno
- **PHP ≥ 8.5**, **Yii 2.0.55**, `mdmsoft/yii2-admin` (RBAC), plantilla Advanced (`frontend`, `backend`, `common`, `console`). `docker-compose.yml` es el de la plantilla (MySQL 5.7), probablemente solo de desarrollo: **comprobar la base de datos y el hosting reales** antes de dimensionar.
- Ya hay comandos de consola para tareas programadas: `QueueController` (`schedule`, `work`, `stats`), `AvisosController` (`diarios`), `BackupController`, `AppAccountsController`. Los detectores de silencio, la purga y los resúmenes se añaden como comandos de consola con cron, siguiendo ese patrón.

### Permisos del panel (administradores y técnicos)
- Los administradores son usuarios de la tabla `user`, con RBAC (`mdm\admin`) y una tabla `user_access` (`user_id`, `profile`, `grants_json`): permisos **por pieza del menú** y acción (ver, crear, modificar, borrar, publicar, importar, asignar), definidos en `common\components\AccessCatalog` con las rutas de cada pieza.
- **Consecuencia:** el módulo nuevo se integra añadiendo una **pieza «Copias de seguridad»** a `AccessCatalog` con sus rutas por acción, y así los permisos de otros técnicos quedan resueltos sin inventar nada (responde a la decisión pendiente sobre «otros técnicos»). Un técnico puede tener solo esa pieza.

### Cuentas y licencias
- `app_account` (cuenta única por persona, con `status`, `requested_app_id`…), `app_account_app` (apps habilitadas por cuenta), `app_session` (`token_hash`, `device_id`, `app_version`), `license` (`app_name`, `license_key`, `status` active/expired/revoked/suspended, `plan`, `max_devices`, `grace_days_offline`, `fecha_inicio`, `fecha_fin`, `app_account_id`) y `license_device` (único por `license_id` + `device_id`).
- `AppAuthService` ya resuelve registro, login, sesiones por `Bearer`, límite de dispositivos y solicitud de acceso a una app.
- **Cosas que hay que tocar para añadir la app «BackupAMG»:**
  - `AmgApp` (valores por defecto) y su tabla `amg_app`.
  - Las constantes de identificador de app en `AppAccount` y en `License` (`APP_SALUD`, `APP_TIRO`, `APP_MOTO`, `APP_SIM112`) y la normalización de nombres de app en licencias (hay una migración que lo hace).
  - **Los planes de `License` son los de TiroAMG** («Entrenamiento», «Competición», «Competición Plus», «Completo»). Para BackupAMG hacen falta planes propios por app (p. ej. por nº de equipos) o planes genéricos. **Decidir antes de implementar.**
- El límite de equipos por empresa ya tiene soporte: `license.max_devices` + `license_device`. Lo nuevo es la ficha de empresa y los datos de cada equipo (`bk_company`, `bk_machine`).

### Portal del cliente: no existe todavía
- Las cuentas de app se autentican hoy **por API con token** (apps móviles); en `frontend` solo existe una pantalla web de restablecer contraseña (`CuentaAmgController::actionNuevaContrasena`). **No hay un área web de cliente.**
- Para el portal de empresa (fase 3) hay que **crear un inicio de sesión web** para `app_account`, separado de los usuarios `user` del panel, reutilizando `AppAuthService` (verificación de contraseña, estado, intentos) y `LoginSecurityService`/`RateLimitFilter`. Es un trabajo propio dentro de la fase 3, no un simple cambio de vista.

### Avisos
- Ya existen `TelegramNotifier` y `SystemTelegramNotifier` (avisos de sistema, activos solo si Telegram está configurado en `cms_settings`), `NotifyChannels`, `BrandedMail` y `tenant_job_queue`/`QueueController`. Reutilizarlos para las alertas y el correo; **crear un canal nuevo** `backup_alerts` si el catálogo de canales (`NotifyChannels`) lo requiere.

### Lo que sigue sin comprobarse
- Hosting real (PHP, base de datos, cron, límites de tiempo de petición, HTTPS y dominio público para `/backup/v1`).
- Cómo se separa hoy el acceso entre `backend` (administración) y `frontend` en producción, y si la API de licencias queda expuesta por el backend o por un dominio aparte.
- Volumen esperado (nº de empresas y equipos) para dimensionar latidos e informes.

---

## 17. Decisiones del usuario (15-nov-2026)

1. **Sin zona de clientes por ahora.** La fase 3 (portal de empresa) **queda aplazada**; será una parte nueva, más adelante. En las primeras fases solo accede el técnico. El alta de equipos sigue siendo automática con la clave de licencia de la empresa (sección 3), pero **sin portal** el usuario principal no puede revocar ni quitar equipos: lo hace el técnico desde el panel. Los avisos de alta nueva por correo siguen valiendo.
2. **Entorno de pruebas del panel:** `https://panel.amg.desarrollo/`. La API de pruebas colgará de ahí. **Todo el desarrollo y las pruebas se hacen contra esa URL**; cuando llegue el momento solo se cambiará a la URL real. Por eso la dirección del servidor debe ser un **ajuste del agente** (configuración y comando de alta), nunca escrita en el código.
3. **Añadir BackupAMG como app** tocando las constantes de identificador de app (`AppAccount`, `License`), `AmgApp` y la normalización de nombres de licencia: **aprobado**.
4. **Planes y precios por número de equipos: configurables en el panel**, no escritos en el código. El agente **no** conoce los planes: solo recibe de la licencia (o del acceso) el número máximo de equipos y la fecha de fin. Hace falta un modelo de planes por app (p. ej. nombre, nº de equipos, precio) editable desde el panel, sin depender de los planes de TiroAMG.
5. **Retención del detalle de las copias (confirmada):** son tres cosas distintas. (a) Las versiones de los ficheros, en el destino del cliente, con la retención del agente (7 diarias, 4 semanales, 12 mensuales). (b) El historial local del equipo, unos 90 días. (c) El historial **en el panel web**: **resumen de cada ejecución 13 meses** y **detalle completo (informe original e incidencias) 90 días**, ambos **configurables en el panel** con esos valores por defecto.
7. **Instalador preparado por empresa (aprobado).** Para que nadie tenga que teclear la clave ni pueda equivocarse de empresa, el panel ofrece en la ficha de la empresa **«Descargar instalador para esta empresa»**. Diseño:
   - El panel **no recompila** el instalador (corre en un servidor sin Inno Setup). Entrega un **paquete .zip** con el instalador genérico (`BackupAgent-Setup.exe`, el mismo para todos) y un fichero pequeño `alta-backupamg.json` con: dirección del servidor y un **código de instalación** de la empresa.
   - **El paquete no lleva la clave de licencia maestra**, sino un **código de instalación** propio: ligado a una empresa, con caducidad (p. ej. 30 días) y que el técnico puede **revocar o regenerar** sin tocar la licencia ni los equipos ya dados de alta. Si el paquete se filtra, se revoca el código y ya está. El límite de equipos de la licencia sigue aplicándose igual.
   - El instalador (Inno Setup) busca `alta-backupamg.json` **junto a sí mismo**; si lo encuentra, lo pasa al agente y el alta se hace sola en el primer arranque. Si no lo encuentra, el asistente pide la clave a mano como hasta ahora. Nunca se lanza `cmd`/PowerShell (`docs\ANTIVIRUS.md`).
   - `enroll` acepta **o** la clave de licencia **o** un código de instalación válido; el resultado es el mismo equipo en la misma empresa.
   - Todo alta hecha con un código queda registrada (qué código, cuándo, qué equipo) en el registro de altas.
   - Va al **final de la fase 1** (después de que `enroll` funcione con la clave), en el panel y en el agente/instalador.
6. **Flujo de trabajo:** **antes de tocar código en el panel Yii2 hay que crear una rama `feature`** (flujo del usuario con git-flow), y lo mismo en este repositorio para los cambios del agente. Sin commit ni push salvo que el usuario lo pida.

---

## 18. Agente: implementación de la fase 1 (15-nov-2026)

El contrato real de la API está en el plan del panel (`D:\Proyectos\yii2\amg\docs\plan-backupamg-fase1.md`, sección 6 y «Bloque final»); manda sobre lo anterior. Lo que hace el agente:

- **Identificadores:** `report_uid` (UUID v4, opcional) en cada informe; los antiguos reciben uno estable derivado del nombre de fichero. `machine_uid = sha256("backupamg|v1|" + MachineGuid)`; en desarrollo (`BACKUP_AGENT_DATA_DIR`) se puede forzar con `BACKUP_AGENT_MACHINE_UID`.
- **Ajustes** (`config.yaml`, sección `cloud`): `server_url` (base, sin `/backup/v1`; https, o http solo a localhost) y `send_paths` (por defecto no). La clave propia del equipo, cifrada con DPAPI, y la cola de informes están en `cloud.json` de la carpeta de datos.
- **Alta:** con clave de licencia (`BKA-…`), con código de instalación (`BKI-…`) o con `alta-backupamg.json` (en la carpeta de datos; el instalador lo copia desde junto a sí mismo). Asistente (último paso, opcional), Ajustes → Plataforma web y consola (`agent enroll [--server URL] <clave|código>`, `agent enroll-file [ruta]`). El fichero se borra al resolverse (éxito o error definitivo) y se conserva si fue red o límite.
- **Envío** (hilo propio, `internal/cloud`): latido cada `heartbeat_interval_seconds` (600) y tras cada informe; informes por lotes de 50 (máx. 256 KB cada uno); cola persistente; reintentos con espera creciente (1 min … 1 h) para red, 429 y 5xx; decisiones de negocio (`license_*`, `seat_limit_reached`, `machine_revoked`, `invalid_key`) se muestran y se vuelve a mirar cada 6 h; `invalid_key` no provoca un alta automática. `stored` y `duplicate` se dan por enviados; `rejected` se descarta y se anota. **Nada de esto detiene ni retrasa las copias.**
- **Privacidad:** se quitan `sources` y `repository`; `skipped_files[].path` viaja como «huella.extensión» (con `send_paths`, la ruta); `errors` y `message` pierden las rutas («[ruta]»).
- **Estados visibles** (panel local, Inicio y bandeja): Conectado, Sin conexión, Licencia caducada, Licencia suspendida, Límite de equipos, Equipo desactivado, Hay que volver a conectar, Sin conectar, Sin configurar.
- **Estado de los destinos** en el latido: `up_to_date`, `pending`, `offline`, `error`, `unknown`.
- **Probado contra `https://panel.amg.desarrollo/`:** alta con clave y con código, alta automática con el fichero, reinstalación sin gastar plaza, límite de plazas, latido, informes sin duplicados al reenviar, sin conexión y vuelta, equipo revocado y reactivado. Pendiente: licencia caducada (el panel aún no permite cambiar la fecha) e instalación real con `alta-backupamg.json`.
