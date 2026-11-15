let seleccion = [];
let detalleSeleccion = {};

// Pantalla Restaurar: elige la copia por fecha, explora como en el selector de carpetas,
// marca lo que quieres recuperar y elige dónde guardarlo.
function pintarRestaurar() {
  const raiz = document.getElementById("pantalla-restaurar");
  raiz.replaceChildren();
  seleccion = [];
  detalleSeleccion = {};
  let copiaId = "";
  let copiaCuando = "";
  let actual = "";
  let nodos = [];
  let raices = [];
  let busqueda = "";
  let pedido = 0;
  let trabajando = false;
  let fuente = ""; // copia adicional de la que se recupera; vacío es la copia principal
  let fuenteNombre = "";

  const titulo = tituloPagina(textos.menu.restaurar, textos.restaurarAyuda);
  const cuerpo = document.createElement("div");
  cuerpo.className = "restaurar-cuerpo-nuevo";
  const mensaje = document.createElement("p");
  mensaje.className = "nota";
  raiz.append(titulo, mensaje, cuerpo);

  // ---- 1. Copia
  const cajaCopia = document.createElement("article");
  cajaCopia.className = "bloque";
  const hCopia = document.createElement("h2");
  hCopia.textContent = textos.restaurarCopiaTitulo;
  const filaCopia = document.createElement("label");
  filaCopia.className = "restaurar-copia";
  const etiquetaCopia = document.createElement("span");
  etiquetaCopia.textContent = textos.copiaDel;
  const selectorCopia = document.createElement("select");
  filaCopia.append(etiquetaCopia, selectorCopia);
  // De dónde se recupera: solo aparece si hay copias adicionales.
  const filaFuente = document.createElement("div");
  filaFuente.hidden = true;
  const etiquetaFuente = document.createElement("label");
  etiquetaFuente.className = "restaurar-copia";
  const textoFuente = document.createElement("span");
  textoFuente.textContent = textos.restaurarDesde;
  const selectorFuente = document.createElement("select");
  etiquetaFuente.append(textoFuente, selectorFuente);
  const notaFuente = document.createElement("p");
  notaFuente.className = "nota";
  notaFuente.setAttribute("role", "status");
  filaFuente.append(etiquetaFuente, notaFuente);
  cajaCopia.append(hCopia, filaFuente, filaCopia);

  // ---- 2. Qué recuperar (mismo aspecto que el selector de carpetas)
  const cajaQue = document.createElement("article");
  cajaQue.className = "bloque";
  const hQue = document.createElement("h2");
  hQue.textContent = textos.restaurarQueTitulo;
  const ayudaQue = document.createElement("p");
  ayudaQue.className = "nota";
  ayudaQue.textContent = textos.restaurarCarpetaEntera;
  const explorador = document.createElement("div");
  explorador.className = "explorador";
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
  const consulta = document.createElement("input");
  consulta.type = "search";
  consulta.className = "explorador-buscar";
  consulta.placeholder = textos.buscarEnCopia;
  consulta.setAttribute("aria-label", textos.buscarEnCopia);
  barra.append(subir, campoRuta, consulta);
  const lista = document.createElement("div");
  lista.className = "explorador-lista";
  lista.setAttribute("role", "list");
  const elegidosCaja = document.createElement("div");
  elegidosCaja.className = "explorador-elegidas";
  elegidosCaja.hidden = true;
  explorador.append(barra, lista, elegidosCaja);
  cajaQue.append(hQue, ayudaQue, explorador);

  // ---- 3. Dónde guardarlo
  const cajaDonde = document.createElement("article");
  cajaDonde.className = "bloque";
  const hDonde = document.createElement("h2");
  hDonde.textContent = textos.guardarEn;
  const destino = document.createElement("input");
  destino.type = "text";
  destino.setAttribute("aria-label", textos.otraCarpeta);
  const radioOtra = document.createElement("input");
  radioOtra.type = "radio";
  radioOtra.name = "modo-restaurar";
  radioOtra.checked = true;
  const radioOriginal = document.createElement("input");
  radioOriginal.type = "radio";
  radioOriginal.name = "modo-restaurar";
  const otra = document.createElement("label");
  otra.append(radioOtra, document.createTextNode(textos.otraCarpeta + " "));
  const recomendado = document.createElement("span");
  recomendado.className = "pastilla ok";
  recomendado.textContent = textos.recomendado;
  otra.append(recomendado);
  const original = document.createElement("label");
  original.append(radioOriginal, document.createTextNode(textos.sitioOriginal));
  const avisoOriginal = document.createElement("p");
  avisoOriginal.className = "nota";
  avisoOriginal.hidden = true;
  avisoOriginal.textContent = textos.avisoOriginal;
  const sustituir = document.createElement("label");
  sustituir.hidden = true;
  const caja = document.createElement("input");
  caja.type = "checkbox";
  sustituir.append(caja, document.createTextNode(textos.sustituir));
  const opcionesDonde = document.createElement("div");
  opcionesDonde.className = "restaurar-opciones";
  opcionesDonde.append(otra, destino, original, avisoOriginal, sustituir);
  cajaDonde.append(hDonde, opcionesDonde);

  const resultado = document.createElement("p");
  resultado.className = "resultado";

  // ---- Barra fija con el resumen y el botón
  const barraFija = document.createElement("div");
  barraFija.className = "guardar-barra";
  const resumen = document.createElement("span");
  const cancelarRestaurar = document.createElement("button");
  cancelarRestaurar.type = "button";
  cancelarRestaurar.className = "secundario";
  cancelarRestaurar.textContent = textos.cancelarRestauracion;
  cancelarRestaurar.hidden = true;
  const recuperar = document.createElement("button");
  recuperar.type = "button";
  recuperar.textContent = textos.recuperar;
  barraFija.append(resumen, cancelarRestaurar, recuperar);

  cuerpo.append(cajaCopia, cajaQue, cajaDonde, resultado, barraFija);

  // ---- Auxiliares
  function iconoNodo(esCarpeta) {
    const icono = document.createElement("span");
    icono.className = "fila-carpeta-icono";
    icono.innerHTML = esCarpeta
      ? iconoSvg("folder")
      : iconoSvg("file");
    return icono;
  }

  function carpetaDe(ruta) {
    const arriba = String(ruta || "").replace(/[\\/]+$/, "").replace(/[\\/][^\\/]+$/, "");
    return arriba.length < 3 ? "" : arriba;
  }

  function padre(ruta) {
    return raices.includes(ruta) ? "" : carpetaDe(ruta);
  }

  function alternar(nodo, marcado) {
    if (marcado && !seleccion.includes(nodo.path)) {
      seleccion.push(nodo.path);
      detalleSeleccion[nodo.path] = { nombre: nodo.name, tipo: nodo.type, tam: nodo.size || 0 };
    }
    if (!marcado) seleccion = seleccion.filter((item) => item !== nodo.path);
    pintarResumen();
    pintarElegidos();
  }

  function pintarResumen() {
    const n = seleccion.length;
    const hayCarpetas = seleccion.some((ruta) => (detalleSeleccion[ruta] || {}).tipo === "dir");
    recuperar.disabled = trabajando || !n || !copiaId;
    if (!n) {
      resumen.textContent = textos.marcaQueRecuperar;
      return;
    }
    let texto = n === 1 ? textos.unElemento : textos.variosElementos.replace("{n}", String(n));
    if (hayCarpetas) {
      texto += " · " + textos.incluyeCarpetas;
    } else {
      const suma = seleccion.reduce((total, ruta) => total + ((detalleSeleccion[ruta] || {}).tam || 0), 0);
      texto += " · " + tamano(suma);
    }
    resumen.textContent = texto;
  }

  function pintarElegidos() {
    elegidosCaja.replaceChildren();
    elegidosCaja.hidden = !seleccion.length;
    if (!seleccion.length) return;
    const tituloElegidos = document.createElement("strong");
    tituloElegidos.textContent = textos.elegidoParaRecuperar.replace("{n}", String(seleccion.length));
    const chips = document.createElement("div");
    chips.className = "explorador-chips";
    seleccion.forEach((ruta) => {
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
        seleccion = seleccion.filter((item) => item !== ruta);
        pintarResumen();
        pintarElegidos();
        pintarLista();
      });
      chip.append(texto, quitar);
      chips.append(chip);
    });
    elegidosCaja.append(tituloElegidos, chips);
  }

  function filaNodo(nodo, conRuta) {
    const fila = document.createElement("div");
    fila.className = "explorador-fila";
    fila.setAttribute("role", "listitem");
    const marca = document.createElement("input");
    marca.type = "checkbox";
    marca.className = "explorador-marca";
    marca.checked = seleccion.includes(nodo.path);
    marca.setAttribute("aria-label", textos.seleccionarElemento.replace("{nombre}", nodo.name));
    marca.addEventListener("change", () => alternar(nodo, marca.checked));
    const esCarpeta = nodo.type === "dir";
    const nombre = document.createElement(esCarpeta ? "button" : "span");
    nombre.className = "explorador-nombre" + (esCarpeta ? "" : " archivo");
    if (esCarpeta) {
      nombre.type = "button";
      nombre.addEventListener("click", () => {
        consulta.value = "";
        busqueda = "";
        cargarCarpeta(nodo.path);
      });
    }
    const texto = document.createElement("span");
    texto.className = "explorador-texto";
    const principal = document.createElement("span");
    principal.textContent = nodo.name;
    texto.append(principal);
    if (conRuta) {
      const donde = document.createElement("small");
      donde.textContent = carpetaDe(nodo.path) || nodo.path;
      texto.append(donde);
    }
    nombre.append(iconoNodo(esCarpeta), texto);
    fila.append(marca, nombre);
    if (!esCarpeta && nodo.size) {
      const peso = document.createElement("span");
      peso.className = "explorador-dato";
      peso.textContent = tamano(nodo.size);
      fila.append(peso);
    }
    if (nodo.mtime) {
      const fecha = document.createElement("span");
      fecha.className = "explorador-dato";
      fecha.textContent = fechaCorta(nodo.mtime);
      fila.append(fecha);
    }
    return fila;
  }

  function pintarLista(mensajeVacio) {
    campoRuta.value = actual || textos.esteEquipo;
    subir.disabled = !actual || !!busqueda;
    lista.replaceChildren();
    if (mensajeVacio) {
      const vacio = document.createElement("p");
      vacio.className = "explorador-vacio";
      vacio.textContent = mensajeVacio;
      lista.append(vacio);
      return;
    }
    if (!nodos.length) {
      const vacio = document.createElement("p");
      vacio.className = "explorador-vacio";
      vacio.textContent = busqueda ? textos.sinResultadosRestaurar : textos.carpetaVaciaRestaurar;
      lista.append(vacio);
      return;
    }
    nodos.forEach((nodo) => lista.append(filaNodo(nodo, !!busqueda)));
  }

  async function cargarCarpeta(ruta) {
    const id = ++pedido;
    actual = ruta;
    nodos = [];
    campoRuta.value = actual || textos.esteEquipo;
    lista.replaceChildren();
    const cargando = document.createElement("p");
    cargando.className = "explorador-vacio";
    cargando.textContent = textos.cargandoRestaurar;
    lista.append(cargando);
    let respuesta;
    try {
      respuesta = await fetch(conFuente("/api/v1/snapshots/" + encodeURIComponent(copiaId) + "/files?path=" + encodeURIComponent(ruta)));
    } catch (e) {
      if (id === pedido) pintarLista(textos.error);
      return;
    }
    if (id !== pedido) return;
    if (!respuesta.ok) {
      pintarLista(textos.error);
      return;
    }
    nodos = (await respuesta.json()) || [];
    if (!ruta) raices = nodos.map((nodo) => nodo.path);
    pintarLista();
  }

  async function buscar() {
    const texto = consulta.value.trim();
    busqueda = texto;
    if (!texto) {
      cargarCarpeta(actual);
      return;
    }
    const id = ++pedido;
    campoRuta.value = textos.resultadosDeBusqueda;
    subir.disabled = true;
    const respuesta = await fetch(conFuente("/api/v1/snapshots/" + encodeURIComponent(copiaId) + "/find?q=" + encodeURIComponent(patronBusqueda(texto))));
    const datos = await respuesta.json().catch(() => []);
    if (id !== pedido) return;
    if (!respuesta.ok) {
      nodos = [];
      pintarLista((datos && datos.error) || textos.error);
      return;
    }
    nodos = datos || [];
    pintarLista();
  }

  function elegirCopia(id, cuando) {
    copiaId = id;
    copiaCuando = cuando;
    seleccion = [];
    detalleSeleccion = {};
    consulta.value = "";
    busqueda = "";
    pintarResumen();
    pintarElegidos();
    cargarCarpeta("");
  }

  // ---- Eventos
  subir.addEventListener("click", () => cargarCarpeta(padre(actual)));
  let buscarTimer = 0;
  consulta.addEventListener("input", () => {
    clearTimeout(buscarTimer);
    buscarTimer = setTimeout(buscar, 300);
  });
  selectorCopia.addEventListener("change", () => {
    const opcion = selectorCopia.selectedOptions[0];
    elegirCopia(selectorCopia.value, opcion ? opcion.dataset.cuando : "");
  });
  function modoOriginal() {
    const activo = radioOriginal.checked;
    avisoOriginal.hidden = !activo;
    sustituir.hidden = !activo;
    destino.disabled = activo;
  }
  radioOtra.addEventListener("change", modoOriginal);
  radioOriginal.addEventListener("change", modoOriginal);

  fetch("/api/v1/restore/target").then(async (respuesta) => {
    if (!respuesta.ok) return;
    const datos = await respuesta.json();
    destino.value = datos.target || "";
  });

  function conFuente(url) {
    return fuente ? url + (url.includes("?") ? "&" : "?") + "replica=" + encodeURIComponent(fuente) : url;
  }

  function cargarCopias() {
  selectorCopia.replaceChildren();
  cuerpo.hidden = false;
  mensaje.className = "nota";
  mensaje.textContent = "";
  seleccion = [];
  detalleSeleccion = {};
  copiaId = "";
  pintarResumen();
  pintarElegidos();
  fetch(conFuente("/api/v1/snapshots")).then(async (respuesta) => {
    if (!respuesta.ok) {
      const dato = await respuesta.json().catch(() => ({}));
      cuerpo.hidden = true;
      mensaje.className = "aviso";
      mensaje.textContent = fuente && dato.error ? dato.error : textos.error;
      return;
    }
    const copias = ((await respuesta.json()) || []).slice().sort((a, b) => new Date(b.time) - new Date(a.time));
    if (!copias.length) {
      cuerpo.hidden = true;
      mensaje.textContent = textos.sinCopiasRestaurar;
      return;
    }
    const grupos = {};
    copias.forEach((item, indice) => {
      const nombreGrupo = grupoCopia(item.time) || textos.recientes;
      if (!grupos[nombreGrupo]) {
        const grupo = document.createElement("optgroup");
        grupo.label = nombreGrupo;
        grupos[nombreGrupo] = grupo;
        selectorCopia.append(grupo);
      }
      const opcion = document.createElement("option");
      opcion.value = item.id;
      opcion.dataset.cuando = tituloCopia(item.time);
      opcion.textContent = tituloCopia(item.time) + " · " + subtituloCopia(item.time) + (indice === 0 ? " · " + textos.masReciente.toLowerCase() : "");
      grupos[nombreGrupo].append(opcion);
    });
    selectorCopia.value = copias[0].id;
    elegirCopia(copias[0].id, tituloCopia(copias[0].time));
  });
  }

  // Con copias adicionales se puede elegir de dónde recuperar. Por defecto, la principal;
  // si no está disponible, la primera copia adicional conectada.
  async function cargarFuentes() {
    let datos = null;
    try {
      const respuesta = await fetch("/api/v1/replicas");
      if (respuesta.ok) datos = await respuesta.json();
    } catch (e) { /* sin copias adicionales: se sigue con la principal */ }
    const adicionales = datos && datos.replicas ? datos.replicas : [];
    if (!adicionales.length) {
      cargarCopias();
      return;
    }
    const principal = datos.primary || {};
    filaFuente.hidden = false;
    const opcionPrincipal = document.createElement("option");
    opcionPrincipal.value = "";
    opcionPrincipal.textContent = textos.copias.principal + (principal.connected ? "" : " · " + textos.fuenteSinConexion);
    opcionPrincipal.disabled = !principal.connected;
    selectorFuente.append(opcionPrincipal);
    adicionales.forEach((c) => {
      const opcion = document.createElement("option");
      opcion.value = c.id;
      opcion.textContent = c.name + " · " + textos.fuenteAdicional + (c.connected ? "" : " · " + textos.fuenteSinConexion);
      opcion.disabled = !c.connected;
      selectorFuente.append(opcion);
    });
    const elegible = principal.connected ? "" : (adicionales.find((c) => c.connected) || {}).id;
    function pintarNotaFuente() {
      const c = adicionales.find((x) => x.id === fuente);
      fuenteNombre = c ? c.name : "";
      if (!c) {
        notaFuente.className = "nota";
        notaFuente.textContent = textos.fuenteNotaPrincipal;
        return;
      }
      const hasta = c.last_success_at ? fechaRelativa(c.last_success_at) : textos.nunca;
      notaFuente.className = principal.connected ? "nota" : "aviso-caja info";
      notaFuente.textContent = (principal.connected ? "" : textos.fuenteNotaCaida.replace("{nombre}", c.name) + " ") + textos.fuenteNotaAdicional.replace("{cuando}", hasta);
    }
    if (elegible === undefined) {
      cuerpo.hidden = true;
      mensaje.className = "aviso";
      mensaje.textContent = textos.fuenteNinguna;
      return;
    }
    fuente = elegible;
    selectorFuente.value = fuente;
    pintarNotaFuente();
    selectorFuente.addEventListener("change", () => {
      fuente = selectorFuente.value;
      pintarNotaFuente();
      cargarCopias();
    });
    cargarCopias();
  }
  cargarFuentes();

  cancelarRestaurar.addEventListener("click", async () => {
    if (!(await confirmarSiNo({
      titulo: textos.confirmarCancelarRestauracionTitulo,
      texto: textos.confirmarCancelarRestauracion,
      aceptar: textos.cancelarRestauracion,
      cancelar: textos.seguirRestaurando,
      peligro: true
    }))) return;
    cancelarRestaurar.disabled = true;
    resultado.textContent = textos.cancelandoRestauracion;
    await fetch("/api/v1/cancel", { method: "POST", headers: { "X-CSRF-Token": csrf } });
    vigilar();
  });

  recuperar.addEventListener("click", async () => {
    if (!seleccion.length) {
      resultado.className = "resultado aviso";
      resultado.textContent = textos.nadaSeleccionado;
      return;
    }
    if (radioOriginal.checked && !(await confirmarSiNo({
      titulo: textos.restaurarOriginalTitulo,
      texto: textos.confirmarOriginal,
      aceptar: textos.restaurarOriginalBoton,
      peligro: true
    }))) return;
    trabajando = true;
    pintarResumen();
    cancelarRestaurar.hidden = false;
    cancelarRestaurar.disabled = false;
    resultado.className = "resultado";
    resultado.textContent = textos.restaurando;
    const respuesta = await fetch("/api/v1/restore", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({
        replica_id: fuente,
        snapshot: copiaId,
        paths: seleccion,
        mode: radioOriginal.checked ? "original" : "folder",
        target: destino.value,
        overwrite: caja.checked
      })
    });
    const cuerpoRespuesta = await respuesta.json().catch(() => ({}));
    if (!respuesta.ok) {
      trabajando = false;
      pintarResumen();
      cancelarRestaurar.hidden = true;
      resultado.className = "resultado aviso";
      resultado.textContent = (cuerpoRespuesta.fields || [cuerpoRespuesta.error || textos.error]).join(" ");
      return;
    }
    vigilar();
  });

  async function vigilar() {
    const respuesta = await fetch("/api/v1/status");
    if (!respuesta.ok) return;
    const estado = await respuesta.json();
    if (estado.activity === "restore") {
      const avance = estado.progress ? " " + Math.round(estado.progress.percent) + " %" : "";
      resultado.textContent = textos.restaurando + avance;
      setTimeout(vigilar, 1000);
      return;
    }
    trabajando = false;
    pintarResumen();
    cancelarRestaurar.hidden = true;
    if (estado.restore) {
      resultado.className = estado.restore.status === "success" ? "resultado aviso-ok" : "resultado aviso";
      resultado.textContent = estado.restore.message + (estado.restore.target ? " " + estado.restore.target : "");
    }
  }

  modoOriginal();
  pintarResumen();
}

