// Iconos Lucide (ISC) del sprite que hay al principio de index.html. Un único juego, trazo de 2 px.
function iconoSvg(nombre) {
  return '<svg viewBox="0 0 24 24" aria-hidden="true"><use href="#i-' + nombre + '"/></svg>';
}

let borrador = null;
let borradorOriginal = null;
let retencionManual = false;
let pantallaActual = "inicio";
const pantallasConfig = ["copiar", "cuando", "retencion", "verificar"];

// ---------------------------------------------------------------------------------------------
// Cuadro de diálogo propio: accesible por teclado (Esc cancela, el foco no se escapa) y que
// sustituye a window.confirm. Devuelve "aceptar", "extra" o "cancelar".
function mostrarDialogo(opciones) {
  return new Promise((resolver) => {
    const previo = document.activeElement;
    const fondo = document.createElement("div");
    fondo.className = "modal-fondo";
    const caja = document.createElement("div");
    caja.className = "modal";
    caja.setAttribute("role", "dialog");
    caja.setAttribute("aria-modal", "true");
    caja.setAttribute("aria-labelledby", "modal-titulo");
    caja.setAttribute("aria-describedby", "modal-texto");
    const titulo = document.createElement("h2");
    titulo.id = "modal-titulo";
    titulo.textContent = opciones.titulo;
    const texto = document.createElement("p");
    texto.id = "modal-texto";
    texto.textContent = opciones.texto;
    const botones = document.createElement("div");
    botones.className = "modal-botones";
    const cancelar = document.createElement("button");
    cancelar.type = "button";
    cancelar.className = "secundario";
    cancelar.textContent = opciones.cancelar || textos.cancelar;
    const extra = document.createElement("button");
    extra.type = "button";
    extra.className = "secundario";
    extra.textContent = opciones.extra || "";
    const aceptar = document.createElement("button");
    aceptar.type = "button";
    aceptar.textContent = opciones.aceptar || textos.entendido;
    if (opciones.peligro) aceptar.classList.add("peligroso");
    if (!opciones.sinCancelar) botones.append(cancelar);
    if (opciones.extra) botones.append(extra);
    botones.append(aceptar);
    caja.append(titulo, texto, botones);
    fondo.append(caja);

    const aplicacion = document.querySelector(".app");
    if (aplicacion) aplicacion.setAttribute("inert", "");

    function cerrar(valor) {
      document.removeEventListener("keydown", alTeclear, true);
      fondo.remove();
      if (aplicacion) aplicacion.removeAttribute("inert");
      try { if (previo && previo.focus) previo.focus(); } catch (e) { /* el elemento ya no existe */ }
      resolver(valor);
    }
    function alTeclear(evento) {
      if (evento.key === "Escape" && !opciones.sinCancelar) {
        evento.preventDefault();
        cerrar("cancelar");
        return;
      }
      if (evento.key !== "Tab") return;
      const enfocables = Array.from(caja.querySelectorAll("button")).filter((boton) => !boton.hidden);
      const primero = enfocables[0];
      const ultimo = enfocables[enfocables.length - 1];
      if (evento.shiftKey && document.activeElement === primero) {
        evento.preventDefault();
        ultimo.focus();
      } else if (!evento.shiftKey && document.activeElement === ultimo) {
        evento.preventDefault();
        primero.focus();
      } else if (!caja.contains(document.activeElement)) {
        evento.preventDefault();
        primero.focus();
      }
    }
    cancelar.addEventListener("click", () => cerrar("cancelar"));
    extra.addEventListener("click", () => cerrar("extra"));
    aceptar.addEventListener("click", () => cerrar("aceptar"));
    fondo.addEventListener("mousedown", (evento) => {
      if (evento.target === fondo && !opciones.sinCancelar) cerrar("cancelar");
    });
    document.addEventListener("keydown", alTeclear, true);
    document.body.append(fondo);
    // En acciones con riesgo, el foco empieza en la opción segura.
    (opciones.peligro && !opciones.sinCancelar ? cancelar : aceptar).focus();
  });
}

async function confirmarSiNo(opciones) {
  return (await mostrarDialogo(opciones)) === "aceptar";
}

// Una sola vez: si la sesión caduca, avisar de qué hacer en lugar de mostrar errores sueltos.
let sesionAvisada = false;
const fetchOriginal = window.fetch.bind(window);
window.fetch = async function (...argumentos) {
  const respuesta = await fetchOriginal(...argumentos);
  if (respuesta.status === 401 && !sesionAvisada) {
    sesionAvisada = true;
    mostrarDialogo({ titulo: textos.sesionCaducadaTitulo, texto: textos.sesionCaducadaTexto, aceptar: textos.entendido, sinCancelar: true });
  }
  return respuesta;
};

// Cambios sin guardar: se comparan con lo que se cargó del servidor.
function hayCambios() {
  return !!borrador && borradorOriginal !== null && JSON.stringify(borrador) !== borradorOriginal;
}

window.addEventListener("beforeunload", (evento) => {
  if (hayCambios()) {
    evento.preventDefault();
    evento.returnValue = "";
  }
});

