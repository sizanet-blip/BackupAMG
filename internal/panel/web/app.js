let csrf = "";
let rol = "user";

function texto(id, valor) {
  const nodo = document.getElementById(id);
  if (nodo) nodo.textContent = valor;
}

function fecha(iso) {
  if (!iso) return textos.nunca;
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return iso;
  return new Intl.DateTimeFormat("es", { dateStyle: "medium", timeStyle: "short" }).format(momento);
}

function tamano(n) {
  if (n == null || !Number.isFinite(n)) return textos.desconocido;
  const unidades = ["B", "KB", "MB", "GB", "TB"];
  let valor = n;
  let i = 0;
  while (valor >= 1024 && i < unidades.length - 1) {
    valor /= 1024;
    i++;
  }
  const cifras = i === 0 ? 0 : 1;
  return valor.toLocaleString("es", { minimumFractionDigits: cifras, maximumFractionDigits: cifras }) + " " + unidades[i];
}

function horaEnFrase(iso) {
  if (!iso) return textos.nunca;
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return iso;
  const dia = (fechaDia) => new Date(fechaDia.getFullYear(), fechaDia.getMonth(), fechaDia.getDate()).getTime();
  const dias = Math.round((dia(new Date()) - dia(momento)) / 86400000);
  const hora = new Intl.DateTimeFormat("es", { hour: "2-digit", minute: "2-digit" }).format(momento);
  if (dias === 0) return "hoy a las " + hora;
  if (dias === 1) return "ayer a las " + hora;
  if (dias > 1 && dias < 7) return "hace " + dias + " días";
  return new Intl.DateTimeFormat("es", { day: "numeric", month: "short" }).format(momento);
}

function nombreDestino(destino) {
  const ruta = destino.path || "";
  if (!ruta) return textos.desconocido;
  if (ruta.startsWith("\\\\")) return ruta;
  const letra = /^[A-Za-z]:/.test(ruta) ? ruta.slice(0, 2).toUpperCase() : "";
  const tipo = destino.kind === "removable" ? textos.discoUsb : (destino.kind === "remote" ? textos.unidadRed : textos.discoLocal);
  if (!letra) return ruta;
  return destino.label ? tipo + " «" + destino.label + "» (" + letra + ")" : tipo + " (" + letra + ")";
}

function numeroCarpetas(n) {
  return n === 1 ? textos.carpetasUna : textos.carpetasNumero.replace("{n}", String(n));
}

function nombreCarpeta(ruta) {
  const partes = String(ruta || "").split(/\\/).filter(Boolean);
  return partes[partes.length - 1] || ruta;
}

function fraseActividad(item) {
  const disparo = textos.disparosLista[item.trigger] || "";
  let base = item.type === "verify" ? textos.verificacionLista : (disparo || textos.tipos.backup);
  if (item.status === "warning" && item.skipped) {
    base += " · " + item.skipped + " " + (item.skipped === 1 ? "archivo no se pudo leer" : "archivos no se pudieron leer");
  } else if (item.status === "failed") {
    const mensaje = (item.message || "").toLowerCase();
    if (mensaje.includes("destino") || mensaje.includes("usb") || mensaje.includes("conect")) base += " · el disco no estaba conectado";
    else if (item.message) base += " · " + item.message;
  }
  return base;
}

function fechaRelativa(iso) {
  if (!iso) return textos.nunca;
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return iso;
  const dia = (fechaDia) => new Date(fechaDia.getFullYear(), fechaDia.getMonth(), fechaDia.getDate()).getTime();
  const dias = Math.round((dia(new Date()) - dia(momento)) / 86400000);
  const hora = new Intl.DateTimeFormat("es", { hour: "2-digit", minute: "2-digit" }).format(momento);
  if (dias === 0) return "Hoy, " + hora;
  if (dias === 1) return "Ayer, " + hora;
  if (dias > 1 && dias < 7) return "Hace " + dias + " días";
  return new Intl.DateTimeFormat("es", { day: "numeric", month: "short", year: "numeric" }).format(momento);
}

