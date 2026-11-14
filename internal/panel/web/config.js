let borrador = null;

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

function mostrarPantalla(id) {
  document.getElementById("pantalla-inicio").hidden = id !== "inicio";
  ["copiar", "destino", "cuando", "retencion", "verificar", "historial", "restaurar", "ajustes"].forEach((nombre) => {
    document.getElementById("pantalla-" + nombre).hidden = nombre !== id;
  });
  document.querySelectorAll("#menu button").forEach((boton) => {
    boton.classList.toggle("activo", boton.dataset.pantalla === id);
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
  pintarAviso(id, textos.guardado, true);
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

function tituloPagina(titulo, ayuda) {
  const bloque = document.createElement("header");
  bloque.className = "cabecera";
  const h = document.createElement("h2");
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
    cargarBorrador(id);
  });
  barra.append(textoBarra, descartar, botonGuardar(id));
  return barra;
}

function subtitulo(textoTitulo) {
  const h = document.createElement("h2");
  h.textContent = textoTitulo;
  return h;
}

const tamanos = {};

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
  const explorar = document.createElement("div");
  explorar.className = "lista";
  explorar.hidden = true;
  const anadir = document.createElement("button");
  anadir.type = "button";
  anadir.className = "secundario";
  anadir.textContent = "+ " + textos.anadirOtra;
  anadir.addEventListener("click", () => {
    explorar.hidden = false;
    if (!explorar.childElementCount) abrirCarpeta(explorar, "");
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
  const personal = document.createElement("textarea");
  personal.rows = 4;
  personal.hidden = true;
  const conocidas = new Set(textos.plantillas.map((plantilla) => plantilla.patron));
  textos.plantillas.forEach((plantilla) => {
    const fila = document.createElement("label");
    fila.className = "regla";
    const textoRegla = document.createElement("span");
    const nombre = document.createElement("strong");
    nombre.textContent = plantilla.texto;
    const ayuda = document.createElement("p");
    ayuda.textContent = plantilla.ayuda || "";
    textoRegla.append(nombre, ayuda);
    const cajaCheck = document.createElement("input");
    cajaCheck.type = "checkbox";
    cajaCheck.checked = (borrador.excludes || []).includes(plantilla.patron);
    cajaCheck.addEventListener("change", () => {
      const actual = new Set(borrador.excludes || []);
      if (cajaCheck.checked) actual.add(plantilla.patron);
      else actual.delete(plantilla.patron);
      borrador.excludes = Array.from(actual);
    });
    fila.append(textoRegla, cajaCheck);
    reglas.append(fila);
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
      const boton = document.createElement("button");
      boton.type = "button";
      const marcada = (borrador.sources || []).includes(atajo.path);
      boton.className = "casilla-carpeta" + (marcada ? " activa" : "");
      const nombre = document.createElement("strong");
      nombre.textContent = textos.atajos[atajo.id] || atajo.id;
      const peso = document.createElement("span");
      peso.className = "peso";
      peso.textContent = tamanos[atajo.path] == null ? "…" : tamano(tamanos[atajo.path]);
      boton.append(nombre, peso);
      boton.addEventListener("click", () => {
        const fuentes = borrador.sources || [];
        borrador.sources = marcada ? fuentes.filter((ruta) => ruta !== atajo.path) : fuentes.concat([atajo.path]);
        pintarCopiar();
      });
      mosaico.append(boton);
      pedirTamano(atajo.path, peso, barra);
    });
    (borrador.sources || []).filter((ruta) => !rutas.has(ruta)).forEach((ruta) => {
      const fila = document.createElement("div");
      fila.className = "fila-carpeta";
      const nombre = document.createElement("span");
      nombre.className = "ruta";
      nombre.textContent = ruta;
      const peso = document.createElement("span");
      peso.className = "nota";
      peso.textContent = tamanos[ruta] == null ? "…" : tamano(tamanos[ruta]);
      const quitar = document.createElement("button");
      quitar.type = "button";
      quitar.className = "enlace";
      quitar.textContent = textos.quitar;
      quitar.addEventListener("click", () => {
        borrador.sources = (borrador.sources || []).filter((item) => item !== ruta);
        pintarCopiar();
      });
      fila.append(nombre, peso, quitar);
      otras.append(fila);
      pedirTamano(ruta, peso, barra);
    });
  });
}