function refrescarBarras() {
  document.querySelectorAll(".guardar-barra").forEach((barra) => {
    if (barra.refrescar) barra.refrescar();
  });
}
["input", "change", "click"].forEach((tipo) => {
  document.addEventListener(tipo, () => setTimeout(refrescarBarras, 0));
});

async function confirmarSalida() {
  return mostrarDialogo({
    titulo: textos.sinGuardarTitulo,
    texto: textos.sinGuardarTexto,
    cancelar: textos.sinGuardarSeguir,
    extra: textos.sinGuardarSalir,
    aceptar: textos.sinGuardarGuardar
  });
}

function iniciarConfiguracion() {
  iniciarMenu(true);
}

function iniciarMenu(admin) {
  document.getElementById("barra").hidden = false;
  const menu = document.getElementById("menu");
  menu.querySelectorAll("button").forEach((boton) => {
    const id = boton.dataset.pantalla;
    boton.hidden = !admin && id !== "inicio" && id !== "historial";
    boton.querySelector("span").textContent = textos.menu[id];
    boton.addEventListener("click", () => mostrarPantalla(id));
  });
  const pedida = new URLSearchParams(location.search).get("pantalla");
  const directas = pedida === "historial" || (admin && (pedida === "restaurar" || pedida === "ajustes" || pedida === "copiar"));
  mostrarPantalla(directas ? pedida : "inicio");
}

function mostrarPantalla(id, sinAviso) {
  const saleDeConfig = pantallasConfig.includes(pantallaActual) && !pantallasConfig.includes(id);
  if (!sinAviso && saleDeConfig && hayCambios()) {
    const origen = pantallaActual;
    confirmarSalida().then(async (decision) => {
      if (decision === "aceptar") {
        await guardar(origen);
        if (!hayCambios()) mostrarPantalla(id, true);
      } else if (decision === "extra") {
        borrador = null;
        borradorOriginal = null;
        mostrarPantalla(id, true);
      }
    });
    return;
  }
  pantallaActual = id;
  document.getElementById("pantalla-inicio").hidden = id !== "inicio";
  ["copiar", "destino", "cuando", "retencion", "verificar", "historial", "restaurar", "ajustes"].forEach((nombre) => {
    document.getElementById("pantalla-" + nombre).hidden = nombre !== id;
  });
  document.querySelectorAll("#menu button").forEach((boton) => {
    const activa = boton.dataset.pantalla === id || (boton.dataset.pantalla === "cuando" && (id === "retencion" || id === "verificar"));
    boton.classList.toggle("activo", activa);
    if (activa) boton.setAttribute("aria-current", "page");
    else boton.removeAttribute("aria-current");
  });
  if (id === "historial") {
    pintarHistorial();
    return;
  }
  if (id === "restaurar") {
    pintarRestaurar();
    return;
  }
  if (id === "ajustes") {
    pintarAjustes();
    return;
  }
  if (id === "destino") {
    pintarDestino();
    return;
  }
  if (id !== "inicio") cargarBorrador(id);
}

async function cargarBorrador(id) {
  if (!borrador) {
    const respuesta = await fetch("/api/v1/config");
    if (!respuesta.ok) {
      pintarAviso(id, textos.error);
      return;
    }
    borrador = await respuesta.json();
    borradorOriginal = JSON.stringify(borrador);
    retencionManual = false;
  }
  if (id === "copiar") pintarCopiar();
  if (id === "cuando") pintarCuando();
  if (id === "retencion") pintarRetencion();
  if (id === "verificar") pintarVerificar();
}

function pintarAviso(id, mensaje, ok) {
  const nodo = document.querySelector("#pantalla-" + id + " .resultado");
  if (!nodo) return;
  nodo.hidden = !mensaje;
  nodo.textContent = mensaje || "";
  nodo.className = "resultado " + (ok ? "aviso-ok" : "aviso");
}

// Aviso breve y flotante, visible aunque la pantalla sea larga.
let toastTimer = 0;
function mostrarToast(mensaje, ok) {
  let nodo = document.getElementById("toast");
  if (!nodo) {
    nodo = document.createElement("div");
    nodo.id = "toast";
    nodo.setAttribute("role", "status");
    document.body.append(nodo);
  }
  nodo.textContent = mensaje;
  nodo.className = "toast " + (ok ? "ok" : "error");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { nodo.className = "toast oculto"; }, 3500);
}

async function guardar(id) {
  const respuesta = await fetch("/api/v1/config", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: JSON.stringify(borrador)
  });
  const cuerpo = await respuesta.json().catch(() => ({}));
  if (!respuesta.ok) {
    const detalle = (cuerpo.fields || []).join(" ");
    pintarAviso(id, (cuerpo.error || textos.error) + (detalle ? " " + detalle : ""), false);
    return;
  }
  borrador = null;
  borradorOriginal = null;
  await cargarBorrador(id);
  pintarAviso(id, textos.guardado, true);
  mostrarToast(textos.guardado, true);
  setTimeout(() => pintarAviso(id, ""), 4000);
  const estado = await fetch("/api/v1/status");
  if (estado.ok) pintar(await estado.json());
}

