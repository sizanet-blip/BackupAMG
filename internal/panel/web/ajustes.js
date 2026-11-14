function pintarAjustes() {
  const raiz = document.getElementById("pantalla-ajustes");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.ajustes, textos.ajustesAyuda);
  const datos = document.createElement("dl");
  datos.className = "datos";
  const registro = document.createElement("pre");
  registro.className = "registro";
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
  raiz.append(titulo, datos, nivel, ver, registro, generar, nota, aviso);

  function fila(etiqueta, valor) {
    const bloque = document.createElement("div");
    const dt = document.createElement("dt");
    dt.textContent = etiqueta;
    const dd = document.createElement("dd");
    dd.textContent = valor || textos.desconocido;
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
  ver.addEventListener("click", cargarRegistro);
  nivel.addEventListener("change", cargarRegistro);
  cargarRegistro();

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
