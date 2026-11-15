# Especificación: Varios destinos de copia (regla 3-2-1) — BackupAMG

> Documento para el asistente de programación.
> Continúa `ESPECIFICACION_AGENTE.md`, `ESPECIFICACION_INTERFAZ.md` y `ESPECIFICACION_DISENO.md`.
> **Estado (nov 2026): en desarrollo, fases 1 a 3 autorizadas por el usuario.** Fases 1 (núcleo) hechas en Go y probadas con restic real; faltan el panel (2) y restaurar desde un secundario (3). La fase 4 (nube) sigue exigiendo preguntar antes. Las secciones 1 a 15 son el diseño original; **la sección 16 recoge las correcciones y decisiones tras contrastarlo con el código real y prevalece sobre ellas.**
> **Objetivo:** que un equipo pueda guardar sus copias en **más de un sitio** (por ejemplo, disco local + disco externo + nube), siguiendo la regla 3-2-1: tres copias de los datos, en dos tipos de soporte distintos, una de ellas fuera del sitio.

---

## 1. Principios

1. **La fiabilidad de la copia principal va primero.** Un destino secundario que falla, está desconectado o va lento **nunca** hace fallar ni retrasa la copia principal.
2. **Un destino principal, varios secundarios.** La copia se hace siempre en el principal. Los secundarios reciben una **réplica** de ese repositorio después.
3. **Compatible hacia atrás.** Una configuración sin destinos secundarios funciona exactamente igual que hoy. No se rompe el formato de la configuración ni el de los informes (`schema_version: 1`; solo se amplía con campos opcionales).
4. **Lenguaje sencillo.** En el panel: «Copia principal», «Copia adicional», «Copia fuera de este equipo». Nada de «repositorio», «replicación» o «restic» fuera de Ajustes y del detalle técnico.
5. **Sin sorpresas con el dinero.** Los destinos en la nube pueden costar. Mostrar siempre el espacio usado y avisar antes de activar uno.

---

## 2. Cómo funciona

```
Copia programada
   │
   ├─ 1. Copia al destino PRINCIPAL (como hoy)  ──► informe «Correcta / Con avisos / Fallida»
   │
   └─ 2. Si el paso 1 salió bien:
         para cada destino SECUNDARIO activo, en orden:
            réplica del repositorio principal (restic copy) ──► estado propio de cada destino
```

- La réplica usa el mecanismo de copia entre repositorios del motor (`restic copy`). **No reimplementar el motor.**
- Los repositorios secundarios se crean con los mismos parámetros que el principal para que `restic copy` pueda trabajar entre ellos. **Decisión tomada:** la misma contraseña que el principal (una sola que guardar), sin opción en el asistente. Dejarlo claro en la hoja de recuperación. Una contraseña distinta por destino solo se plantea con la nube (fase 4).
- Si no hay copia nueva que replicar, no se hace nada (no se repite trabajo).
- Si la réplica de un destino falla, el informe de la copia principal **sigue siendo correcto**; el fallo se anota en el estado de ese destino y se avisa (ver §5).
- Reglas de CLAUDE.md que siguen valiendo: nada de PowerShell/`cmd`/`rundll32` lanzados por el programa, nada de UPX, contraseña solo por entorno (`RESTIC_PASSWORD` y, en la réplica, `RESTIC_FROM_PASSWORD`), nunca en claro en logs ni argumentos.

---

## 3. Tipos de destino

| Tipo | Fase | Notas |
|---|---|---|
| Disco local o USB | 1 | Como hoy. Identificado por `volume_id`, no solo por la letra. |
| Carpeta de red (UNC) | 1 | Como hoy. Solo rutas UNC; el servicio no ve unidades mapeadas. |
| Nube compatible con S3 (Backblaze B2, Wasabi, Amazon S3…) | 2 | Claves de acceso guardadas cifradas con DPAPI de máquina, igual que la contraseña. |
| Otros (SFTP, etc.) | Fuera de alcance | Solo si hay demanda real. |

