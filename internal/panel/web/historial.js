function pintarHistorial() {
  const raiz = document.getElementById("pantalla-historial");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.historial, textos.historialAyuda);
  const filtros = document.createElement("div");
  filtros.className = "barra-filtros";
  const estado = document.createElement("select");
  [["", textos.todos], ["success", textos.estados.success], ["warning", textos.estados.warning], ["failed", textos.estados.failed]].forEach((opcion) => {
    const item = document.createElement("option");
    item.value = opcion[0];
    item.textContent = opcion[1];
    estado.append(item);
  });
  const desde = fechaInput(textos.desde);
  const hasta = fechaInput(textos.hasta);
  const filtrar = document.createElement("button");
  filtrar.type = "button";
  filtrar.className = "secundario";
  filtrar.textContent = textos.filtrar;
  filtros.append(estado, desde.etiqueta, hasta.etiqueta, filtrar);
  const cajaFiltros = document.createElement("article");
  cajaFiltros.className = "bloque";
  cajaFiltros.append(filtros);
  const lista = document.createElement("div");
  lista.className = "lista";
  const detalle = document.createElement("div");
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  const cajaLista = document.createElement("article");
  cajaLista.className = "bloque";
  const pie = document.createElement("div");
  pie.className = "historial-pie";
  const contador = document.createElement("span");
  contador.className = "nota";
  const masBoton = document.createElement("button");
  masBoton.type = "button";
  masBoton.className = "secundario";
  masBoton.textContent = textos.cargarMas;
  pie.hidden = true;
  pie.append(contador, masBoton);
  cajaLista.append(lista, aviso, pie);
  const cajaDetalle = document.createElement("article");
  cajaDetalle.className = "bloque";
  cajaDetalle.hidden = true;
  cajaDetalle.append(detalle);
  raiz.append(titulo, cajaFiltros, cajaLista, cajaDetalle);
  const PAGINA = 50;
  let cargadas = 0;
  filtrar.addEventListener("click", () => cargar(false));
  masBoton.addEventListener("click", () => cargar(true));
  cargar(false);

  async function cargar(masDeLaLista) {
    if (!masDeLaLista) {
      lista.replaceChildren();
      detalle.replaceChildren();
      cajaDetalle.hidden = true;
      cargadas = 0;
      pie.hidden = true;
      aviso.className = "resultado nota";
      aviso.textContent = textos.cargandoHistorial;
    }
    masBoton.disabled = true;
    const consulta = new URLSearchParams();
    if (estado.value) consulta.set("status", estado.value);
    if (desde.input.value) consulta.set("from", desde.input.value);
    if (hasta.input.value) consulta.set("to", hasta.input.value);
    consulta.set("limit", String(PAGINA));
    consulta.set("offset", String(cargadas));
    const respuesta = await fetch("/api/v1/history?" + consulta.toString());
    masBoton.disabled = false;
    if (!respuesta.ok) {
      aviso.className = "resultado aviso";
      aviso.textContent = textos.error;
      return;
    }
    const items = await respuesta.json();
    const total = Number(respuesta.headers.get("X-Total-Count")) || 0;
    aviso.className = "resultado";
    aviso.textContent = "";
    if (!items || !items.length) {
      if (!cargadas) aviso.textContent = textos.vacio;
      pie.hidden = true;
      return;
    }
    cargadas += items.length;
    pie.hidden = total <= cargadas;
    contador.textContent = textos.mostrandoDe.replace("{n}", String(cargadas)).replace("{total}", String(total));
    items.forEach((item) => {
      const boton = document.createElement("button");
      boton.type = "button";
      boton.className = "fila-historial";
      const pastilla = document.createElement("span");
      const clase = item.status === "success" ? "ok" : item.status === "warning" ? "aviso" : "error";
      pastilla.className = "pastilla " + clase;
      pastilla.textContent = textos.estados[item.status] || item.status;
      const descripcion = document.createElement("span");
      descripcion.className = "actividad-texto";
      descripcion.textContent = fraseActividad(item);
      const medida = document.createElement("span");
      medida.className = "nota";
      medida.textContent = (item.type === "verify" || item.type === "replica" ? "" : tamano(item.data_added_bytes) + " · ") + duracion(item.duration_seconds);
      const cuando = document.createElement("span");
      cuando.className = "nota";
      cuando.textContent = fechaRelativa(item.finished_at);
      boton.append(pastilla, descripcion, medida, cuando);
      boton.addEventListener("click", () => abrir(item.id));
      lista.append(boton);
    });
  }

  async function abrir(id) {
    const respuesta = await fetch("/api/v1/history/" + encodeURIComponent(id));
    const cuerpo = await respuesta.json().catch(() => ({}));
    detalle.replaceChildren();
    if (!respuesta.ok) {
      aviso.textContent = cuerpo.error || textos.error;
      return;
    }
    aviso.textContent = "";
    const texto = textoDetalle(cuerpo);
    const pre = document.createElement("pre");
    pre.textContent = texto;
    const copiar = document.createElement("button");
    copiar.type = "button";
    copiar.textContent = textos.copiarDetalle;
    copiar.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(texto);
        aviso.className = "resultado aviso-ok";
        aviso.textContent = textos.copiado;
      } catch (e) {
        aviso.className = "resultado";
        aviso.textContent = textos.error;
      }
    });
    detalle.append(pre, copiar);
    cajaDetalle.hidden = false;
    cajaDetalle.scrollIntoView({ block: "nearest" });
  }
}

function fechaInput(etiquetaTexto) {
  const etiqueta = document.createElement("label");
  etiqueta.className = "campo-fecha";
  etiqueta.append(document.createTextNode(etiquetaTexto));
  const input = document.createElement("input");
  input.type = "date";
  etiqueta.append(input);
  return { etiqueta, input };
}

function textoDetalle(item) {
  const lineas = [
    (textos.tipos[item.type] || item.type) + " · " + (textos.estados[item.status] || item.status),
    textos.ultima + ": " + fecha(item.started_at),
    textos.duracion + ": " + duracion(item.duration_seconds),
    textos.anadidos + ": " + tamano(item.data_added_bytes),
    textos.destino + ": " + (item.repository || textos.desconocido),
    textos.disparos[item.trigger] || item.trigger || "",
    textos.abiertos + ": " + (item.vss_used ? textos.si : textos.no),
    item.message || ""
  ];
  if (item.snapshot_id) lineas.push(textos.identificador + ": " + item.snapshot_id);
  if (item.retention && item.retention.applied) {
    lineas.push(textos.conservadas + ": " + item.retention.snapshots_removed);
  }
  const omitidos = item.skipped_files || [];
  if (omitidos.length) {
    lineas.push(textos.omitidos + ":");
    omitidos.forEach((fichero) => lineas.push(fichero.path + " — " + fichero.reason));
  }
  const errores = item.errors || [];
  if (errores.length) {
    lineas.push(textos.errores + ":");
    errores.forEach((error) => lineas.push(error));
  }
  return lineas.filter((linea) => linea !== "").join("\n");
}
