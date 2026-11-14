# Especificación: Varios destinos de copia (regla 3-2-1) — BackupAMG

> Documento para el asistente de programación.
> Continúa `ESPECIFICACION_AGENTE.md`, `ESPECIFICACION_INTERFAZ.md` y `ESPECIFICACION_DISENO.md`.
> **Estado: borrador para después de las pruebas reales y del uso piloto** (hoja de ruta, punto 2). No empezar antes sin confirmación del usuario.
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
- Los repositorios secundarios se crean con los mismos parámetros que el principal para que `restic copy` pueda trabajar entre ellos. **Decisión pendiente:** usar la misma contraseña que el principal (más simple, una sola que guardar) o una distinta por destino (más seguro si un destino se compromete). Recomendación: **la misma contraseña**, y dejarlo claro en la hoja de recuperación.
- Si no hay copia nueva que replicar, no se hace nada (no se repite trabajo).
- Si la réplica de un destino falla, el informe de la copia principal **sigue siendo correcto**; el fallo se anota en el estado de ese destino y se avisa (ver §5).
- Reglas de CLAUDE.md que siguen valiendo: nada de PowerShell/`cmd`/`rundll32` lanzados por el programa, nada de UPX, contraseña solo por `RESTIC_PASSWORD`, nunca en claro en logs ni argumentos.

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
- Una réplica en curso **no impide** que empiece la siguiente copia principal. **Decisión pendiente:** si hay conflicto de bloqueo sobre el repositorio principal, la copia principal tiene prioridad y la réplica se interrumpe y se reintenta.

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