function duracion(segundos) {
  if (!Number.isFinite(segundos) || segundos < 0) return textos.desconocido;
  const total = Math.round(segundos);
  const minutos = Math.floor(total / 60);
  const resto = total % 60;
  if (minutos <= 0) return resto + " s";
  return minutos + " min " + resto + " s";
}

function resultado(run) {
  if (!run || !run.finished_at) return textos.nunca;
  const nombre = textos.estados[run.status] || run.status;
  let linea = fecha(run.finished_at) + " · " + nombre;
  if (run.duration_seconds) linea += " · " + textos.duracion + " " + duracion(run.duration_seconds);
  if (run.data_added_bytes) linea += " · " + textos.anadidos + " " + tamano(run.data_added_bytes);
  return linea;
}

let actividadAnterior = "";

function fraseEstado(estado) {
  if (estado.activity === "backup" || estado.activity === "restore") return textos.frases.copiando;
  if (estado.activity === "verify") return textos.frases.comprobando;
  if (estado.level === "green") return textos.frases.protegidos;
  if (estado.level === "yellow") return textos.frases.avisos;
  if (estado.level === "red") return textos.frases.atencion;
  return textos.frases.parado;
}

function pintar(estado) {
  document.getElementById("pantalla-inicio").classList.remove("esqueleto");
  const nivel = estado.level || "red";
  const enCurso = estado.activity === "backup" || estado.activity === "restore" || estado.activity === "verify";
  const luz = document.getElementById("semaforo");
  luz.className = "escudo " + (enCurso ? "blue" : nivel);
  const pastilla = document.getElementById("pastilla");
  const clases = { green: "ok", yellow: "aviso", red: "error", blue: "curso" };
  pastilla.className = "pastilla " + (enCurso ? "curso" : (clases[nivel] || ""));
  pastilla.textContent = enCurso ? textos.pastillas.curso : (textos.pastillas[nivel] || textos.pastillas.red);
  texto("nivel", fraseEstado(estado));
  if (estado.agent_name) texto("equipo", estado.agent_name);
  const ultima = estado.last_backup;
  const lineaUltima = document.getElementById("linea-ultima");
  const lineaProxima = document.getElementById("linea-proxima");
  lineaUltima.hidden = false;
  lineaProxima.hidden = enCurso;
  if (enCurso && estado.activity === "backup") {
    lineaUltima.textContent = fraseCurso(estado);
  } else if (ultima && ultima.finished_at) {
    lineaUltima.textContent = "Última copia " + horaEnFrase(ultima.finished_at) + " · " + tamano(ultima.data_added_bytes) + " nuevos · " + duracion(ultima.duration_seconds);
  } else {
    lineaUltima.textContent = textos.nunca;
  }
  lineaProxima.textContent = "Próxima copia: " + horaEnFrase(estado.next_backup);
  document.title = "BackupAMG · " + (enCurso && estado.progress ? textos.frases.copiandoCorto + " " + Math.round(estado.progress.percent) + " %" : pastilla.textContent);

  const destino = estado.destination || {};
  texto("destino-nombre", nombreDestino(destino));
  const estadoDestino = document.getElementById("destino-estado");
  estadoDestino.textContent = destino.connected ? textos.conectado : textos.desconectado;
  estadoDestino.className = "punto " + (destino.connected ? "ok" : "error");
  const total = destino.total_bytes || 0;
  const usado = total > destino.free_bytes ? total - destino.free_bytes : 0;
  const ancho = destino.connected && total > 0 ? Math.max(2, Math.min(100, usado / total * 100)) : 0;
  const disco = document.getElementById("barra-disco");
  disco.style.width = ancho + "%";
  disco.parentElement.hidden = enCurso;
  texto("destino-espacio", destino.connected && total > 0 ? (enCurso ? tamano(destino.free_bytes) + " libres" : tamano(destino.free_bytes) + " libres de " + tamano(total)) : textos.destinoAusente);
  const tarjetas = document.getElementById("tarjetas");
  tarjetas.classList.toggle("ocupado", enCurso);
  document.querySelector(".actividad").hidden = enCurso;
  if (enCurso) {
    texto("etiq-verificacion", textos.ultimaTerminada);
    if (ultima && ultima.finished_at) {
      texto("verificacion", horaEnFrase(ultima.finished_at) + " · " + (textos.estados[ultima.status] || "").toLowerCase());
      texto("verificacion-cuando", tamano(ultima.data_added_bytes) + " nuevos · " + duracion(ultima.duration_seconds));
      document.getElementById("verificacion-estado").textContent = "";
      document.getElementById("verificacion-estado").className = "punto";
    }
  } else {
    texto("etiq-verificacion", textos.verificacion);
    pintarVerificacion(estado.last_verify);
  }
  texto("servicio", textos.servicioActivo);

  const caja = document.getElementById("progreso");
  caja.hidden = !enCurso;
  document.getElementById("aviso-curso").hidden = !enCurso;
  if (enCurso && estado.progress) {
    const p = estado.progress;
    document.getElementById("avance").style.width = Math.max(0, Math.min(100, p.percent)) + "%";
    texto("porcentaje", Math.round(p.percent) + " %");
    texto("ficheros", Number(p.files_done || 0).toLocaleString("es") + " de " + Number(p.files_total || 0).toLocaleString("es"));
    texto("anadidos", tamano(p.bytes_done));
    texto("velocidad", tamano(p.bytes_per_second) + "/s");
    texto("queda", textos.queda.replace("{tiempo}", quedaEnFrase(p.seconds_remaining)));
    texto("transcurrido", duracion(p.seconds_elapsed));
    texto("actual", p.current_file || "");
  }
  if (estado.activity === "verify") {
    texto("porcentaje", textos.comprobando);
    document.getElementById("avance").style.width = "100%";
  }

  const ocupado = enCurso;
  document.getElementById("copiar").hidden = ocupado;
  document.getElementById("verificar").hidden = ocupado || rol !== "admin";
  document.getElementById("cancelar").hidden = !ocupado;
  if (estado.problem) mostrarAviso(estado.problem);
  if (actividadAnterior && !enCurso) cargarRecientes();
  actividadAnterior = estado.activity || "";
}