function botonGuardar(id) {
  const boton = document.createElement("button");
  boton.type = "button";
  boton.textContent = textos.guardar;
  boton.addEventListener("click", () => guardar(id));
  return boton;
}

// Un botón que no puede usarse se desactiva y dice qué falta. motivo() devuelve "" si ya se puede.
function notaFalta() {
  const nota = document.createElement("p");
  nota.className = "falta";
  nota.setAttribute("role", "status");
  nota.hidden = true;
  return nota;
}

function exigir(boton, nota, motivo) {
  const revisar = () => {
    const falta = motivo();
    boton.disabled = !!falta;
    nota.textContent = falta || "";
    nota.hidden = !falta;
  };
  revisar();
  return revisar;
}

function faltaParaGuardar(id) {
  if (!borrador) return "";
  if (id === "copiar" && !(borrador.sources || []).length) return textos.sinCarpetas;
  if (id === "cuando" && !(borrador.times || []).length) return textos.sinHora;
  if (id === "cuando" && !(borrador.weekdays || []).length) return textos.sinDias;
  return "";
}

function tituloPagina(titulo, ayuda) {
  const bloque = document.createElement("header");
  bloque.className = "cabecera";
  const h = document.createElement("h1");
  h.textContent = titulo;
  const p = document.createElement("p");
  p.className = "nota";
  p.textContent = ayuda || "";
  bloque.append(h, p);
  return bloque;
}

function barraGuardar(id, resumen) {
  const barra = document.createElement("div");
  barra.className = "guardar-barra";
  const textoBarra = document.createElement("span");
  textoBarra.textContent = resumen || "";
  const descartar = document.createElement("button");
  descartar.type = "button";
  descartar.className = "secundario";
  descartar.textContent = textos.descartar;
  descartar.addEventListener("click", () => {
    borrador = null;
    borradorOriginal = null;
    cargarBorrador(id);
  });
  const guardarBoton = botonGuardar(id);
  barra.append(textoBarra, descartar, guardarBoton);
  // La barra solo aparece si hay cambios; el botón se apaga si falta algo imprescindible.
  barra.refrescar = () => {
    const cambios = hayCambios();
    barra.hidden = !cambios;
    const falta = faltaParaGuardar(id);
    guardarBoton.disabled = !cambios || !!falta;
    if (falta) textoBarra.textContent = falta;
    else if (id === "copiar" && borrador) textoBarra.textContent = resumenCopiar();
    else textoBarra.textContent = textos.cambiosSinGuardar;
  };
  barra.refrescar();
  return barra;
}

function subtitulo(textoTitulo) {
  const h = document.createElement("h2");
  h.textContent = textoTitulo;
  return h;
}

const tamanos = {};

// Tarjeta-casilla de una carpeta habitual. La usan Qué copiar y el asistente.
function tarjetaCarpeta(atajo, marcada, alPulsar) {
  const boton = document.createElement("button");
  boton.type = "button";
  boton.className = "casilla-carpeta" + (marcada ? " activa" : "");
  boton.setAttribute("aria-pressed", marcada ? "true" : "false");
  const nombre = document.createElement("strong");
  nombre.textContent = textos.atajos[atajo.id] || atajo.id;
  const peso = document.createElement("span");
  peso.className = "peso";
  peso.textContent = tamanos[atajo.path] == null ? "…" : tamano(tamanos[atajo.path]);
  boton.append(nombre, peso);
  boton.addEventListener("click", alPulsar);
  return { boton, peso };
}

// Fila de regla "No copiar": nombre, explicación debajo y casilla a la derecha.
function filaRegla(plantilla, marcada, alCambiar) {
  const fila = document.createElement("label");
  fila.className = "regla";
  const textoRegla = document.createElement("span");
  const nombre = document.createElement("strong");
  nombre.textContent = plantilla.texto;
  const ayuda = document.createElement("p");
  ayuda.textContent = plantilla.ayuda || "";
  textoRegla.append(nombre, ayuda);
  const caja = document.createElement("input");
  caja.type = "checkbox";
  caja.checked = marcada;
  caja.addEventListener("change", () => alCambiar(caja.checked));
  fila.append(textoRegla, caja);
  return fila;
}

