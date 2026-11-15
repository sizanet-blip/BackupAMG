const iconoDisco = iconoSvg("hard-drive");
const iconoCarpeta = iconoSvg("folder");
const iconoReloj = iconoSvg("clock");

// Título grande de cada paso, con su frase de ayuda.
function tituloPaso(titulo, ayuda) {
  const h = document.createElement("h2");
  h.textContent = titulo;
  const trozos = [h];
  if (ayuda) {
    const p = document.createElement("p");
    p.className = "nota";
    p.textContent = ayuda;
    trozos.push(p);
  }
  return trozos;
}

// Sección con título pequeño y, si hace falta, una línea de ayuda.
function seccionAsistente(titulo, ayuda) {
  const seccion = document.createElement("section");
  seccion.className = "seccion";
  const h = document.createElement("h2");
  h.textContent = titulo;
  seccion.append(h);
  if (ayuda) {
    const p = document.createElement("p");
    p.className = "nota";
    p.textContent = ayuda;
    seccion.append(p);
  }
  return seccion;
}

function filaIcono(icono, texto, clase) {
  const fila = document.createElement("div");
  fila.className = "fila-carpeta" + (clase ? " " + clase : "");
  const caja = document.createElement("span");
  caja.className = "fila-carpeta-icono";
  caja.innerHTML = icono;
  const ruta = document.createElement("span");
  ruta.className = "fila-carpeta-ruta";
  ruta.textContent = texto;
  fila.append(caja, ruta);
  return fila;
}