async function cargarRecientes() {
  const caja = document.getElementById("recientes");
  if (!caja) return;
  const respuesta = await fetch("/api/v1/history");
  caja.replaceChildren();
  if (!respuesta.ok) return;
  const items = await respuesta.json();
  if (!items || !items.length) {
    const vacio = document.createElement("p");
    vacio.className = "nota";
    vacio.textContent = textos.sinActividad;
    caja.append(vacio);
    return;
  }
  items.slice(0, 5).forEach((item) => {
    const fila = document.createElement("div");
    fila.className = "fila";
    const pastilla = document.createElement("span");
    const clase = item.status === "success" ? "ok" : item.status === "warning" ? "aviso" : "error";
    pastilla.className = "pastilla " + clase;
    pastilla.textContent = textos.estados[item.status] || item.status;
    const textoFila = document.createElement("span");
    textoFila.className = "actividad-texto";
    textoFila.textContent = fraseActividad(item);
    const medida = document.createElement("span");
    medida.className = "nota";
    medida.textContent = (item.type === "verify" ? "" : tamano(item.data_added_bytes) + " · ") + duracion(item.duration_seconds);
    const cuando = document.createElement("span");
    cuando.className = "nota";
    cuando.textContent = fechaRelativa(item.finished_at);
    fila.append(pastilla, textoFila, medida, cuando);
    caja.append(fila);
  });
}

function fraseCurso(estado) {
  const tarea = estado.job || {};
  const tipo = textos.disparosLista[tarea.trigger] || textos.frases.copiandoCorto;
  let linea = tipo;
  if (tarea.started_at) linea += " iniciada " + horaEnFrase(tarea.started_at);
  if (tarea.use_vss) linea += " · usando instantánea de Windows para los archivos abiertos";
  return linea;
}