function pintarCopiar() {
  const raiz = document.getElementById("pantalla-copiar");
  raiz.replaceChildren();
  const habitual = document.createElement("article");
  habitual.className = "bloque";
  const tituloHabitual = document.createElement("h2");
  tituloHabitual.textContent = textos.habituales;
  const quien = document.createElement("p");
  quien.className = "nota";
  habitual.append(tituloHabitual, quien);
  const mosaico = document.createElement("div");
  mosaico.className = "mosaico";
  habitual.append(mosaico);
  const otrasCaja = document.createElement("article");
  otrasCaja.className = "bloque";
  const cabezaOtras = document.createElement("div");
  cabezaOtras.className = "bloque-cabeza";
  const textoOtras = document.createElement("div");
  const tituloOtras = document.createElement("h2");
  tituloOtras.textContent = textos.otrasCarpetas;
  const ayudaOtras = document.createElement("p");
  ayudaOtras.className = "nota";
  ayudaOtras.textContent = textos.otrasAyuda;
  textoOtras.append(tituloOtras, ayudaOtras);
  const otras = document.createElement("div");
  otras.className = "lista";
  if (!exploradorCopiar) {
    exploradorCopiar = crearExplorador({
      modo: "anadir",
      alElegir: anadirOrigen,
      elegidas: () => (borrador && borrador.sources) || [],
      alQuitar: quitarOrigen,
      alCerrar: () => {
        exploradorCopiarAbierto = false;
        exploradorCopiar.raiz.hidden = true;
        const boton = document.querySelector("#pantalla-copiar [aria-expanded]");
        if (boton) { boton.setAttribute("aria-expanded", "false"); boton.focus(); }
      }
    });
  }
  const explorador = exploradorCopiar;
  const explorar = explorador.raiz;
  explorar.hidden = !exploradorCopiarAbierto;
  if (exploradorCopiarAbierto) explorador.refrescar();
  const anadir = document.createElement("button");
  anadir.type = "button";
  anadir.className = "secundario";
  anadir.textContent = "+ " + textos.anadirOtra;
  anadir.setAttribute("aria-expanded", exploradorCopiarAbierto ? "true" : "false");
  anadir.addEventListener("click", () => {
    anadir.setAttribute("aria-expanded", "true");
    exploradorCopiarAbierto = true;
    explorar.hidden = false;
    explorador.cargarSiHaceFalta();
    explorador.refrescar();
  });
  cabezaOtras.append(textoOtras, anadir);
  otrasCaja.append(cabezaOtras, otras, explorar);
  const noCaja = document.createElement("article");
  noCaja.className = "bloque";
  const tituloNo = document.createElement("h2");
  tituloNo.textContent = textos.noCopiar;
  const ayudaNo = document.createElement("p");
  ayudaNo.className = "nota";
  ayudaNo.textContent = textos.noCopiarAyuda;
  noCaja.append(tituloNo, ayudaNo);
  const reglas = document.createElement("div");
  reglas.className = "plantillas";
  const personal = document.createElement("textarea");
  personal.rows = 4;
  personal.hidden = true;
  const conocidas = new Set(textos.plantillas.map((plantilla) => plantilla.patron));
  textos.plantillas.forEach((plantilla) => {
    reglas.append(filaRegla(plantilla, (borrador.excludes || []).includes(plantilla.patron), (activa) => {
      const actual = new Set(borrador.excludes || []);
      if (activa) actual.add(plantilla.patron);
      else actual.delete(plantilla.patron);
      borrador.excludes = Array.from(actual);
    }));
  });
  personal.value = (borrador.excludes || []).filter((patron) => !conocidas.has(patron)).join("\n");
  personal.addEventListener("change", () => {
    const fijas = (borrador.excludes || []).filter((patron) => conocidas.has(patron));
    const extra = personal.value.split(/\r?\n/).map((linea) => linea.trim()).filter(Boolean);
    borrador.excludes = fijas.concat(extra);
  });
  const reglaExtra = document.createElement("button");
  reglaExtra.type = "button";
  reglaExtra.className = "enlace";
  reglaExtra.textContent = "+ " + textos.reglaPersonal;
  reglaExtra.addEventListener("click", () => { personal.hidden = false; });
  noCaja.append(reglas, reglaExtra, personal);
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  const barra = barraGuardar("copiar", resumenCopiar());
  raiz.append(tituloPagina(textos.menu.copiar, textos.copiarAyuda), habitual, otrasCaja, noCaja, aviso, barra);
  fetch("/api/v1/shortcuts").then((respuesta) => respuesta.json()).then((atajos) => {
    const rutas = new Set((atajos || []).map((atajo) => atajo.path));
    const usuarios = Array.from(new Set((atajos || []).map((atajo) => atajo.user).filter(Boolean)));
    quien.textContent = usuarios.length === 1 ? textos.delUsuario.replace("{nombre}", usuarios[0]) : "";
    (atajos || []).forEach((atajo) => {
      const marcada = (borrador.sources || []).includes(atajo.path);
      const tarjeta = tarjetaCarpeta(atajo, marcada, () => {
        const fuentes = borrador.sources || [];
        borrador.sources = marcada ? fuentes.filter((ruta) => ruta !== atajo.path) : fuentes.concat([atajo.path]);
        pintarCopiar();
      });
      mosaico.append(tarjeta.boton);
      pedirTamano(atajo.path, tarjeta.peso, barra);
    });
    (borrador.sources || []).filter((ruta) => !rutas.has(ruta)).forEach((ruta) => {
      const fila = document.createElement("div");
      fila.className = "fila-carpeta";
      const icono = document.createElement("span");
      icono.className = "fila-carpeta-icono";
      icono.innerHTML = iconoSvg("folder");
      const nombre = document.createElement("span");
      nombre.className = "fila-carpeta-ruta";
      nombre.textContent = ruta;
      const peso = document.createElement("span");
      peso.className = "fila-carpeta-peso";
      peso.textContent = tamanos[ruta] == null ? "…" : tamano(tamanos[ruta]);
      const quitar = document.createElement("button");
      quitar.type = "button";
      quitar.className = "enlace";
      quitar.textContent = textos.quitar;
      quitar.addEventListener("click", () => {
        borrador.sources = (borrador.sources || []).filter((item) => item !== ruta);
        pintarCopiar();
      });
      fila.append(icono, nombre, peso, quitar);
      otras.append(fila);
      pedirTamano(ruta, peso, barra);
    });
  });
}

