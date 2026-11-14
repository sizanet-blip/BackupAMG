# Especificación: Rediseño visual del panel (BackupAMG)

> Documento para el asistente de programación (Cursor).
> Continúa `ESPECIFICACION_AGENTE.md` y `ESPECIFICACION_INTERFAZ.md`, que ya están implementadas.
> **Objetivo:** que el panel y el icono de bandeja se vean profesionales, coherentes y sean fáciles de usar para alguien no técnico.
> **No cambies** la API, la seguridad, el formato de informes ni la lógica del agente: este trabajo es solo de presentación (HTML, CSS, JS del panel y menú de la bandeja).

---

## 1. Principios

1. **Lo primero que se ve es la respuesta a "¿están bien mis copias?"**. Cada pantalla tiene una sola acción principal, claramente destacada.
2. **Lenguaje sencillo.** Nada de "snapshot", "repositorio", "prune", "restic", "VSS" a la vista. Usar: "copia", "destino", "comprobar copias", "instantánea de Windows" (solo en detalles). Los términos técnicos van únicamente en el detalle de un informe y en Ajustes.
3. **Consistencia.** Mismos colores, tamaños, espaciados, botones y tarjetas en todas las pantallas. Un mismo concepto siempre con la misma palabra.
4. **Calma.** Mucho espacio en blanco, fondo gris muy claro, tarjetas blancas, un único color de acento. Sin degradados, sombras fuertes, bordes de colores a un lado ni emojis.
5. **Accesible.** Botones de al menos 44 px de alto, contraste de texto 4,5:1, elementos reales (`<button>`, `<a>`, `<label>` + `<input>`), navegable con teclado y foco visible.

---

## 2. Sistema de diseño (tokens)

Definir todo como variables CSS en `:root` y usarlas siempre. Prohibido escribir colores o tamaños sueltos en las pantallas.

### Colores

| Token | Valor | Uso |
|---|---|---|
| `--bg` | `#F5F6F8` | Fondo de la aplicación |
| `--surface` | `#FFFFFF` | Tarjetas, barra lateral |
| `--surface-muted` | `#F7F8FA` | Bloques dentro de tarjetas |
| `--border` | `#E3E6EB` | Bordes de tarjetas |
| `--border-strong` | `#D3D8E0` | Bordes de campos y botones secundarios |
| `--divider` | `#EDEFF3` | Separadores de filas |
| `--text` | `#16181D` | Texto principal |
| `--text-2` | `#3B414C` | Texto secundario fuerte |
| `--text-muted` | `#5B6270` | Texto de apoyo |
| `--accent` | `#2350D8` | Botón principal, selección, progreso |
| `--accent-text` | `#1E44B8` | Enlaces y texto en acento |
| `--accent-soft` | `#EEF2FD` | Fondo de elemento activo |
| `--ok` / `--ok-soft` | `#17794A` / `#E6F4EC` | Correcto |
| `--warn` / `--warn-soft` | `#7A4F00` / `#FDF1D8` | Con avisos |
| `--error` / `--error-soft` | `#A8241A` / `#FCE9E7` | Fallo |
| `--neutral` | `#8A909B` | Servicio parado / sin datos |

Modo oscuro: mismas variables redefinidas bajo `@media (prefers-color-scheme: dark)`, manteniendo los contrastes. El estado nunca debe depender solo del color: siempre acompañado de texto ("Correcta", "Con avisos", "Fallida").

### Tipografía

- Fuente **Inter** incluida localmente en el ejecutable (ficheros `.woff2` embebidos, nunca desde Google Fonts). Alternativa: `system-ui, "Segoe UI", sans-serif`.
- Rutas y nombres de fichero: `Consolas, ui-monospace, monospace`.

| Uso | Tamaño | Peso |
|---|---|---|
| Título de pantalla (h1) | 24 px | 700 |
| Título de estado grande (h2) | 22 px | 700 |
| Título de tarjeta | 16 px | 600 |
| Texto normal | 14 px | 400–500 |
| Texto de apoyo | 13 px | 400 |
| Etiquetas pequeñas / pastillas | 12 px | 600 |

### Espaciado y formas

- Escala de espacios: 4, 8, 12, 16, 20, 24, 32, 40 px.
- Radio: tarjetas 14 px, botones y campos 10 px, pastillas 999 px.
- Padding de tarjeta: 20–28 px. Separación entre tarjetas: 16–24 px.
- Sin sombras (o una muy sutil en menús emergentes).

