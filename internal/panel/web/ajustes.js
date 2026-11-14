function pintarAjustes() {
  const raiz = document.getElementById("pantalla-ajustes");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.ajustes, textos.ajustesAyuda);
  const datos = document.createElement("div");
  datos.className = "tarjetas-datos";
  const registro = document.createElement("pre");
  registro.className = "registro";
  registro.hidden = true;
  const aviso = document.createElement("p");
  aviso.className = "resultado";
  const nivel = document.createElement("select");
  [["", textos.nivelesLog.todos], ["info", textos.nivelesLog.info], ["warn", textos.nivelesLog.warn], ["error", textos.nivelesLog.error]].forEach((opcion) => {
    const item = document.createElement("option");
    item.value = opcion[0];
    item.textContent = opcion[1];
    nivel.append(item);
  });
  const ver = document.createElement("button");
  ver.type = "button";
  ver.className = "secundario";
  ver.textContent = textos.verRegistro;
  const generar = document.createElement("button");
  generar.type = "button";
  generar.textContent = textos.generarPaquete;
  const nota = document.createElement("p");
  nota.className = "nota";
  nota.textContent = textos.paqueteNota;
  const cajaRegistro = document.createElement("article");
  cajaRegistro.className = "bloque";
  const hRegistro = document.createElement("h2");
  hRegistro.textContent = textos.tituloRegistro;
  const ayudaRegistro = document.createElement("p");
  ayudaRegistro.className = "nota";
  ayudaRegistro.textContent = textos.ayudaRegistro;
  const accionesRegistro = document.createElement("div");
  accionesRegistro.className = "barra-filtros";
  accionesRegistro.append(ver, nivel);
  nivel.hidden = true;
  cajaRegistro.append(hRegistro, ayudaRegistro, accionesRegistro, registro);
  const cajaSoporte = document.createElement("article");
  cajaSoporte.className = "bloque";
  const hSoporte = document.createElement("h2");
  hSoporte.textContent = textos.tituloSoporte;
  cajaSoporte.append(hSoporte, nota, generar, aviso);
  raiz.append(titulo, datos, cajaRegistro, cajaSoporte);

  function fila(etiqueta, valor) {
    const bloque = document.createElement("article");
    bloque.className = "dato";
    const dt = document.createElement("h2");
    dt.textContent = etiqueta;
    const dd = document.createElement("p");
    dd.className = "fuerte dato-valor";
    dd.textContent = valor || textos.desconocido;
    dd.title = valor || "";
    bloque.append(dt, dd);
    datos.append(bloque);
  }

  fetch("/api/v1/diagnostics").then(async (respuesta) => {
    if (!respuesta.ok) {
      aviso.textContent = textos.error;
      return;
    }
    const info = await respuesta.json();
    fila(textos.versionAgente, info.agent_version);
    fila(textos.versionRestic, info.restic_version);
    fila(textos.rutaDatos, info.data_dir);
    fila(textos.puerto, info.panel_port ? String(info.panel_port) : "");
  });

  async function cargarRegistro() {
    const respuesta = await fetch("/api/v1/diagnostics/log?level=" + encodeURIComponent(nivel.value));
    const cuerpo = await respuesta.json().catch(() => ({}));
    if (!respuesta.ok) {
      registro.textContent = cuerpo.error || textos.error;
      return;
    }
    const lineas = cuerpo.lines || [];
    registro.textContent = lineas.length ? lineas.join("\n") : textos.sinRegistro;
  }
  ver.addEventListener("click", () => {
    const abrir = registro.hidden;
    registro.hidden = !abrir;
    nivel.hidden = !abrir;
    ver.textContent = abrir ? textos.ocultarRegistro : textos.verRegistro;
    if (abrir) cargarRegistro();
  });
  nivel.addEventListener("change", cargarRegistro);

  generar.addEventListener("click", async () => {
    generar.disabled = true;
    aviso.textContent = textos.generando;
    const respuesta = await fetch("/api/v1/diagnostics/bundle");
    if (!respuesta.ok) {
      generar.disabled = false;
      const cuerpo = await respuesta.json().catch(() => ({}));
      aviso.className = "resultado";
      aviso.textContent = cuerpo.error || textos.error;
      return;
    }
    const blob = await respuesta.blob();
    const enlace = document.createElement("a");
    enlace.href = URL.createObjectURL(blob);
    enlace.download = "diagnostico.zip";
    enlace.click();
    setTimeout(() => URL.revokeObjectURL(enlace.href), 1500);
    generar.disabled = false;
    aviso.className = "resultado aviso-ok";
    aviso.textContent = textos.paqueteListo;
  });
}