function resumenCopiar() {
  const rutas = borrador.sources || [];
  let texto = rutas.length === 1 ? textos.seCopiaraUna : textos.seCopiaran.replace("{n}", String(rutas.length));
  if (rutas.length && rutas.every((ruta) => tamanos[ruta] != null)) {
    const suma = rutas.reduce((total, ruta) => total + tamanos[ruta], 0);
    texto += " · " + textos.tamanoEstimado.replace("{tamano}", tamano(suma));
  }
  return texto;
}

async function pedirTamano(ruta, nodo, barra) {
  if (tamanos[ruta] == null) {
    const respuesta = await fetch("/api/v1/size", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ sources: [ruta] })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    if (!respuesta.ok) {
      nodo.textContent = "";
      return;
    }
    tamanos[ruta] = cuerpo.bytes || 0;
  }
  if (nodo.isConnected) nodo.textContent = tamano(tamanos[ruta]);
  const resumen = barra && barra.querySelector("span");
  if (resumen) resumen.textContent = resumenCopiar();
}

// El selector de carpetas de Qué copiar se conserva mientras la pantalla siga abierta:
// recuerda por dónde ibas y no se cierra al añadir una carpeta.
let exploradorCopiar = null;
let exploradorCopiarAbierto = false;

function quitarOrigen(ruta) {
  borrador.sources = (borrador.sources || []).filter((item) => item !== ruta);
  pintarCopiar();
}

function anadirOrigen(ruta) {
  borrador.sources = borrador.sources || [];
  if (!borrador.sources.includes(ruta)) borrador.sources.push(ruta);
  pintarCopiar();
}

function pintarCuando() {
  const raiz = document.getElementById("pantalla-cuando");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.cuando, textos.cuandoAyuda);
  const atajos = pestanasProgramacion("cuando");

  const dias = document.createElement("article");
  dias.className = "bloque";
  const hDias = document.createElement("h2");
  hDias.textContent = textos.diasTitulo;
  const rejilla = document.createElement("div");
  rejilla.className = "dias-semana";
  textos.dias.forEach((nombre, indice) => {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.className = "dia";
    boton.textContent = textos.diasCortos[indice];
    boton.setAttribute("aria-label", nombre);
    const dia = indice + 1;
    const activo = (borrador.weekdays || []).includes(dia);
    boton.classList.toggle("activo", activo);
    boton.setAttribute("aria-pressed", activo ? "true" : "false");
    boton.addEventListener("click", () => {
      const lista = new Set(borrador.weekdays || []);
      if (lista.has(dia)) lista.delete(dia);
      else lista.add(dia);
      borrador.weekdays = Array.from(lista).sort((a, b) => a - b);
      pintarCuando();
    });
    rejilla.append(boton);
  });
  dias.append(hDias, rejilla);

  const horasBloque = document.createElement("article");
  horasBloque.className = "bloque";
  const hHoras = document.createElement("h2");
  hHoras.textContent = textos.horas;
  const lista = document.createElement("div");
  lista.className = "lista";
  (borrador.times || []).forEach((hora, indice) => {
    const fila = document.createElement("div");
    fila.className = "fila-hora";
    const icono = document.createElement("span");
    icono.className = "fila-carpeta-icono";
    icono.innerHTML = iconoSvg("clock");
    const campo = document.createElement("input");
    campo.type = "time";
    campo.step = "60";
    campo.value = horaVisible(hora);
    campo.addEventListener("change", () => { borrador.times[indice] = horaVisible(campo.value); });
    const quitar = document.createElement("button");
    quitar.type = "button";
    quitar.className = "enlace";
    quitar.textContent = textos.quitar;
    quitar.addEventListener("click", () => {
      borrador.times.splice(indice, 1);
      pintarCuando();
    });
    fila.append(icono, campo, quitar);
    lista.append(fila);
  });
  const anadir = document.createElement("button");
  anadir.type = "button";
  anadir.className = "secundario";
  anadir.textContent = textos.anadirHora;
  anadir.addEventListener("click", () => {
    borrador.times = borrador.times || [];
    borrador.times.push("12:00");
    pintarCuando();
  });
  horasBloque.append(hHoras, lista, anadir);

  const fallos = document.createElement("article");
  fallos.className = "bloque";
  const hFallos = document.createElement("h2");
  hFallos.textContent = textos.siNoTermina;
  const cifras = document.createElement("div");
  cifras.className = "cifras-ajuste";
  cifras.append(
    numero(textos.reintentos, borrador.max_retries, (valor) => { borrador.max_retries = valor; }),
    numero(textos.espera, borrador.retry_delay_minutes, (valor) => { borrador.retry_delay_minutes = valor; }),
    numero(textos.diasRojo, borrador.stale_after_days, (valor) => { borrador.stale_after_days = valor; })
  );
  fallos.append(
    hFallos,
    filaOpcion(textos.alArrancar, borrador.run_missed_on_startup, (valor) => { borrador.run_missed_on_startup = valor; }),
    cifras
  );

  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  raiz.append(titulo, atajos, dias, horasBloque, fallos, barraGuardar("cuando", textos.guardarPista), aviso);
}