function quedaEnFrase(segundos) {
  const n = Number(segundos);
  if (!Number.isFinite(n) || n <= 0) return "un momento";
  if (n < 90) return "unos segundos";
  const minutos = Math.max(1, Math.round(n / 60));
  return minutos === 1 ? "1 minuto" : minutos + " minutos";
}

function pintarVerificacion(run) {
  const estado = document.getElementById("verificacion-estado");
  const boton = document.getElementById("verificacion-real");
  const detalle = document.getElementById("verificacion-detalle");
  const real = run && run.detail && run.detail !== run.message ? run.detail : "";
  const abierto = real !== "" && !detalle.hidden && detalle.textContent === real;
  boton.hidden = real === "";
  detalle.hidden = !abierto;
  detalle.textContent = real;
  boton.textContent = abierto ? textos.ocultarInformacion : textos.verInformacion;
  boton.onclick = () => {
    detalle.hidden = !detalle.hidden;
    boton.textContent = detalle.hidden ? textos.verInformacion : textos.ocultarInformacion;
  };
  if (!run || !run.finished_at) {
    texto("verificacion", textos.verificacionNunca);
    texto("verificacion-cuando", "");
    estado.textContent = "";
    estado.className = "punto";
    boton.hidden = true;
    detalle.hidden = true;
    return;
  }
  if (run.status === "success") {
    texto("verificacion", textos.verificacionBien);
    estado.textContent = textos.sinErrores;
    estado.className = "punto ok";
    boton.hidden = true;
    detalle.hidden = true;
  } else if (run.status === "warning") {
    texto("verificacion", textos.verificacionAvisos);
    estado.textContent = textos.conAvisos;
    estado.className = "punto aviso";
  } else {
    texto("verificacion", textos.verificacionMal);
    estado.textContent = run.message || textos.estados.failed;
    estado.className = "punto error";
  }
  texto("verificacion-cuando", horaEnFrase(run.finished_at));
}

function mostrarAviso(mensaje) {
  const aviso = document.getElementById("aviso");
  aviso.hidden = !mensaje;
  aviso.textContent = mensaje || "";
}

async function accion(ruta) {
  if (ruta.endsWith("/cancel") && !(await confirmarSiNo({
    titulo: textos.cancelarCopiaTitulo,
    texto: textos.confirmarCancelar,
    aceptar: textos.cancelarCopiaBoton,
    cancelar: textos.seguirCopiando,
    peligro: true
  }))) return;
  mostrarAviso("");
  const botones = ["copiar", "verificar"].map((id) => document.getElementById(id));
  botones.forEach((boton) => { boton.disabled = true; });
  const respuesta = await fetch(ruta, {
    method: "POST",
    headers: { "X-CSRF-Token": csrf }
  });
  const cuerpo = await respuesta.json().catch(() => ({}));
  botones.forEach((boton) => { boton.disabled = false; });
  if (!respuesta.ok) {
    mostrarAviso(cuerpo.error || textos.error);
    return;
  }
  if (ruta.endsWith("/backup")) mostrarToast(textos.copiaIniciada, true);
  if (ruta.endsWith("/verify")) mostrarToast(textos.comprobacionIniciada, true);
  const estado = await fetch("/api/v1/status");
  if (estado.ok) pintar(await estado.json());
}