---

## 3. Estructura de la aplicación

- **Barra lateral fija a la izquierda** (≈ 240 px), fondo blanco con borde derecho:
  - Arriba: logo + nombre del programa + nombre del equipo.
  - Navegación: Inicio, Qué copiar, Destino, Programación, Historial, Restaurar, Ajustes. Cada elemento con icono de línea y texto; el activo con fondo `--accent-soft` y texto `--accent-text`.
  - Abajo: estado del servicio (punto verde + "Servicio activo · vX.Y.Z").
  - Para usuarios no administradores, ocultar (no solo deshabilitar) las secciones que no pueden usar.
- **Contenido** a la derecha: título de pantalla + una línea de explicación, y debajo las tarjetas.
- En ventanas estrechas la barra lateral pasa arriba.
- Iconos: un único conjunto de iconos de línea (por ejemplo Lucide, incluido localmente), trazo 2 px, 18 px en el menú.

---

## 4. Componentes

- **Botón principal**: fondo `--accent`, texto blanco, 44–48 px de alto. Solo uno por pantalla.
- **Botón secundario**: blanco con borde `--border-strong`.
- **Botón peligroso** (Cancelar copia, Quitar): texto `--error`, sin fondo rojo.
- **Tarjeta**: fondo blanco, borde `--border`, radio 14 px.
- **Pastilla de estado**: fondo suave + texto del color del estado, siempre con palabra.
- **Barra de progreso / espacio**: 8–12 px de alto, fondo `#EDEFF3`, relleno `--accent`, con texto debajo (no solo la barra).
- **Selección de carpetas**: tarjetas‑casilla; marcada = borde 2 px `--accent` y fondo `#F6F8FE`.
- **Interruptores de reglas**: fila con título + explicación en una línea + casilla a la derecha.
- **Barra de guardado**: aparece al final de las pantallas de configuración cuando hay cambios sin guardar ("Se copiarán 4 carpetas · 53,0 GB" + Descartar / Guardar cambios). Fondo oscuro para que destaque.
- **Avisos**: caja con icono, título en negrita y una frase. Amarillo para advertencias importantes, azul para información.
- **Estados vacíos**: nunca una tabla vacía. Frase que explique qué pasa y un botón para resolverlo ("Todavía no hay copias. Hacer la primera copia").
- **Mensajes de error**: qué ha pasado + qué tiene que hacer el usuario ("El disco USB no está conectado. Conéctalo y pulsa Reintentar").

---

## 5. Pantallas (seguir el diseño de referencia)

El diseño de referencia está en el lienzo "BackupAMG – Rediseño del panel" (capturas adjuntas por el usuario). Reproducirlo fielmente.

1. **Inicio**
   - Tarjeta grande de estado: icono circular con escudo, pastilla ("Todo correcto" / "Con avisos" / "Atención" / "Copiando"), frase principal ("Tus archivos están protegidos"), última copia y próxima copia. Botones a la derecha: "Verificar copias" (secundario) y "Copiar ahora" (principal).
   - Fila de 3 tarjetas: **Destino** (nombre, conectado sí/no, barra de espacio), **Qué se copia** (nº de carpetas y tamaño, enlace "Cambiar selección"), **Última verificación**.
   - **Actividad reciente**: 5 últimas ejecuciones con pastilla, descripción en lenguaje claro, tamaño/duración y fecha relativa ("Hoy, 13:30", "Ayer, 20:00"). Enlace "Ver historial completo".
   - Textos del estado principal según situación: verde "Tus archivos están protegidos"; amarillo "La última copia terminó con avisos"; rojo "Tus archivos no se están copiando" + causa + botón para resolver; gris "El servicio de copias está parado".
2. **Copia en curso** (la misma pantalla de Inicio en estado azul)
   - Porcentaje grande, barra de progreso, tiempo restante, 4 cifras (archivos revisados, datos nuevos, velocidad, tiempo transcurrido) y archivo actual en una línea truncada.
   - Botón "Cancelar copia" (estilo peligroso).
   - Aviso informativo: "Puedes cerrar esta ventana: la copia continúa en segundo plano".