// Horario, Retención y Verificación son partes de la misma programación: se ven como pestañas.
function pestanasProgramacion(activa) {
  const nav = document.createElement("nav");
  nav.className = "pestanas";
  nav.setAttribute("aria-label", textos.menu.cuando);
  [["cuando", textos.pestanaHorario], ["retencion", textos.menu.retencion], ["verificar", textos.menu.verificar]].forEach((par) => {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.textContent = par[1];
    if (par[0] === activa) boton.setAttribute("aria-current", "page");
    else boton.addEventListener("click", () => mostrarPantalla(par[0]));
    nav.append(boton);
  });
  return nav;
}

function botonVolver() {
  const boton = document.createElement("button");
  boton.type = "button";
  boton.className = "volver";
  boton.textContent = textos.volver;
  boton.addEventListener("click", () => mostrarPantalla("cuando"));
  return boton;
}

function accesoAjuste(titulo, ayuda, alPulsar) {
  const boton = document.createElement("button");
  boton.type = "button";
  boton.className = "acceso";
  const nombre = document.createElement("strong");
  nombre.textContent = titulo;
  const nota = document.createElement("span");
  nota.textContent = ayuda;
  boton.append(nombre, nota);
  boton.addEventListener("click", alPulsar);
  return boton;
}

function horaVisible(valor) {
  const partes = String(valor || "").trim().split(":");
  if (partes.length < 2) return "12:00";
  const hora = partes[0].padStart(2, "0").slice(-2);
  const minuto = partes[1].padStart(2, "0").slice(0, 2);
  if (!/^\d{2}$/.test(hora) || !/^\d{2}$/.test(minuto)) return "12:00";
  return hora + ":" + minuto;
}

function filaOpcion(etiqueta, valor, alCambiar) {
  const campo = document.createElement("label");
  campo.className = "opcion";
  const texto = document.createElement("span");
  texto.textContent = etiqueta;
  const caja = document.createElement("input");
  caja.type = "checkbox";
  caja.checked = !!valor;
  caja.addEventListener("change", () => alCambiar(caja.checked));
  const pista = document.createElement("span");
  pista.className = "pista";
  pista.setAttribute("aria-hidden", "true");
  campo.append(texto, caja, pista);
  return campo;
}

function pintarRetencion() {
  const raiz = document.getElementById("pantalla-retencion");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.retencion, textos.retencionAyuda);
  const pestanas = pestanasProgramacion("retencion");
  const opciones = document.createElement("article");
  opciones.className = "bloque";
  const rejilla = document.createElement("div");
  rejilla.className = "presets";
  const actual = presetActual();
  ["basica", "recomendada", "larga", "avanzada"].forEach((nombre) => {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.className = "preset";
    boton.classList.toggle("activo", nombre === actual);
    boton.setAttribute("aria-pressed", nombre === actual ? "true" : "false");
    const nombreNodo = document.createElement("strong");
    nombreNodo.textContent = textos.presetNombres[nombre];
    const nota = document.createElement("span");
    nota.textContent = nombre === "avanzada" ? textos.avanzada : textos.presets[nombre].texto;
    boton.append(nombreNodo, nota);
    boton.addEventListener("click", () => {
      if (nombre === "avanzada") {
        retencionManual = true;
      } else {
        retencionManual = false;
        const preset = textos.presets[nombre];
        borrador.keep_daily = preset.daily;
        borrador.keep_weekly = preset.weekly;
        borrador.keep_monthly = preset.monthly;
        borrador.keep_yearly = preset.yearly;
      }
      pintarRetencion();
    });
    rejilla.append(boton);
  });
  opciones.append(rejilla);

  const avanzada = actual === "avanzada";
  const cifrasBloque = document.createElement("article");
  cifrasBloque.className = "bloque";
  const cifras = document.createElement("div");
  cifras.className = "cifras-ajuste cuatro";
  cifras.append(
    numero(textos.diaria, borrador.keep_daily, (valor) => { borrador.keep_daily = valor; }, !avanzada),
    numero(textos.semanal, borrador.keep_weekly, (valor) => { borrador.keep_weekly = valor; }, !avanzada),
    numero(textos.mensual, borrador.keep_monthly, (valor) => { borrador.keep_monthly = valor; }, !avanzada),
    numero(textos.anual, borrador.keep_yearly, (valor) => { borrador.keep_yearly = valor; }, !avanzada)
  );
  cifrasBloque.append(
    cifras,
    filaOpcion(textos.liberar, borrador.prune, (valor) => { borrador.prune = valor; })
  );

  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  raiz.append(titulo, pestanas, opciones, cifrasBloque, barraGuardar("retencion", textos.guardarPista), aviso);
}

function presetActual() {
  if (retencionManual) return "avanzada";
  const nombres = ["basica", "recomendada", "larga"];
  for (const nombre of nombres) {
    const preset = textos.presets[nombre];
    if (borrador.keep_daily === preset.daily && borrador.keep_weekly === preset.weekly && borrador.keep_monthly === preset.monthly && borrador.keep_yearly === preset.yearly) {
      return nombre;
    }
  }
  return "avanzada";
}

