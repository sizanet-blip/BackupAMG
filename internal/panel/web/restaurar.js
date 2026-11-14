let seleccion = [];

function pintarRestaurar() {
  const raiz = document.getElementById("pantalla-restaurar");
  raiz.replaceChildren();
  seleccion = [];
  let copiaId = "";
  let copiaCuando = "";

  const pasos = document.createElement("p");
  pasos.className = "pasos";
  const arbol = document.createElement("div");
  arbol.className = "lista";
  const miga = document.createElement("div");
  miga.className = "miga";
  const consulta = document.createElement("input");
  consulta.type = "text";
  consulta.placeholder = textos.buscarArchivo;
  const resultados = document.createElement("div");
  resultados.className = "lista";
  const resumen = document.createElement("div");
  const destino = document.createElement("input");
  destino.type = "text";
  const radioOtra = document.createElement("input");
  radioOtra.type = "radio";
  radioOtra.name = "modo-restaurar";
  radioOtra.checked = true;
  const radioOriginal = document.createElement("input");
  radioOriginal.type = "radio";
  radioOriginal.name = "modo-restaurar";
  const aviso = document.createElement("p");
  aviso.className = "nota";
  aviso.hidden = true;
  aviso.textContent = textos.avisoOriginal;
  const sustituir = document.createElement("label");
  sustituir.hidden = true;
  const caja = document.createElement("input");
  caja.type = "checkbox";
  sustituir.append(caja, document.createTextNode(textos.sustituir));
  const recuperar = document.createElement("button");
  recuperar.type = "button";
  recuperar.textContent = textos.recuperar;
  const resultado = document.createElement("p");
  resultado.className = "resultado";
  const lista = document.createElement("div");

  function pintarPasos() {
    pasos.replaceChildren();
    const estadoPaso = [copiaId ? "hecho" : "activo", copiaId ? "activo" : "", ""];
    textos.pasosRestaurar.forEach((nombre, indice) => {
      const paso = document.createElement("span");
      paso.className = estadoPaso[indice];
      paso.textContent = (indice + 1) + "  " + nombre;
      pasos.append(paso);
    });
  }

  function pintarResumen() {
    resumen.replaceChildren();
    const titulo = document.createElement("strong");
    titulo.textContent = textos.archivosSeleccionados.replace("{n}", String(seleccion.length));
    const nota = document.createElement("p");
    nota.className = "nota";
    nota.textContent = copiaCuando ? textos.deLaCopia.replace("{cuando}", copiaCuando) : "";
    resumen.append(titulo, nota);
  }

  const cuerpo = document.createElement("div");
  cuerpo.className = "restaurar-cuerpo";
  const izquierda = document.createElement("div");
  izquierda.className = "columna";
  const derecha = document.createElement("div");
  derecha.className = "columna";
  const tituloCopias = document.createElement("h2");
  tituloCopias.textContent = textos.copiasDisponibles;
  izquierda.append(tituloCopias, lista);
  derecha.append(consulta, miga, arbol, resultados);
  cuerpo.append(izquierda, derecha);

  const pie = document.createElement("div");
  pie.className = "barra-restaurar";
  const modos = document.createElement("div");
  const etiqGuardar = document.createElement("p");
  etiqGuardar.textContent = textos.guardarEn;
  const otra = document.createElement("label");
  otra.append(radioOtra, document.createTextNode(textos.otraCarpeta + " "));
  const marca = document.createElement("span");
  marca.className = "pastilla ok";
  marca.textContent = textos.recomendado;
  otra.append(marca);
  const original = document.createElement("label");
  original.append(radioOriginal, document.createTextNode(textos.sitioOriginal));
  modos.append(etiqGuardar, otra, destino, original, aviso, sustituir);
  pie.append(resumen, modos, recuperar);
  raiz.append(tituloPagina(textos.menu.restaurar, textos.restaurarAyuda), pasos, cuerpo, pie, resultado);
  pintarPasos();
  pintarResumen();

  function modoOriginal() {
    const activo = radioOriginal.checked;
    aviso.hidden = !activo;
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
  fetch("/api/v1/snapshots").then(async (respuesta) => {
    if (!respuesta.ok) {
      resultado.textContent = textos.error;
      return;
    }
    const copias = await respuesta.json();
    let grupo = "";
    (copias || []).forEach((item, indice) => {
      const siguiente = grupoCopia(item.time);
      if (siguiente && siguiente !== grupo) {
        grupo = siguiente;
        const cabecera = document.createElement("p");
        cabecera.className = "grupo-copia";
        cabecera.textContent = grupo;
        lista.append(cabecera);
      }
      const boton = document.createElement("button");
      boton.type = "button";
      boton.className = "copia-item" + (indice === 0 ? " activo" : "");
      const textoFecha = document.createElement("strong");
      textoFecha.textContent = tituloCopia(item.time);
      const sub = document.createElement("small");
      sub.textContent = subtituloCopia(item.time);
      if (indice === 0) {
        const reciente = document.createElement("span");
        reciente.className = "pastilla";
        reciente.textContent = textos.masReciente;
        boton.append(textoFecha, reciente, sub);
      } else {
        boton.append(textoFecha, sub);
      }
      boton.addEventListener("click", () => {
        copiaId = item.id;
        copiaCuando = tituloCopia(item.time);
        lista.querySelectorAll(".copia-item").forEach((nodo) => nodo.classList.remove("activo"));
        boton.classList.add("activo");
        pintarPasos();
        pintarResumen();
        cargarCarpeta("");
      });
      lista.append(boton);
      if (indice === 0) {
        copiaId = item.id;
        copiaCuando = tituloCopia(item.time);
      }
    });
    pintarPasos();
    pintarResumen();
    if (copiaId) cargarCarpeta("");
  });

  function pintarMiga(path) {
    miga.replaceChildren();
    const raizBtn = document.createElement("button");
    raizBtn.type = "button";
    raizBtn.textContent = "Equipo";
    raizBtn.addEventListener("click", () => cargarCarpeta(""));
    miga.append(raizBtn);
    const partes = String(path || "").split(/\\/).filter(Boolean);
    let acumulado = "";
    partes.forEach((parte) => {
      acumulado = acumulado ? acumulado.replace(/\\$/, "") + "\\" + parte : parte;
      const boton = document.createElement("button");
      boton.type = "button";
      boton.textContent = parte;
      const destinoRuta = acumulado;
      boton.addEventListener("click", () => cargarCarpeta(destinoRuta));
      miga.append(document.createTextNode(" › "), boton);
    });
  }

  async function cargarCarpeta(path) {
    arbol.replaceChildren();
    resultados.replaceChildren();
    pintarMiga(path);
    const respuesta = await fetch("/api/v1/snapshots/" + encodeURIComponent(copiaId) + "/files?path=" + encodeURIComponent(path));
    if (!respuesta.ok) return;
    const nodos = await respuesta.json();
    (nodos || []).forEach((nodo) => arbol.append(filaArchivo(nodo)));
  }

  function filaArchivo(nodo) {
    const fila = document.createElement("label");
    fila.className = "fila tabla-archivo";
    const cajaNodo = document.createElement("input");
    cajaNodo.type = "checkbox";
    cajaNodo.checked = seleccion.includes(nodo.path);
    cajaNodo.addEventListener("change", () => {
      if (cajaNodo.checked && !seleccion.includes(nodo.path)) seleccion.push(nodo.path);
      if (!cajaNodo.checked) seleccion = seleccion.filter((item) => item !== nodo.path);
      pintarResumen();
    });
    const nombre = document.createElement("button");
    nombre.type = "button";
    nombre.className = "enlace";
    nombre.textContent = (nodo.type === "dir" ? "📁 " : "📄 ") + nodo.name;
    if (nodo.type === "dir") nombre.addEventListener("click", (evento) => {
      evento.preventDefault();
      cargarCarpeta(nodo.path);
    });
    const modificado = document.createElement("span");
    modificado.className = "nota";
    modificado.textContent = nodo.mtime ? fechaCorta(nodo.mtime) : "";
    const peso = document.createElement("span");
    peso.className = "nota";
    peso.textContent = nodo.type === "dir" ? "" : tamano(nodo.size);
    fila.append(cajaNodo, nombre, modificado, peso);
    return fila;
  }

  let buscarTimer = 0;
  consulta.addEventListener("input", () => {
    clearTimeout(buscarTimer);
    buscarTimer = setTimeout(buscar, 300);
  });
  async function buscar() {
    resultados.replaceChildren();
    if (!consulta.value.trim() || !copiaId) return;
    const respuesta = await fetch("/api/v1/snapshots/" + encodeURIComponent(copiaId) + "/find?q=" + encodeURIComponent(consulta.value));
    const nodos = await respuesta.json().catch(() => []);
    if (!respuesta.ok) {
      resultado.textContent = (nodos && nodos.error) || textos.error;
      return;
    }
    (nodos || []).forEach((nodo) => resultados.append(filaArchivo(nodo)));
  }

  recuperar.addEventListener("click", async () => {
    if (!seleccion.length) {
      resultado.className = "resultado";
      resultado.textContent = textos.nadaSeleccionado;
      return;
    }
    if (radioOriginal.checked && !window.confirm(textos.confirmarOriginal)) return;
    recuperar.disabled = true;
    resultado.textContent = textos.restaurando;
    const respuesta = await fetch("/api/v1/restore", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({
        snapshot: copiaId,
        paths: seleccion,
        mode: radioOriginal.checked ? "original" : "folder",
        target: destino.value,
        overwrite: caja.checked
      })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    if (!respuesta.ok) {
      recuperar.disabled = false;
      resultado.className = "resultado";
      resultado.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
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
    recuperar.disabled = false;
    if (estado.restore) {
      resultado.className = estado.restore.status === "success" ? "resultado aviso-ok" : "resultado";
      resultado.textContent = estado.restore.message + (estado.restore.target ? " " + estado.restore.target : "");
    }
  }
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