// Quien busca escribe "informe", no "*informe*": se busca por parte del nombre sin pedirle comodines.
function patronBusqueda(texto) {
  return /[*?\[]/.test(texto) ? texto : "*" + texto + "*";
}

function diasDesde(iso) {
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return 999;
  const dia = (fecha) => new Date(fecha.getFullYear(), fecha.getMonth(), fecha.getDate()).getTime();
  return Math.round((dia(new Date()) - dia(momento)) / 86400000);
}

function grupoCopia(iso) {
  const dias = diasDesde(iso);
  if (dias <= 7) return "";
  if (dias <= 35) return textos.semanales;
  return textos.mensuales;
}

function tituloCopia(iso) {
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return iso;
  const hora = new Intl.DateTimeFormat("es", { hour: "2-digit", minute: "2-digit" }).format(momento);
  const dias = diasDesde(iso);
  if (dias === 0) return "Hoy, " + hora;
  if (dias === 1) return "Ayer, " + hora;
  return new Intl.DateTimeFormat("es", { day: "numeric", month: "short" }).format(momento) + ", " + hora;
}

function subtituloCopia(iso) {
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return "";
  return new Intl.DateTimeFormat("es", { weekday: "long", day: "numeric", month: "short" }).format(momento);
}

function fechaCorta(iso) {
  const momento = new Date(iso);
  if (Number.isNaN(momento.getTime())) return "";
  return new Intl.DateTimeFormat("es", { day: "numeric", month: "short", year: "numeric" }).format(momento);
}