function pintarVerificar() {
  const raiz = document.getElementById("pantalla-verificar");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.verificar, textos.verificarAyuda);
  const pestanas = pestanasProgramacion("verificar");
  const bloque = document.createElement("article");
  bloque.className = "bloque";
  const cifras = document.createElement("div");
  cifras.className = "cifras-ajuste dos";
  const porcion = textoCampo(textos.porcion, borrador.read_data_subset, (valor) => { borrador.read_data_subset = valor; });
  const ayuda = document.createElement("p");
  ayuda.className = "nota";
  ayuda.textContent = textos.porcionAyuda;
  porcion.append(ayuda);
  cifras.append(
    numero(textos.cada, borrador.every_days, (valor) => { borrador.every_days = valor; }),
    porcion
  );
  bloque.append(cifras);
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  raiz.append(titulo, pestanas, bloque, barraGuardar("verificar", textos.guardarPista), aviso);
}

function numero(etiqueta, valor, alCambiar, bloqueado) {
  const campo = document.createElement("label");
  campo.textContent = etiqueta;
  const entrada = document.createElement("input");
  entrada.type = "number";
  entrada.min = "0";
  entrada.value = valor;
  entrada.disabled = !!bloqueado;
  entrada.addEventListener("change", () => alCambiar(Number(entrada.value)));
  campo.appendChild(entrada);
  return campo;
}

function textoCampo(etiqueta, valor, alCambiar) {
  const campo = document.createElement("label");
  campo.textContent = etiqueta;
  const entrada = document.createElement("input");
  entrada.type = "text";
  entrada.value = valor || "";
  entrada.addEventListener("change", () => alCambiar(entrada.value.trim()));
  campo.appendChild(entrada);
  return campo;
}

function interruptor(etiqueta, valor, alCambiar) {
  const campo = document.createElement("label");
  const caja = document.createElement("input");
  caja.type = "checkbox";
  caja.checked = !!valor;
  caja.addEventListener("change", () => alCambiar(caja.checked));
  campo.append(caja, document.createTextNode(etiqueta));
  return campo;
}