async function cargar() {
  document.getElementById("pantalla-inicio").classList.add("esqueleto");
  texto("salir", textos.ocultarSesion);
  texto("titulo-inicio", textos.menu.inicio);
  texto("ayuda-inicio", textos.ayudaInicio);
  texto("etiq-destino", textos.destino);
  texto("etiq-seleccion", textos.queSeCopia);
  texto("etiq-verificacion", textos.verificacion);
  texto("etiq-reciente", textos.reciente);
  texto("ver-historial", textos.verHistorial);
  texto("cambiar-seleccion", textos.cambiarSeleccion);
  texto("etiq-ficheros", textos.ficheros);
  texto("etiq-anadidos", textos.datosCopiados);
  texto("etiq-velocidad", textos.velocidad);
  texto("etiq-transcurrido", textos.transcurrido);
  texto("etiq-actual", textos.ahoraMismo);
  texto("aviso-curso", textos.avisoCurso);
  document.querySelector("#copiar span").textContent = textos.copiar;
  texto("verificar", textos.verificar);
  texto("cancelar", textos.cancelar);
  texto("servicio", textos.servicioActivo);

  const sesion = await fetch("/api/v1/session");
  if (!sesion.ok) {
    texto("nivel", textos.error);
    return;
  }
  const datos = await sesion.json();
  csrf = datos.csrf;
  rol = datos.role;
  document.getElementById("verificar").hidden = rol !== "admin";
  document.getElementById("cambiar-seleccion").hidden = rol !== "admin";
  const preparado = await fetch("/api/v1/setup");
  if (preparado.ok) {
    const info = await preparado.json();
    if (info.hostname) texto("equipo", info.hostname);
    if (info.needed) {
      if (rol !== "admin") {
        texto("nivel", textos.asistentePendiente);
        return;
      }
      document.body.classList.add("con-asistente");
      document.querySelector("main").hidden = true;
      pintarAsistente(info);
      return;
    }
  }
  iniciarMenu(rol === "admin");
  document.getElementById("salir").addEventListener("click", async () => {
    await fetch("/api/v1/logout", { method: "POST", headers: { "X-CSRF-Token": csrf } });
    location.href = "/";
  });
  document.getElementById("copiar").addEventListener("click", () => accion("/api/v1/backup"));
  document.getElementById("verificar").addEventListener("click", () => accion("/api/v1/verify"));
  document.getElementById("cancelar").addEventListener("click", () => accion("/api/v1/cancel"));
  document.getElementById("ver-historial").addEventListener("click", () => mostrarPantalla("historial"));
  document.getElementById("cambiar-seleccion").addEventListener("click", () => mostrarPantalla("copiar"));
  if (rol === "admin") {
    const configuracion = await fetch("/api/v1/config");
    if (configuracion.ok) {
      const cfg = await configuracion.json();
      const fuentes = cfg.sources || [];
      texto("seleccion-texto", numeroCarpetas(fuentes.length));
      texto("seleccion-nombres", fuentes.map(nombreCarpeta).join(", "));
      const peso = await fetch("/api/v1/size", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({ sources: fuentes })
      });
      if (peso.ok) {
        const cuerpo = await peso.json();
        if (cuerpo.bytes) texto("seleccion-texto", numeroCarpetas(fuentes.length) + " · " + tamano(cuerpo.bytes));
      }
    }
  } else {
    texto("seleccion-texto", textos.seleccionUsuario);
  }
  cargarRecientes();

  const estado = await fetch("/api/v1/status");
  if (estado.ok) pintar(await estado.json());
  else {
    document.getElementById("pantalla-inicio").classList.remove("esqueleto");
    texto("nivel", textos.error);
  }

  const fuente = new EventSource("/api/v1/events");
  // Si el canal en directo se corta, comprobar la sesión: si ha caducado, se avisa una sola vez.
  fuente.onerror = () => { fetch("/api/v1/session").catch(() => {}); };
  fuente.onmessage = (evento) => {
    try {
      const data = JSON.parse(evento.data);
      if (data && data.navigate) {
        if (typeof mostrarPantalla === "function") mostrarPantalla(data.navigate);
        try { window.focus(); } catch (e) { /* el navegador puede bloquear el foco */ }
        if (document.hidden && typeof Notification === "function" && Notification.permission === "granted") {
          const aviso = new Notification("BackupAMG", { body: "El panel se ha actualizado. Pulsa para verlo.", tag: "backupamg-panel" });
          aviso.onclick = () => { try { window.focus(); } catch (e) {} aviso.close(); };
        }
        return;
      }
      pintar(data);
    } catch (e) { /* un aviso suelto no pinta el estado */ }
  };
  if (typeof Notification === "function" && Notification.permission === "default") {
    try { Notification.requestPermission(); } catch (e) { /* opcional */ }
  }
}

cargar();