function pintarAsistente(info) {
  const raiz = document.getElementById("pantalla-asistente");
  raiz.hidden = false;
  const CLAVE_BORRADOR = "backupamg-asistente";
  const valoresIniciales = () => ({
    paso: 0,
    path: "",
    probado: "",
    sources: [],
    excludes: ["~$*", "*.tmp"],
    times: ["13:30"],
    weekdays: [1, 2, 3, 4, 5, 6, 7],
    password: "",
    confirm: "",
    ack: false
  });
  const datos = valoresIniciales();
  // El borrador vive solo en esta pestaña. Nunca guarda la contraseña ni nada secreto.
  function guardarBorrador() {
    try {
      sessionStorage.setItem(CLAVE_BORRADOR, JSON.stringify({
        paso: datos.paso, path: datos.path, sources: datos.sources,
        excludes: datos.excludes, times: datos.times, weekdays: datos.weekdays
      }));
    } catch (e) { /* sin almacenamiento: el asistente funciona igual */ }
  }
  function borrarBorrador() {
    try { sessionStorage.removeItem(CLAVE_BORRADOR); } catch (e) { /* nada que borrar */ }
  }
  let borradorRecuperado = false;
  try {
    const guardado = JSON.parse(sessionStorage.getItem(CLAVE_BORRADOR) || "null");
    if (guardado && (guardado.paso > 0 || guardado.path || (guardado.sources || []).length)) {
      datos.path = String(guardado.path || "");
      datos.sources = Array.isArray(guardado.sources) ? guardado.sources.map(String) : [];
      if (Array.isArray(guardado.excludes)) datos.excludes = guardado.excludes.map(String);
      if (Array.isArray(guardado.times)) datos.times = guardado.times.map(String);
      if (Array.isArray(guardado.weekdays)) datos.weekdays = guardado.weekdays.map(Number);
      // Sin la contraseña no se puede pasar del paso 2: se vuelve a ella.
      datos.paso = Math.min(Number(guardado.paso) || 0, 1);
      borradorRecuperado = true;
    }
  } catch (e) { /* borrador ilegible: se ignora */ }
  // El selector del paso 3 se conserva entre pasos: recuerda por dónde ibas y no se cierra al añadir.
  let exploradorCopiar = null;
  let exploradorAbierto = false;
  dibujar();

  function dibujar() {
    guardarBorrador();
    raiz.replaceChildren();
    const marca = document.createElement("div");
    marca.className = "marca-centro";
    const imagen = document.createElement("img");
    imagen.src = "/marca-completa.svg";
    imagen.className = "marca-completa";
    imagen.height = 84;
    imagen.alt = textos.marca;
    marca.append(imagen);
    const pista = document.createElement("div");
    pista.className = "pasos-track";
    const nombres = document.createElement("div");
    nombres.className = "pasos-nombres";
    textos.pasos.forEach((nombre, i) => {
      const tramo = document.createElement("span");
      const etiq = document.createElement("span");
      etiq.textContent = (i + 1) + " · " + nombre;
      if (i < datos.paso) {
        tramo.className = "hecho";
        etiq.className = "hecho";
      } else if (i === datos.paso) {
        tramo.className = "activo";
        etiq.className = "activo";
      }
      pista.append(tramo);
      nombres.append(etiq);
    });
    const cuerpo = document.createElement("div");
    const aviso = document.createElement("p");
    aviso.className = "resultado";
    raiz.append(marca, pista, nombres);
    if (borradorRecuperado) {
      borradorRecuperado = false;
      const caja = document.createElement("div");
      caja.className = "caja-info";
      const texto = document.createElement("p");
      texto.textContent = textos.borradorRecuperado;
      const descartar = document.createElement("button");
      descartar.type = "button";
      descartar.className = "secundario";
      descartar.textContent = textos.borradorDescartar;
      descartar.addEventListener("click", () => {
        borrarBorrador();
        Object.assign(datos, valoresIniciales());
        exploradorCopiar = null;
        exploradorAbierto = false;
        dibujar();
      });
      caja.append(texto, descartar);
      raiz.append(caja);
    }
    raiz.append(cuerpo, aviso);
    if (datos.paso === 0) pasoDestino(cuerpo, aviso);
    if (datos.paso === 1) pasoClave(cuerpo, aviso);
    if (datos.paso === 2) pasoCopiar(cuerpo, aviso);
    if (datos.paso === 3) pasoCuando(cuerpo, aviso);
    if (datos.paso === 4) pasoResumen(cuerpo, aviso);
  }

  function pasoDestino(cuerpo, aviso) {
    const ayuda = document.createElement("p");
    ayuda.className = "nota";
    ayuda.textContent = textos.ayudaDisco;
    const redAyuda = document.createElement("p");
    redAyuda.className = "nota";
    redAyuda.textContent = textos.ayudaRed;
    const campo = document.createElement("label");
    campo.className = "campo-columna";
    campo.append(document.createTextNode(textos.rutaDestino));
    const ruta = document.createElement("input");
    ruta.type = "text";
    ruta.value = datos.path;
    campo.append(ruta);
    const falta = notaFalta();
    let probando = false;
    const explorador = crearExplorador({ modo: "elegir", alElegir: (carpeta) => {
      ruta.value = carpeta;
      datos.probado = "";
      aviso.textContent = "";
      actualizar();
    } });
    const explorar = explorador.raiz;
    const probar = document.createElement("button");
    probar.type = "button";
    probar.className = "secundario";
    probar.textContent = textos.probar;
    const siguiente = document.createElement("button");
    siguiente.type = "button";
    siguiente.textContent = textos.siguiente;
    const pie = document.createElement("div");
    pie.className = "pie-paso";
    pie.append(probar, falta, siguiente);
    cuerpo.append(...tituloPaso(textos.eligeDestino), ayuda, redAyuda, campo, explorar, pie);
    explorador.cargarSiHaceFalta();
    const revisarSiguiente = exigir(siguiente, falta, () => {
      const actual = ruta.value.trim();
      if (!actual) return textos.faltaDestino;
      if (datos.probado !== actual) return textos.faltaProbar;
      return "";
    });
    function actualizar() {
      revisarSiguiente();
      probar.disabled = probando || !ruta.value.trim();
    }
    ruta.addEventListener("input", () => {
      datos.path = ruta.value.trim();
      guardarBorrador();
      if (datos.probado && ruta.value.trim() !== datos.probado) {
        datos.probado = "";
        aviso.textContent = "";
      }
      actualizar();
    });
    actualizar();
    probar.addEventListener("click", async () => {
      datos.path = ruta.value.trim();
      datos.probado = "";
      probando = true;
      actualizar();
      aviso.className = "resultado";
      aviso.textContent = textos.probando;
      const respuesta = await fetch("/api/v1/destination/test", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({ path: datos.path })
      });
      const cuerpoRes = await respuesta.json().catch(() => ({}));
      probando = false;
      const detalle = cuerpoRes.message || cuerpoRes.error || textos.error;
      aviso.className = cuerpoRes.ok ? "resultado aviso-ok" : "resultado aviso";
      aviso.textContent = cuerpoRes.ok ? textos.destinoCorrecto + ". " + detalle : detalle + " " + textos.revisaDestino;
      if (cuerpoRes.ok) datos.probado = datos.path;
      actualizar();
    });
    siguiente.addEventListener("click", () => {
      datos.path = ruta.value.trim();
      if (!datos.path || datos.probado !== datos.path) {
        aviso.className = "resultado aviso";
        aviso.textContent = textos.faltaProbar;
        return;
      }
      datos.paso = 1;
      dibujar();
    });
  }

  function pasoClave(cuerpo, aviso) {
    const titulo = document.createElement("h2");
    titulo.textContent = textos.creaClave;
    const ayuda = document.createElement("p");
    ayuda.className = "nota";
    ayuda.textContent = textos.creaClaveAyuda;
    const nueva = secreto(textos.nuevaClave, datos.password);
    const fuerza = document.createElement("div");
    fuerza.className = "fuerza";
    for (let i = 0; i < 4; i++) fuerza.append(document.createElement("i"));
    const nivel = document.createElement("em");
    fuerza.append(nivel);
    const repite = secreto(textos.repiteClave, datos.confirm);
    const coinciden = document.createElement("p");
    coinciden.className = "nota";
    const peligro = document.createElement("div");
    peligro.className = "caja-aviso";
    const peligroTitulo = document.createElement("strong");
    peligroTitulo.textContent = textos.avisoClaveTitulo;
    const peligroCuerpo = document.createElement("p");
    peligroCuerpo.textContent = textos.avisoClaveCuerpo;
    peligro.append(peligroTitulo, peligroCuerpo);
    const hoja = document.createElement("div");
    hoja.className = "hoja-caja";
    const hojaTitulo = document.createElement("p");
    hojaTitulo.textContent = textos.hojaTitulo;
    const descargar = document.createElement("button");
    descargar.type = "button";
    descargar.className = "secundario";
    descargar.textContent = textos.hoja;
    const imprimir = document.createElement("button");
    imprimir.type = "button";
    imprimir.className = "secundario";
    imprimir.textContent = textos.imprimir;
    const accionesHoja = document.createElement("div");
    accionesHoja.className = "barra-filtros";
    accionesHoja.append(imprimir, descargar);
    hoja.append(hojaTitulo, accionesHoja);
    const casilla = document.createElement("label");
    const marca = document.createElement("input");
    marca.type = "checkbox";
    marca.checked = datos.ack;
    casilla.append(marca, document.createTextNode(textos.guardadaClave));
    const pie = document.createElement("div");
    pie.className = "pie-paso";
    const atras = botonAtras();
    const siguiente = document.createElement("button");
    siguiente.type = "button";
    siguiente.textContent = textos.continuar;
    const falta = notaFalta();
    pie.append(atras, falta, siguiente);
    cuerpo.append(titulo, ayuda, nueva.etiqueta, fuerza, repite.etiqueta, coinciden, peligro, hoja, casilla, pie);
    const revisarSiguiente = exigir(siguiente, falta, () => {
      if (!nueva.input.value) return textos.faltaClave;
      if (nueva.input.value !== repite.input.value) return textos.noCoinciden + ".";
      if (!marca.checked) return textos.faltaMarca;
      return "";
    });
    marca.addEventListener("change", revisarSiguiente);
    function revisar() {
      const clave = nueva.input.value;
      const n = fuerzaClave(clave);
      fuerza.className = "fuerza n" + n;
      nivel.textContent = clave ? textos.fuerzaClave[n] : "";
      if (!repite.input.value) {
        coinciden.textContent = "";
        return;
      }
      const igual = clave === repite.input.value;
      coinciden.textContent = igual ? "✓ " + textos.coinciden : textos.noCoinciden;
      coinciden.className = igual ? "nota aviso-ok" : "nota aviso";
    }
    nueva.input.addEventListener("input", () => { revisar(); revisarSiguiente(); });
    repite.input.addEventListener("input", () => { revisar(); revisarSiguiente(); });
    revisar();
    function fuerzaClave(clave) {
      let n = 0;
      if (clave.length >= 8) n++;
      if (clave.length >= 12) n++;
      if (/[a-z]/.test(clave) && /[A-Z]/.test(clave)) n++;
      if (/\d/.test(clave) && /[^A-Za-z0-9]/.test(clave)) n++;
      return n;
    }
    descargar.addEventListener("click", () => descargarHoja(info.hostname, datos.path));
    imprimir.addEventListener("click", () => imprimirHoja(info.hostname, datos.path));
    siguiente.addEventListener("click", () => {
      datos.password = nueva.input.value;
      datos.confirm = repite.input.value;
      datos.ack = marca.checked;
      if (!datos.password || datos.password !== datos.confirm || !datos.ack) {
        aviso.textContent = textos.avisoClave;
        return;
      }
      datos.paso = 2;
      dibujar();
    });
  }

  function pasoCopiar(cuerpo, aviso) {
    const titulo = tituloPaso(textos.eligeCopiar, textos.eligeCopiarAyuda);

    const habituales = seccionAsistente(textos.habituales);
    const quien = document.createElement("p");
    quien.className = "nota";
    const mosaico = document.createElement("div");
    mosaico.className = "mosaico";
    habituales.append(quien, mosaico);

    const otras = seccionAsistente(textos.otrasCarpetas, textos.otrasAyuda);
    const lista = document.createElement("div");
    lista.className = "lista";
    const anadirOtra = document.createElement("button");
    anadirOtra.type = "button";
    anadirOtra.className = "secundario";
    anadirOtra.textContent = "+ " + textos.anadirOtra;
    if (!exploradorCopiar) {
      exploradorCopiar = crearExplorador({
        modo: "anadir",
        alElegir: (carpeta) => {
          if (!datos.sources.includes(carpeta)) datos.sources.push(carpeta);
          dibujar();
        },
        elegidas: () => datos.sources,
        alQuitar: (ruta) => {
          datos.sources = datos.sources.filter((item) => item !== ruta);
          dibujar();
        },
        alCerrar: () => {
          exploradorAbierto = false;
          exploradorCopiar.raiz.hidden = true;
        }
      });
    }
    const explorador = exploradorCopiar;
    const explorar = explorador.raiz;
    explorar.hidden = !exploradorAbierto;
    if (exploradorAbierto) explorador.refrescar();
    anadirOtra.addEventListener("click", () => {
      exploradorAbierto = true;
      explorar.hidden = false;
      explorador.cargarSiHaceFalta();
      explorador.refrescar();
    });
    otras.append(lista, anadirOtra, explorar);

    fetch("/api/v1/shortcuts").then(async (respuesta) => {
      const items = respuesta.ok ? ((await respuesta.json()) || []) : [];
      const rutas = new Set(items.map((item) => item.path));
      const usuarios = Array.from(new Set(items.map((item) => item.user).filter(Boolean)));
      quien.textContent = usuarios.length === 1 ? textos.delUsuario.replace("{nombre}", usuarios[0]) : "";
      items.forEach((item) => {
        const marcada = datos.sources.includes(item.path);
        const tarjeta = tarjetaCarpeta(item, marcada, () => {
          datos.sources = marcada ? datos.sources.filter((ruta) => ruta !== item.path) : datos.sources.concat([item.path]);
          dibujar();
        });
        mosaico.append(tarjeta.boton);
        pedirTamano(item.path, tarjeta.peso);
      });
      datos.sources.filter((ruta) => !rutas.has(ruta)).forEach((ruta) => {
        const fila = filaIcono(iconoCarpeta, ruta);
        const quitar = document.createElement("button");
        quitar.type = "button";
        quitar.className = "enlace";
        quitar.textContent = textos.quitar;
        quitar.addEventListener("click", () => {
          datos.sources = datos.sources.filter((item) => item !== ruta);
          dibujar();
        });
        fila.append(quitar);
        lista.append(fila);
      });
    });

    const noCopiar = seccionAsistente(textos.noCopiar, textos.noCopiarAyuda);
    const exclusiones = document.createElement("div");
    exclusiones.className = "plantillas";
    textos.plantillas.forEach((plantilla) => {
      exclusiones.append(filaRegla(plantilla, datos.excludes.includes(plantilla.patron), (activa) => {
        if (activa && !datos.excludes.includes(plantilla.patron)) datos.excludes.push(plantilla.patron);
        if (!activa) datos.excludes = datos.excludes.filter((item) => item !== plantilla.patron);
      }));
    });
    noCopiar.append(exclusiones);

    const siguiente = botonSiguiente(() => {
      if (!datos.sources.length) {
        aviso.textContent = textos.sinCarpetas;
        return false;
      }
      return true;
    });
    const falta = notaFalta();
    exigir(siguiente, falta, () => (datos.sources.length ? "" : textos.sinCarpetas));
    cuerpo.append(...titulo, habituales, otras, noCopiar, pieAtras(siguiente, falta));
  }

  function pasoCuando(cuerpo, aviso) {
    const titulo = tituloPaso(textos.eligeCuando, textos.eligeCuandoAyuda);
    const horasSec = seccionAsistente(textos.horas);
    const horas = document.createElement("div");
    horas.className = "lista";
    datos.times.forEach((hora) => {
      const fila = document.createElement("div");
      fila.className = "fila-hora";
      const icono = document.createElement("span");
      icono.className = "fila-carpeta-icono";
      icono.innerHTML = iconoReloj;
      const texto = document.createElement("strong");
      texto.textContent = hora;
      const quitar = document.createElement("button");
      quitar.type = "button";
      quitar.className = "enlace";
      quitar.textContent = textos.quitar;
      quitar.addEventListener("click", () => {
        datos.times = datos.times.filter((item) => item !== hora);
        dibujar();
      });
      fila.append(icono, texto, quitar);
      horas.append(fila);
    });
    const nueva = document.createElement("input");
    nueva.type = "text";
    nueva.placeholder = "13:30";
    nueva.setAttribute("aria-label", textos.horas);
    const anadir = document.createElement("button");
    anadir.type = "button";
    anadir.className = "secundario";
    anadir.textContent = textos.anadirHora;
    anadir.addEventListener("click", () => {
      const hora = nueva.value.trim();
      if (!/^\d{2}:\d{2}$/.test(hora)) {
        aviso.textContent = textos.sinHora;
        return;
      }
      if (!datos.times.includes(hora)) datos.times.push(hora);
      dibujar();
    });
    const entrada = document.createElement("div");
    entrada.className = "barra-filtros";
    entrada.append(nueva, anadir);
    horasSec.append(horas, entrada);

    const diasSec = seccionAsistente(textos.diasTitulo);
    const dias = document.createElement("div");
    dias.className = "opciones-tipo";
    let revisarSiguiente = () => {};
    textos.dias.forEach((nombre, i) => {
      const etiqueta = document.createElement("label");
      const caja = document.createElement("input");
      caja.type = "checkbox";
      caja.checked = datos.weekdays.includes(i + 1);
      caja.addEventListener("change", () => {
        if (caja.checked) datos.weekdays.push(i + 1);
        else datos.weekdays = datos.weekdays.filter((dia) => dia !== i + 1);
        revisarSiguiente();
      });
      etiqueta.append(caja, document.createTextNode(nombre));
      dias.append(etiqueta);
    });
    diasSec.append(dias);
    const siguiente = botonSiguiente(() => {
      if (!datos.times.length) {
        aviso.textContent = textos.sinHora;
        return false;
      }
      if (!datos.weekdays.length) {
        aviso.textContent = textos.sinDias;
        return false;
      }
      return true;
    });
    const falta = notaFalta();
    revisarSiguiente = exigir(siguiente, falta, () => {
      if (!datos.times.length) return textos.sinHora;
      if (!datos.weekdays.length) return textos.sinDias;
      return "";
    });
    cuerpo.append(...titulo, horasSec, diasSec, pieAtras(siguiente, falta));
  }

  function pasoResumen(cuerpo, aviso) {
    const titulo = tituloPaso(textos.revisaTitulo, textos.resumenAyuda);
    const bloque = (tituloSec, filas) => {
      const seccion = seccionAsistente(tituloSec);
      const lista = document.createElement("div");
      lista.className = "lista resumen";
      filas.forEach((fila) => lista.append(fila));
      seccion.append(lista);
      return seccion;
    };
    const dias = datos.weekdays.length === 7
      ? textos.todosLosDias
      : datos.weekdays.slice().sort((a, b) => a - b).map((dia) => textos.dias[dia - 1]).join(", ");
    const resumen = [
      bloque(textos.resumenDestino, [filaIcono(iconoDisco, datos.path)]),
      bloque(textos.resumenCarpetas, datos.sources.map((ruta) => filaIcono(iconoCarpeta, ruta))),
      bloque(textos.resumenCuando, datos.times.map((hora) => filaIcono(iconoReloj, hora + " · " + dias)))
    ];
    const empezar = document.createElement("button");
    empezar.type = "button";
    empezar.textContent = textos.empezar;
    // Plataforma web (opcional): se pide la clave aquí y el alta se hace al terminar la configuración.
    const nube = seccionAsistente(textos.plataforma.asistenteTitulo);
    const nubeAyuda = document.createElement("p");
    nubeAyuda.className = "nota";
    nubeAyuda.textContent = textos.plataforma.asistenteAyuda;
    const nubeCampos = document.createElement("div");
    nubeCampos.hidden = true;
    const campoUrl = document.createElement("label");
    campoUrl.className = "campo-columna";
    campoUrl.append(document.createTextNode(textos.plataforma.direccion));
    const nubeUrl = document.createElement("input");
    nubeUrl.type = "text";
    nubeUrl.inputMode = "url";
    nubeUrl.placeholder = textos.plataforma.direccionPista;
    nubeUrl.autocomplete = "off";
    nubeUrl.value = datos.nubeUrl || "";
    nubeUrl.addEventListener("input", () => { datos.nubeUrl = nubeUrl.value.trim(); });
    campoUrl.append(nubeUrl);
    const campoClave = document.createElement("label");
    campoClave.className = "campo-columna";
    campoClave.append(document.createTextNode(textos.plataforma.clave));
    const nubeClave = document.createElement("input");
    nubeClave.type = "text";
    nubeClave.placeholder = textos.plataforma.clavePista;
    nubeClave.autocomplete = "off";
    nubeClave.value = datos.nubeClave || "";
    nubeClave.addEventListener("input", () => { datos.nubeClave = nubeClave.value.trim(); });
    campoClave.append(nubeClave);
    nubeCampos.append(campoUrl, campoClave);
    nube.append(nubeAyuda, nubeCampos);
    fetch("/api/v1/cloud").then(async (r) => {
      if (!r.ok) { nubeCampos.hidden = false; return; }
      const vista = await r.json();
      if (vista.alta_file) {
        nubeAyuda.textContent = textos.plataforma.altaFichero;
        return;
      }
      if (!datos.nubeUrl && vista.server_url) { nubeUrl.value = vista.server_url; datos.nubeUrl = vista.server_url; }
      nubeCampos.hidden = false;
    }).catch(() => { nubeCampos.hidden = false; });
    cuerpo.append(...titulo, ...resumen, nube, pieAtras(empezar));
    empezar.addEventListener("click", async () => {
      empezar.disabled = true;
      empezar.textContent = textos.preparando;
      const respuesta = await fetch("/api/v1/setup", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({
          path: datos.path,
          sources: datos.sources,
          excludes: datos.excludes,
          times: datos.times,
          weekdays: datos.weekdays,
          password: datos.password,
          confirm: datos.confirm,
          acknowledged: datos.ack
        })
      });
      const cuerpoRes = await respuesta.json().catch(() => ({}));
      if (!respuesta.ok) {
        empezar.disabled = false;
        empezar.textContent = textos.empezar;
        aviso.textContent = (cuerpoRes.fields || [cuerpoRes.error || textos.error]).join(" ");
        return;
      }
      datos.password = "";
      datos.confirm = "";
      borrarBorrador();
      // La configuración ya existe: ahora se puede dar de alta en la plataforma. Si falla, no
      // se bloquea nada; se avisa en la pantalla de Inicio y se puede repetir en Ajustes.
      if (datos.nubeClave) {
        const alta = await fetch("/api/v1/cloud/enroll", {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
          body: JSON.stringify({ credential: datos.nubeClave, server_url: datos.nubeUrl || "" })
        });
        if (!alta.ok) {
          const motivo = await alta.json().catch(() => ({}));
          try {
            sessionStorage.setItem("ba_aviso", textos.plataforma.asistenteFalloAviso.replace("{motivo}", (motivo.fields || [motivo.error || ""]).join(" ")));
          } catch (e) { /* sin almacenamiento: no pasa nada */ }
        }
        datos.nubeClave = "";
      }
      location.reload();
    });
  }

  function pieAtras(principal, falta) {
    const pie = document.createElement("div");
    pie.className = "pie-paso";
    if (falta) pie.append(botonAtras(), falta, principal);
    else pie.append(botonAtras(), principal);
    return pie;
  }

  function botonAtras() {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.className = "secundario";
    boton.textContent = textos.atras;
    boton.addEventListener("click", () => { datos.paso -= 1; dibujar(); });
    return boton;
  }

  function botonSiguiente(vale) {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.textContent = textos.siguiente;
    boton.addEventListener("click", () => {
      if (vale && !vale()) return;
      datos.paso += 1;
      dibujar();
    });
    return boton;
  }
}