Los destinos en la nube necesitan **dependencia o configuración nueva**: antes de implementarlos, **preguntar al usuario** (regla 8 de CLAUDE.md), junto con la decisión sobre credenciales y costes.

---

## 4. Configuración (ampliación compatible)

```yaml
repository:                 # destino PRINCIPAL: igual que hoy
  path: "E:\\Copias"
  volume_id: "..."
  password_encrypted: "..."

replicas:                   # OPCIONAL. Si falta o está vacía, todo funciona como hoy.
  - id: "usb-oficina"       # identificador interno estable
    name: "Disco externo"   # nombre visible en el panel
    type: "local"           # local | unc | s3
    path: "F:\\CopiasExtra"
    volume_id: "..."
    enabled: true
    when: "after_backup"    # after_backup | daily_at (ver §6)
  - id: "nube"
    name: "Nube (Backblaze)"
    type: "s3"
    endpoint: "s3.eu-central-003.backblazeb2.com"
    bucket: "copias-cliente-01"
    credentials_encrypted: "..."   # DPAPI de máquina; nunca en claro
    enabled: true
    when: "daily_at"
    time: "23:00"
```

- Validación: ids únicos, ruta no vacía, el destino secundario **no puede ser el mismo** que el principal ni estar dentro de él, ni uno de los orígenes de la copia.
- Una configuración inválida **nunca** sustituye a la que está en uso (regla 1 de CLAUDE.md).

---

## 5. Estado y avisos

Estado propio por destino secundario, guardado en `state.json` de forma compatible:

- Última réplica correcta (fecha y hora).
- Último intento y resultado.
- Número de copias pendientes de replicar.
- Espacio usado (si el destino lo permite).

**Semáforo de Inicio:**

- Verde: copia principal al día **y** todas las copias adicionales al día.
- Ámbar: la principal está bien, pero una copia adicional lleva más de `stale_after_days` sin replicarse. Texto: «Tus archivos están protegidos, pero la copia en {nombre} no se ha actualizado desde hace {n} días».
- Rojo: solo cuando falla la principal (como hoy).
- Nunca mostrar «protegido en 3 sitios» si alguno no está al día.

Notificaciones de Windows con el mismo criterio que hoy: título claro y acción «Abrir panel».

---

## 6. Cuándo se replica

- **`after_backup` (por defecto en discos y red):** al terminar con éxito cada copia principal.
- **`daily_at` (recomendado en la nube):** una vez al día a una hora fijada, para no saturar la conexión en horas de trabajo.
- Límite de velocidad opcional para la nube (`limit_upload_kbps`), por defecto sin límite.
- Si el destino no está disponible (USB desconectado, sin red), se reintenta en la siguiente ocasión con el mismo `max_retries`/`retry_delay_minutes` que la copia principal, sin bloquear nada más.
- Una réplica en curso **no impide** que empiece la siguiente copia principal. **Decisión tomada:** prioridad absoluta a la copia principal: si llega, la réplica en curso se cancela y se reintenta después (ver §16.2).

---

## 7. Retención y verificación

- La retención del destino principal sigue igual.
- En cada destino secundario se aplica la **misma política** de retención por defecto (opción de una política distinta, por ejemplo más corta en la nube, como ampliación posterior).
- La limpieza (`prune`) de un secundario es un paso separado, con su propio resultado: un fallo de limpieza **no marca** la réplica como fallida si la copia se hizo bien (misma regla que la copia principal).
- Verificación periódica: por defecto solo en el principal. Opción de verificar un secundario (en la nube suele ser costoso por la descarga).

---

## 8. Restaurar

- Por defecto se restaura desde el **destino principal**.
- Si el principal no está disponible o falla, el panel ofrece «Restaurar desde otra copia» y deja elegir entre los secundarios disponibles.
- La pantalla Restaurar mostrará de dónde procede la copia: «Disco local», «Disco externo», «Nube». Mismas fechas y mismo flujo en los tres pasos.
- Restaurar desde la nube: avisar de que puede tardar y consumir datos.