function resumenCopiar() {
  const rutas = borrador.sources || [];
  let texto = textos.seCopiaran.replace("{n}", String(rutas.length));
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

function anadirOrigen(ruta) {
  borrador.sources = borrador.sources || [];
  if (!borrador.sources.includes(ruta)) borrador.sources.push(ruta);
  pintarCopiar();
}

async function abrirCarpeta(caja, ruta) {
  caja.innerHTML = "";
  const respuesta = await fetch("/api/v1/folders?path=" + encodeURIComponent(ruta));
  const carpetas = await respuesta.json().catch(() => []);
  if (!respuesta.ok) {
    caja.textContent = carpetas.error || textos.error;
    return;
  }
  carpetas.forEach((carpeta) => {
    const fila = document.createElement("div");
    fila.className = "fila";
    const nombre = document.createElement("span");
    nombre.className = "ruta";
    nombre.textContent = carpeta.name;
    const acciones = document.createElement("span");
    const abrir = document.createElement("button");
    abrir.type = "button";
    abrir.className = "secundario";
    abrir.textContent = textos.abrir;
    abrir.addEventListener("click", () => abrirCarpeta(caja, carpeta.path));
    const incluir = document.createElement("button");
    incluir.type = "button";
    incluir.textContent = textos.anadirCarpeta;
    incluir.addEventListener("click", () => anadirOrigen(carpeta.path));
    acciones.append(abrir, incluir);
    fila.append(nombre, acciones);
    caja.appendChild(fila);
  });
}

function pintarCuando() {
  const raiz = document.getElementById("pantalla-cuando");
  raiz.innerHTML = "";
  const titulo = tituloPagina(textos.menu.cuando, textos.cuandoAyuda);
  const mas = document.createElement("p");
  const irRetencion = document.createElement("button");
  irRetencion.type = "button";
  irRetencion.className = "enlace";
  irRetencion.textContent = textos.menu.retencion;
  irRetencion.addEventListener("click", () => mostrarPantalla("retencion"));
  const irVerificar = document.createElement("button");
  irVerificar.type = "button";
  irVerificar.className = "enlace";
  irVerificar.textContent = textos.menu.verificar;
  irVerificar.addEventListener("click", () => mostrarPantalla("verificar"));
  mas.append(irRetencion, document.createTextNode(" · "), irVerificar);
  const horas = document.createElement("div");
  horas.className = "horas";
  (borrador.times || []).forEach((hora, indice) => {
    const campo = document.createElement("input");
    campo.type = "text";
    campo.value = hora;
    campo.className = "campo";
    campo.addEventListener("change", () => { borrador.times[indice] = campo.value.trim(); });
    const quitar = document.createElement("button");
    quitar.type = "button";
    quitar.className = "secundario";
    quitar.textContent = textos.quitar;
    quitar.addEventListener("click", () => {
      borrador.times.splice(indice, 1);
      pintarCuando();
    });
    horas.append(campo, quitar);
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
  const etiqHoras = document.createElement("p");
  etiqHoras.textContent = textos.horas;
  raiz.append(titulo, mas, etiqHoras, horas, anadir);
  textos.dias.forEach((nombre, indice) => {
    const etiqueta = document.createElement("label");
    const caja = document.createElement("input");
    caja.type = "checkbox";
    const dia = indice + 1;
    caja.checked = (borrador.weekdays || []).includes(dia);
    caja.addEventListener("change", () => {
      const dias = new Set(borrador.weekdays || []);
      if (caja.checked) dias.add(dia);
      else dias.delete(dia);
      borrador.weekdays = Array.from(dias).sort((a, b) => a - b);
    });
    etiqueta.append(caja, document.createTextNode(nombre));
    raiz.appendChild(etiqueta);
  });
  raiz.append(
    interruptor(textos.alArrancar, borrador.run_missed_on_startup, (valor) => { borrador.run_missed_on_startup = valor; }),
    numero(textos.reintentos, borrador.max_retries, (valor) => { borrador.max_retries = valor; }),
    numero(textos.espera, borrador.retry_delay_minutes, (valor) => { borrador.retry_delay_minutes = valor; }),
    numero(textos.diasRojo, borrador.stale_after_days, (valor) => { borrador.stale_after_days = valor; }),
    barraGuardar("cuando", textos.menu.cuando)
  );
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  raiz.appendChild(aviso);
}

function pintarRetencion() {
  const raiz = document.getElementById("pantalla-retencion");
  raiz.innerHTML = "";
  const titulo = document.createElement("h2");
  titulo.textContent = textos.menu.retencion;
  const explicacion = document.createElement("p");
  raiz.append(titulo, explicacion);
  const nombres = ["basica", "recomendada", "larga", "avanzada"];
  const actual = presetActual();
  nombres.forEach((nombre) => {
    const etiqueta = document.createElement("label");
    const radio = document.createElement("input");
    radio.type = "radio";
    radio.name = "preset";
    radio.checked = nombre === actual;
    radio.addEventListener("change", () => {
      if (nombre === "avanzada") {
        explicacion.textContent = textos.avanzada;
        return;
      }
      const preset = textos.presets[nombre];
      borrador.keep_daily = preset.daily;
      borrador.keep_weekly = preset.weekly;
      borrador.keep_monthly = preset.monthly;
      borrador.keep_yearly = preset.yearly;
      explicacion.textContent = preset.texto;
      pintarRetencion();
    });
    etiqueta.append(radio, document.createTextNode(textos.presetNombres[nombre]));
    raiz.appendChild(etiqueta);
  });
  explicacion.textContent = actual === "avanzada" ? textos.avanzada : textos.presets[actual].texto;
  const avanzada = actual === "avanzada";
  raiz.append(
    numero(textos.diaria, borrador.keep_daily, (valor) => { borrador.keep_daily = valor; }, !avanzada),
    numero(textos.semanal, borrador.keep_weekly, (valor) => { borrador.keep_weekly = valor; }, !avanzada),
    numero(textos.mensual, borrador.keep_monthly, (valor) => { borrador.keep_monthly = valor; }, !avanzada),
    numero(textos.anual, borrador.keep_yearly, (valor) => { borrador.keep_yearly = valor; }, !avanzada),
    interruptor(textos.liberar, borrador.prune, (valor) => { borrador.prune = valor; }),
    barraGuardar("retencion", textos.menu.retencion)
  );
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  raiz.appendChild(aviso);
}

function presetActual() {
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
  raiz.innerHTML = "";
  const titulo = document.createElement("h2");
  titulo.textContent = textos.menu.verificar;
  const ayuda = document.createElement("p");
  ayuda.textContent = textos.porcionAyuda;
  raiz.append(
    titulo,
    numero(textos.cada, borrador.every_days, (valor) => { borrador.every_days = valor; }),
    textoCampo(textos.porcion, borrador.read_data_subset, (valor) => { borrador.read_data_subset = valor; }),
    ayuda,
    barraGuardar("verificar", textos.menu.verificar)
  );
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  aviso.hidden = true;
  raiz.appendChild(aviso);
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