function textoHoja(equipo, destino) {
  const fecha = new Intl.DateTimeFormat("es", { day: "2-digit", month: "2-digit", year: "numeric" }).format(new Date());
  return [
    "Hoja de recuperación de BackupAgent",
    "",
    "Equipo: " + (equipo || ""),
    ...(Array.isArray(destino)
      ? ["Destinos (la misma contraseña en todos):", ...destino.map((d) => "  - " + d)]
      : ["Destino: " + (destino || "")]),
    "Fecha: " + fecha,
    "",
    "Contraseña del repositorio (anótala a mano; esta hoja no la incluye):",
    "",
    "________________________________",
    "",
    "Si se pierde la contraseña, las copias no se pueden recuperar.",
    "Guarda esta hoja fuera del equipo."
  ].join("\n");
}

function descargarHoja(equipo, destino) {
  const blob = new Blob([textoHoja(equipo, destino)], { type: "text/plain;charset=utf-8" });
  const enlace = document.createElement("a");
  enlace.href = URL.createObjectURL(blob);
  enlace.download = "recuperacion-backup.txt";
  enlace.click();
  URL.revokeObjectURL(enlace.href);
}

function imprimirHoja(equipo, destino) {
  const ventana = window.open("", "_blank");
  if (!ventana) return;
  const pre = ventana.document.createElement("pre");
  pre.textContent = textoHoja(equipo, destino);
  ventana.document.body.append(pre);
  ventana.print();
}

function secreto(texto, valor) {
  const etiqueta = document.createElement("label");
  etiqueta.className = "campo-columna";
  etiqueta.append(document.createTextNode(texto));
  const input = document.createElement("input");
  input.type = "password";
  input.autocomplete = "new-password";
  input.value = valor || "";
  etiqueta.append(input);
  return { etiqueta, input };
}