---

## 9. Panel (diseño)

Seguir `ESPECIFICACION_DISENO.md` (tokens, tarjetas, pastillas, barra de guardado).

- **Destino** pasa a llamarse **Destinos**:
  - Tarjeta «Copia principal» (como la actual de Destino).
  - Sección «Copias adicionales» con una tarjeta por destino: nombre, tipo, estado (pastilla con texto: «Al día», «Pendiente», «Sin conexión», «Error»), última réplica, espacio usado y acciones «Probar», «Editar», «Quitar».
  - Botón «Añadir copia adicional» que abre un asistente corto de 3 pasos: tipo de destino → datos (carpeta o credenciales) → probar y guardar.
  - Indicador sencillo de la regla 3-2-1: «Tienes 2 copias en 2 tipos de soporte. Te falta una copia fuera de este equipo». Con enlace para añadirla. Solo consejo, nunca alarmista.
- **Inicio:** la tarjeta Destino muestra «Copia principal» y una línea por cada copia adicional con su estado.
- **Asistente de primera configuración:** sin cambios; en el último paso, una línea opcional «Más adelante podrás añadir otra copia en un disco externo o en la nube».
- Confirmación antes de quitar un destino: «Se dejará de copiar en {nombre}. Las copias que ya hay allí no se borran».
- Estados vacíos, errores con causa y acción, y el botón «Siguiente» desactivado hasta que el destino se haya probado, igual que en el resto del panel.

---

## 10. API del panel (ampliación compatible)

Solo añadir rutas y campos opcionales; mantener las existentes. Mismas reglas de seguridad (token, cookie, CSRF, Host/Origin) y roles (solo administrador configura).

- `GET /api/v1/replicas` — lista y estado de los destinos secundarios.
- `POST /api/v1/replicas` — alta (con prueba previa del destino y preparación del repositorio, como hace `prepareRepository` en el asistente).
- `PUT /api/v1/replicas/{id}` y `DELETE /api/v1/replicas/{id}`.
- `POST /api/v1/replicas/{id}/test` — prueba el destino sin guardar.
- `POST /api/v1/replicas/{id}/run` — réplica manual («Copiar ahora a este destino»).
- El estado de `GET /api/v1/status` incorpora, de forma opcional, el resumen de las copias adicionales.

---

## 11. Informes

- Informe de copia principal: igual que hoy.
- Informe de réplica (nuevo tipo, `type: "replica"`), con `schema_version: 1` y los mismos campos comunes: destino, resultado, duración, datos transferidos, errores.
- No cambiar el formato de los informes existentes; las herramientas que ya los lean no deben romperse.

---

## 12. Casos que deben funcionar (pruebas obligatorias)

1. Sin `replicas` en la configuración: comportamiento idéntico al actual.
2. Copia principal correcta + réplica a disco externo correcta: dos destinos al día, informes correctos.
3. Disco externo desconectado: la copia principal sigue «Correcta»; el estado del secundario es «Sin conexión»; Inicio en ámbar tras `stale_after_days`; al reconectarlo (aunque cambie de letra) se replica solo.
4. Fallo de red durante la réplica a la nube: se reintenta, no bloquea la copia principal ni deja el repositorio principal en mal estado.
5. Apagado brusco durante una réplica: el siguiente arranque la reanuda o la reintenta sin dañar nada.
6. Cancelar una réplica no tumba el agente.
7. Destino secundario igual al principal, o dentro de él, o dentro de un origen: la configuración se rechaza con un mensaje claro.
8. Credenciales de nube: cifradas con DPAPI, nunca en claro en configuración, logs, informes ni argumentos.
9. Restaurar con el principal caído: se puede restaurar desde un secundario y los archivos se abren (sobre todo el `.pst`).
10. Quitar un destino no borra nada en él.
11. Cambiar de destino principal con réplicas existentes: aviso claro de qué ocurre.
12. Usuario sin permisos de administrador: ve el estado de las copias, no puede configurarlas.