// Selector de carpetas con aspecto de explorador de archivos.
// Se usa en Destino, en el asistente y en Qué copiar. Solo presenta lo que da /api/v1/folders.
// opciones.modo: "elegir" (un botón para usar la carpeta actual) o "anadir" (además, "Añadir" en cada fila).
function crearExplorador(opciones) {
  const modo = opciones.modo === "anadir" ? "anadir" : "elegir";
  const ocultas = /^(\$RECYCLE\.BIN|System Volume Information)$/i;
  let actual = "";
  let carpetas = [];
  let pedido = 0;
  let cargado = false;

  const raiz = document.createElement("div");
  raiz.className = "explorador";

  const barra = document.createElement("div");
  barra.className = "explorador-barra";
  const subir = document.createElement("button");
  subir.type = "button";
  subir.className = "secundario explorador-subir";
  subir.innerHTML = iconoSvg("arrow-up");
  const subirTexto = document.createElement("span");
  subirTexto.textContent = textos.carpetaSuperior;
  subir.append(subirTexto);
  const campoRuta = document.createElement("input");
  campoRuta.type = "text";
  campoRuta.readOnly = true;
  campoRuta.className = "explorador-ruta";
  campoRuta.setAttribute("aria-label", textos.carpetaActual);
  const buscar = document.createElement("input");
  buscar.type = "search";
  buscar.className = "explorador-buscar";
  buscar.placeholder = textos.buscarCarpetas;
  buscar.setAttribute("aria-label", textos.buscarCarpetas);
  barra.append(subir, campoRuta, buscar);

  const lista = document.createElement("div");
  lista.className = "explorador-lista";
  lista.setAttribute("role", "list");

  const pie = document.createElement("div");
  pie.className = "explorador-pie";
  const elegida = document.createElement("p");
  elegida.className = "explorador-elegida";
  const elegir = document.createElement("button");
  elegir.type = "button";
  elegir.textContent = modo === "anadir" ? textos.anadirEsta : textos.usarCarpeta;
  if (modo === "anadir") elegir.className = "secundario";
  const listo = document.createElement("button");
  listo.type = "button";
  listo.textContent = textos.listo;
  listo.hidden = modo !== "anadir";
  const acciones = document.createElement("div");
  acciones.className = "explorador-acciones";
  acciones.append(elegir, listo);
  pie.append(elegida, acciones);

  const elegidasCaja = document.createElement("div");
  elegidasCaja.className = "explorador-elegidas";
  elegidasCaja.hidden = true;

  raiz.append(barra, lista, elegidasCaja, pie);

  function listaElegidas() {
    return opciones.elegidas ? (opciones.elegidas() || []) : [];
  }

  function padre(ruta) {
    const arriba = ruta.replace(/[\\/]+$/, "").replace(/[\\/][^\\/]+$/, "");
    return arriba.length < 3 ? "" : arriba;
  }

  function nombreVisible(carpeta) {
    if (actual) return carpeta.name || carpeta.path;
    const letra = (carpeta.name || carpeta.path).replace(/\\$/, "");
    return carpeta.label ? carpeta.label + " (" + letra + ")" : letra;
  }

  function iconoCarpeta(unidad) {
    const icono = document.createElement("span");
    icono.className = "fila-carpeta-icono";
    icono.innerHTML = unidad
      ? iconoSvg("hard-drive")
      : iconoSvg("folder");
    return icono;
  }

  function pintarElegidas() {
    const rutas = listaElegidas();
    elegidasCaja.replaceChildren();
    elegidasCaja.hidden = modo !== "anadir" || !rutas.length;
    if (elegidasCaja.hidden) return;
    const titulo = document.createElement("strong");
    titulo.textContent = textos.carpetasElegidas.replace("{n}", String(rutas.length));
    const filas = document.createElement("div");
    filas.className = "explorador-chips";
    rutas.forEach((ruta) => {
      const chip = document.createElement("span");
      chip.className = "explorador-chip";
      const texto = document.createElement("span");
      texto.textContent = ruta;
      texto.title = ruta;
      const quitar = document.createElement("button");
      quitar.type = "button";
      quitar.className = "enlace";
      quitar.textContent = textos.quitar;
      quitar.setAttribute("aria-label", textos.quitarCarpeta.replace("{ruta}", ruta));
      quitar.addEventListener("click", () => {
        if (opciones.alQuitar) opciones.alQuitar(ruta);
        pintar();
      });
      chip.append(texto, quitar);
      filas.append(chip);
    });
    elegidasCaja.append(titulo, filas);
  }

  function pintar() {
    campoRuta.value = actual || textos.esteEquipo;
    subir.disabled = !actual;
    const yaElegida = modo === "anadir" && listaElegidas().includes(actual);
    elegir.disabled = !actual || yaElegida;
    if (modo === "anadir") elegir.textContent = yaElegida ? textos.anadida : textos.anadirEsta;
    pintarElegidas();
    elegida.textContent = actual ? textos.carpetaActual + ": " + actual : textos.elegirUnidad;
    lista.replaceChildren();
    const filtro = buscar.value.trim().toLowerCase();
    const propias = carpetas.filter((carpeta) => !(actual && ocultas.test(carpeta.name || "")));
    const visibles = propias.filter((carpeta) => !filtro || nombreVisible(carpeta).toLowerCase().includes(filtro));
    if (!visibles.length) {
      const vacio = document.createElement("p");
      vacio.className = "explorador-vacio";
      vacio.textContent = propias.length ? textos.sinCoincidencias : textos.sinSubcarpetas;
      lista.append(vacio);
      return;
    }
    visibles.forEach((carpeta) => {
      const fila = document.createElement("div");
      fila.className = "explorador-fila";
      fila.setAttribute("role", "listitem");
      const entrar = document.createElement("button");
      entrar.type = "button";
      entrar.className = "explorador-nombre";
      const nombre = document.createElement("span");
      nombre.textContent = nombreVisible(carpeta);
      entrar.append(iconoCarpeta(!actual), nombre);
      entrar.addEventListener("click", () => abrir(carpeta.path));
      fila.append(entrar);
      if (!actual && carpeta.total_bytes) {
        const espacio = document.createElement("span");
        espacio.className = "explorador-dato";
        espacio.textContent = textos.unidadLibres.replace("{libre}", tamano(carpeta.free_bytes || 0)).replace("{total}", tamano(carpeta.total_bytes));
        fila.append(espacio);
      }
      if (modo === "anadir") {
        if (listaElegidas().includes(carpeta.path)) {
          const hecho = document.createElement("span");
          hecho.className = "explorador-hecho";
          hecho.textContent = textos.anadida;
          fila.append(hecho);
        } else {
          const anadir = document.createElement("button");
          anadir.type = "button";
          anadir.className = "enlace";
          anadir.textContent = textos.anadir;
          anadir.addEventListener("click", () => {
            opciones.alElegir(carpeta.path);
            pintar();
          });
          fila.append(anadir);
        }
      }
      lista.append(fila);
    });
  }

  async function abrir(ruta) {
    const id = ++pedido;
    actual = ruta;
    buscar.value = "";
    cargado = true;
    let datos = [];
    try {
      const respuesta = await fetch("/api/v1/folders?path=" + encodeURIComponent(ruta));
      if (id !== pedido) return;
      if (respuesta.ok) datos = (await respuesta.json()) || [];
    } catch (e) {
      if (id !== pedido) return;
    }
    carpetas = datos;
    pintar();
  }

  subir.addEventListener("click", () => abrir(padre(actual)));
  buscar.addEventListener("input", pintar);
  elegir.addEventListener("click", () => {
    if (!actual) return;
    opciones.alElegir(actual);
    if (modo === "elegir") elegida.textContent = textos.carpetaElegida + ": " + actual;
    else pintar();
  });
  listo.addEventListener("click", () => {
    if (opciones.alCerrar) opciones.alCerrar();
  });
  pintar();

  return { raiz, abrir, refrescar: pintar, cargarSiHaceFalta: () => { if (!cargado) abrir(""); } };
}
