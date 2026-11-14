function pintarHistorial() {
  const raiz = document.getElementById("pantalla-historial");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.historial, textos.historialAyuda);
  const filtros = document.createElement("div");
  filtros.className = "horas";
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
  filtrar.textContent = textos.filtrar;
  filtros.append(estado, desde.etiqueta, hasta.etiqueta, filtrar);
  const lista = document.createElement("div");
  lista.className = "lista";
  const detalle = document.createElement("div");
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  raiz.append(titulo, filtros, lista, detalle, aviso);
  filtrar.addEventListener("click", cargar);
  cargar();

  async function cargar() {
    lista.replaceChildren();
    detalle.replaceChildren();
    const consulta = new URLSearchParams();
    if (estado.value) consulta.set("status", estado.value);
    if (desde.input.value) consulta.set("from", desde.input.value);
    if (hasta.input.value) consulta.set("to", hasta.input.value);
    const respuesta = await fetch("/api/v1/history?" + consulta.toString());
    if (!respuesta.ok) {
      aviso.textContent = textos.error;
      return;
    }
    const items = await respuesta.json();
    aviso.textContent = "";
    if (!items || !items.length) {
      aviso.textContent = textos.vacio;
      return;
    }
    items.forEach((item) => {
      const boton = document.createElement("button");
      boton.type = "button";
      boton.className = "fila-historial";
      const pastilla = document.createElement("span");
      const clase = item.status === "success" ? "ok" : item.status === "warning" ? "aviso" : "error";
      pastilla.className = "pastilla " + clase;
      pastilla.textContent = textos.estados[item.status] || item.status;
      const tipo = textos.tipos[item.type] || item.type;
      const medio = document.createElement("span");
      medio.textContent = tipo + " · " + tamano(item.data_added_bytes) + " · " + duracion(item.duration_seconds);
      const cuando = document.createElement("span");
      cuando.className = "nota";
      cuando.textContent = fechaRelativa(item.finished_at);
      boton.append(pastilla, medio, cuando);
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
  }
}

function fechaInput(etiquetaTexto) {
  const etiqueta = document.createElement("label");
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