---

## 13. Fases

1. **Núcleo:** configuración `replicas`, validación, réplica a disco local/USB/UNC con `restic copy`, estado por destino, informes de réplica y tests en Go.
2. **Panel:** pantalla Destinos, asistente «Añadir copia adicional», estado en Inicio y semáforo ámbar.
3. **Restaurar desde un secundario.**
4. **Nube (S3):** **antes de empezar, preguntar al usuario** por dependencias, almacenamiento de credenciales, límites de velocidad y costes. Después implementar y probar con el proveedor elegido.
5. **Pulido:** límite de velocidad, política de retención propia por destino, verificación opcional de secundarios, notificaciones.

---

## 14. Fuera de alcance

- Replicación entre equipos o desde la plataforma web central (va con la fase 4 de la hoja de ruta).
- Cifrado distinto por destino, destinos SFTP u otros proveedores.
- Mover automáticamente copias antiguas entre destinos.
- Actualizaciones automáticas, macOS y Linux.

---

## 15. Cómo debe trabajar el asistente

- No empezar hasta que el usuario lo confirme (tras pruebas reales y piloto).
- La fiabilidad de las copias va primero; cualquier duda sobre riesgo, preguntar antes de tocar.
- Dependencia nueva, cambio de formato o decisión de arquitectura no cubierta: **preguntar antes**.
- Probar antes de decir que algo está hecho: compilar no basta; reiniciar el binario afectado y comprobar el síntoma concreto, con un destino secundario real (USB y, en la fase 4, nube).
- Textos en `textos.js`, en español natural y sin términos técnicos fuera de Ajustes y del detalle.
- No hacer commit ni push salvo que el usuario lo pida.

---

## 16. Correcciones y decisiones tras contrastar con el código (nov 2026)

Este apartado prevalece sobre lo anterior. La especificación se escribió sin mirar el código; esto es lo que cambia.

### 16.1 Decisiones del usuario

1. **Contraseña:** la misma que la del principal, sin opción en el asistente.
2. **Réplica contra copia principal:** prioridad absoluta a la principal. La réplica se cancela, se reintenta después, y tras `3` interrupciones seguidas espera a un hueco de más de una hora hasta la siguiente copia (estado «Pendiente», sin alarma). Un fallo de réplica nunca cambia el estado de la copia principal ni pone el semáforo en rojo (solo ámbar por antigüedad, fase 2).

### 16.2 Correcciones