3. **Qué copiar**
   - "Carpetas habituales": 6 tarjetas‑casilla (Documentos, Escritorio, Imágenes, Descargas, Vídeos, Música) con su tamaño.
   - "Otras carpetas": lista con ruta, tamaño y "Quitar"; botón "Añadir carpeta" que abre el explorador de carpetas en un diálogo.
   - "No copiar": reglas predefinidas como interruptores con explicación; enlace "Añadir regla personalizada" (los patrones técnicos solo dentro de ese diálogo).
   - Barra de guardado al final.
4. **Restaurar**
   - Indicador de 3 pasos: Elige la fecha → Elige los archivos → Dónde guardarlos.
   - Columna izquierda: copias disponibles agrupadas (recientes, semanales, mensuales), la seleccionada destacada.
   - Columna derecha: buscador, ruta de migas de pan y tabla (casilla, icono por tipo, nombre, modificado, tamaño). Filas seleccionadas con fondo suave.
   - Barra inferior: "N archivos seleccionados · tamaño", opciones "Una carpeta nueva en el Escritorio (Recomendado)" / "Su ubicación original (sin reemplazar los archivos actuales)" y botón "Restaurar".
5. **Asistente de primera configuración**
   - Sin barra lateral. Logo arriba, barra de 5 pasos, una tarjeta centrada (máx. 680 px) por paso, botones Atrás / Continuar abajo.
   - Paso de contraseña: dos campos, indicador de fortaleza, comprobación de coincidencia, aviso amarillo de que no se puede recuperar, botón "Imprimir hoja de recuperación" y casilla obligatoria "He guardado la contraseña en un lugar seguro" (sin ella, Continuar desactivado).
6. **Historial, Destino, Programación, Ajustes**: no tienen diseño propio en el lienzo; aplicar exactamente los mismos componentes y estilo (tarjetas, filas, pastillas, barra de guardado).

---

## 6. Icono y menú de bandeja

- Icono: escudo con el punto de color del estado: verde (al día), ámbar (con avisos), rojo (falla o sin copia), azul (copiando, con animación sutil), gris (servicio parado). Versiones para fondo claro y oscuro de la barra de tareas, en 16, 20, 24 y 32 px.
- Al hacer clic: menú con cabecera de estado (icono + "Copias al día" + "Última: hoy 13:30 · Próxima: 20:00") y opciones: **Abrir panel** (en negrita), Copiar ahora, Ver historial, Restaurar archivos, separador, **Ocultar el icono** con la aclaración "Las copias siguen funcionando".
- Notificaciones de Windows con título claro y acción: "No se ha podido hacer la copia — El disco USB no está conectado" + botón "Abrir panel".

---

## 7. Detalles que marcan la diferencia

- Fechas relativas ("Hoy", "Ayer", "hace 3 días") y formatos españoles: `14 nov 2026`, `48,2 GB`, `9.812`.
- Tamaños siempre legibles (KB, MB, GB con una decimal).
- Estados de carga con esqueletos grises, no pantallas en blanco.
- Confirmaciones solo para acciones con riesgo (cancelar copia, restaurar a ubicación original, cambiar contraseña), con texto que explique la consecuencia.
- Al guardar, mensaje breve "Cambios guardados" que desaparece solo.
- Título de la pestaña del navegador con el estado, por ejemplo "BackupAMG · Al día" o "BackupAMG · Copiando 64 %".
- Favicon con el mismo escudo.

---

## 8. Fases

1. Tokens CSS, fuente e iconos locales, y componentes base (botones, tarjetas, pastillas, campos, avisos).
2. Barra lateral y estructura general.
3. Inicio (todos sus estados) y copia en curso.
4. Qué copiar, Destino, Programación, Retención y Verificación con barra de guardado.
5. Historial y detalle.
6. Restaurar.
7. Asistente.
8. Menú e iconos de la bandeja y notificaciones.
9. Modo oscuro, ventanas estrechas, teclado y revisión de textos.

Al terminar cada fase, adjunta captura de cómo ha quedado y compárala con el diseño de referencia.

---

## 9. Cómo debe trabajar el asistente

- No modifiques lógica ni API; si una pantalla necesita un dato que la API no da, **pregunta antes**.
- Todos los textos en el fichero central de textos, revisados para que suenen naturales en español.
- Ninguna dependencia cargada desde internet: todo embebido.
- Si algo del diseño no encaja con la funcionalidad existente, propón la solución más simple y coherente con este documento.