- **a) Guardar Ajustes borraba la sección.** `config.Parse` rechaza campos desconocidos y `marshalFile` (`internal/config/update.go`) escribe una estructura fija: si no se amplía, cualquier guardado de Ajustes elimina `replicas`. Ya se escribe también ahí, con test de ida y vuelta. `SetDestination`, `SetReplicas` y el cambio de contraseña editan el árbol YAML y la conservan. Una configuración con `replicas` no se puede abrir con un agente anterior (campo desconocido); no hay vuelta atrás de versión con réplicas configuradas.
- **b) Contraseña del origen en `restic copy`.** La contraseña solo iba por `RESTIC_PASSWORD`. Para copiar entre repositorios se pasa también `RESTIC_FROM_PASSWORD` (comprobado con restic 0.19.1) y `--from-repo`. Se limpian del entorno heredado todas las variables `RESTIC_FROM_*` y nunca viaja en argumentos ni en logs ni en mensajes de error.
- **c) Deduplicación.** El repositorio secundario se crea con `restic init --from-repo <principal> --copy-chunker-params`; si no, lo copiado no se reconoce como igual y ocupa hasta el doble.
- **d) Estado e informes.** `state.json` gana, de forma opcional, `last_good_snapshot` (instantánea de la última copia correcta o con avisos; no la borra una copia fallida posterior) y `replicas`: un mapa por id con `last_success_at`, `last_snapshot` (instantánea principal ya replicada), `last_attempt_at`, `last_status` (`success`, `failed`, `offline`, `interrupted`), `last_error`, `report` e `interrupted`. «Pendiente» = la última instantánea correcta del principal no está en la adicional. Los informes `type: "replica"` añaden `replica_id`, `replica_name` y `snapshots_copied` solo en ese tipo (`omitempty`): los de copia y verificación no cambian. Solo se escribe informe de réplica si se copió algo o falló; «sin conexión» e «interrumpida» no generan informe.
- **e) Concurrencia.** La réplica corre en su propio hilo (`replicaLoop`), no en el del planificador, y usa la sala común de actividad (`live.enter`, tipo `replica`) para no solaparse con copia, verificación ni restauración. La copia principal (programada o «Copiar ahora») cancela una réplica en curso y espera a que suelte la sala. No empieza una réplica si la próxima copia principal está a menos de 10 minutos (60 si ya fue interrumpida 3 veces), ni mientras corre la verificación posterior a la copia. Tras fallar, no se reintenta antes de 30 minutos. Se comprueba cada 5 minutos, tras cada copia y al cambiar la configuración; así un disco que se reconecta (aunque cambie de letra) se pone al día solo. La actividad `replica` no pone el semáforo en azul.
- **f) Validación de rutas.** Una copia adicional no puede ser la misma carpeta que la principal, ni estar dentro, ni contenerla; lo mismo con cada carpeta de origen y entre copias adicionales (`config.PathsOverlap`, sin distinguir mayúsculas). `type` es opcional (se deduce de la ruta: `unc` si empieza por `\`); `s3` se rechaza con un mensaje claro. `daily_at` exige `time` (HH:MM). Ids: minúsculas, números, guion y guion bajo, hasta 40 caracteres.
- **g) Restaurar desde un secundario (fase 3).** Hoy `snapshots`, `files`, `find` y `restore` usan siempre `cfg.Repository.Path`. Hay que parametrizar el repositorio por `replica_id` opcional en la API (por defecto el principal). Se hace en la fase 3.
- **h) Retención de la adicional.** Tras copiar algo, se aplica en la adicional la misma política (`forget`, con `prune` si está activado). Si falla, la réplica sigue siendo correcta y el aviso queda en `errors` del informe.
- **i) Nube (S3).** Fuera de la fase 1-3. No hay campos `s3` activos en el formato.

### 16.3 Implementación de la fase 1 (hecha)

- `internal/config/replica.go`: tipo `Replica`, validación, `SetReplicas`, `ValidateReplicas`.
- `internal/restic/replica.go`: `InitReplica` y `Copy`.
- `internal/state`, `internal/report`, `internal/status`: campos y tipo nuevos.
- `cmd/agent/replica.go`: `replicaLoop`, `runReplicas`, `replicateOne`, prioridad de la principal. Comando de consola `agent replicate [id]`.
- Pruebas: Go (config, estado, informes, restic real en carpetas temporales, decisión de cuándo replicar) y agente real con dos carpetas, réplica interrumpida por una copia manual y reanudada, y destino sin conexión.
### 16.4 Implementación de la fase 2 (hecha)

- **API** (solo administrador, con sesión, Origin y CSRF como el resto): `GET /api/v1/replicas`, `POST /api/v1/replicas` (prepara la carpeta con la contraseña del principal antes de guardar), `PUT` y `DELETE /api/v1/replicas/{id}` (quitar no borra nada en el destino), `POST /api/v1/replicas/{id}/run` y `POST /api/v1/replicas/test` (recibe la carpeta; sirve para probar antes de guardar y para una ya guardada). Diferencia con §10: la prueba no lleva `{id}`. `PUT` cambia nombre, pausa y cuándo; para otra carpeta se quita y se añade una nueva.
- **Estado:** `GET /api/v1/status` incluye `replicas` (sin rutas, lo ven todos los usuarios) con estado `ok`, `pending`, `offline`, `failed` o `disabled`, `stale` y `days_behind`. El semáforo pasa de verde a ámbar si una copia adicional activa lleva más de `schedule.stale_after_days` sin ponerse al día (desde su última réplica correcta o desde que se añadió); nunca a rojo.
- **Panel:** «Destino» pasa a «Destinos»: tarjeta «Copia principal», «Copias adicionales» (pastillas con texto, causa y acción, Actualizar ahora, Probar, Editar, Quitar con confirmación propia), asistente de 3 pasos que reutiliza el selector de carpetas, consejo informativo 3-2-1 y línea «Copias adicionales» en Inicio con enlace. Nuevo `copias.js`.
- La carpeta de la copia principal se ve como un resumen (ruta, disco, espacio libre, estado) con un botón «Cambiar carpeta» (`aria-expanded`) que despliega el selector; «Cancelar» o Esc lo pliega y devuelve el foco. Sin destino todavía, se muestra desplegado.

### 16.5 Implementación de la fase 3 (hecha)

- **API compatible:** `GET /api/v1/snapshots`, `.../{id}/files` y `.../{id}/find` aceptan `?replica=<id>`; `POST /api/v1/restore` acepta `replica_id`. Sin ellos, siempre la copia principal. Solo administrador, como antes. La copia adicional tiene que estar conectada y ser un repositorio; si no, error claro.
- **Panel:** en Restaurar, si hay copias adicionales, aparece «Recuperar desde:» con la copia principal y cada copia adicional (las desconectadas, desactivadas y marcadas «sin conexión»). Si la principal no está disponible se elige sola la primera adicional conectada, con un aviso de hasta cuándo está actualizada.
- La carpeta de recuperación no puede estar dentro de ninguna copia adicional.
- Probado con el agente real: con la copia principal quitada, se restauró desde la adicional y los ficheros eran idénticos (hash), la base de datos pasó su comprobación de integridad y el documento se abrió.

### 16.6 Cambio de contraseña con varios destinos (hecho)

Problema que se encontró al probar: el cambio de contraseña solo cambiaba la copia principal; las copias adicionales dejaban de replicarse y de poder abrirse. Ahora se cambia en todos los destinos, en dos fases, y nunca quedan repositorios con contraseñas distintas en uso:

0. Todos los repositorios (principal y cada copia adicional, también las pausadas) tienen que estar conectados y abrirse con la contraseña actual. Si no, **se bloquea** y se dice cuáles («Conecta “X” o quítala antes de cambiar la contraseña»). No hay modo «pendiente de contraseña». Se hace dentro de la sala de actividad; una copia programada que llega mientras tanto espera unos segundos.
A. En cada repositorio se añade la contraseña nueva (`restic key add`, con fichero temporal; nunca en argumentos ni en logs) y se comprueba que abre. Si algo falla, se quitan las nuevas ya añadidas y no cambia nada.
C. Se guarda la nueva en la configuración (DPAPI). Un corte en cualquier punto deja un estado válido: las dos contraseñas abren todos los repositorios hasta el final.
B. Se quita la antigua de cada repositorio. Si alguna falla (por ejemplo, la adicional se desconecta justo entonces), la nueva ya está en uso, se avisa, y la clave antigua queda anotada en `state.json` (`pending_key_removals`, solo identificadores) para quitarla en cuanto el destino vuelva.
- Mensaje claro cuando una copia adicional tiene otra contraseña: «Esta copia adicional tiene otra contraseña que la copia principal.»
- Panel: la tarjeta «Contraseña de las copias» pasa de Destinos a **Ajustes → Seguridad**, con el aviso de que protege todos los destinos, la confirmación propia y la oferta de volver a imprimir la hoja de recuperación (que ahora lista todos los destinos). En Destinos queda un enlace.
- **Pendiente (anotado, no hecho): reparar una copia adicional ya rota.** Una acción «Poner la contraseña actual» que pida la contraseña con la que abre esa copia y le añada la actual (`key add`), para casos creados con una versión anterior. En el piloto no debería hacer falta.
