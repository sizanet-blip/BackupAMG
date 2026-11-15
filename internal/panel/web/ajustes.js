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
  raiz.append(titulo, crearSeguridad(), crearPlataforma(), datos, cajaRegistro, cajaSoporte);

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

// Seguridad: la contraseña común a todos los destinos y la hoja de recuperación.
function crearSeguridad() {
  const tx = textos.seguridad;
  const caja = document.createElement("article");
  caja.className = "bloque seguridad";
  const h2 = document.createElement("h2");
  h2.textContent = tx.titulo;
  const ayuda = document.createElement("p");
  ayuda.className = "nota";
  ayuda.textContent = tx.ayuda;
  const aviso = document.createElement("p");
  aviso.className = "peligro";
  aviso.setAttribute("role", "note");
  aviso.textContent = tx.aviso;

  let destinos = [];
  fetch("/api/v1/replicas").then(async (respuesta) => {
    if (!respuesta.ok) return;
    const d = await respuesta.json();
    destinos = [(d.primary && (d.primary.path || d.primary.resolved)) || ""].concat((d.replicas || []).map((c) => c.name + " (" + c.path + ")")).filter(Boolean);
  }).catch(() => {});
  const equipo = () => (document.getElementById("equipo") || {}).textContent || "";

  // Hoja de recuperación
  const hoja = document.createElement("div");
  hoja.className = "hoja-caja";
  const hojaTitulo = document.createElement("h3");
  hojaTitulo.textContent = tx.hojaTitulo;
  const hojaTexto = document.createElement("p");
  hojaTexto.className = "nota";
  hojaTexto.textContent = tx.hojaTexto;
  const accionesHoja = document.createElement("div");
  accionesHoja.className = "barra-filtros";
  const imprimir = document.createElement("button");
  imprimir.type = "button";
  imprimir.className = "secundario";
  imprimir.textContent = textos.imprimir;
  imprimir.addEventListener("click", () => imprimirHoja(equipo(), destinos));
  const descargar = document.createElement("button");
  descargar.type = "button";
  descargar.className = "secundario";
  descargar.textContent = textos.hoja;
  descargar.addEventListener("click", () => descargarHoja(equipo(), destinos));
  accionesHoja.append(imprimir, descargar);
  hoja.append(hojaTitulo, hojaTexto, accionesHoja);

  // Cambiar la contraseña
  const cambio = document.createElement("div");
  cambio.className = "cambio-clave";
  const hCambio = document.createElement("h3");
  hCambio.textContent = tx.cambiarTitulo;
  const ayudaCambio = document.createElement("p");
  ayudaCambio.className = "nota";
  ayudaCambio.textContent = tx.cambiarAyuda;
  const nueva = secreto(textos.nuevaClave);
  const repite = secreto(textos.repiteClave);
  const casilla = document.createElement("label");
  const marca = document.createElement("input");
  marca.type = "checkbox";
  casilla.append(marca, document.createTextNode(textos.guardadaClave));
  const cambiar = document.createElement("button");
  cambiar.type = "button";
  cambiar.className = "secundario";
  cambiar.textContent = textos.cambiarClave;
  const falta = notaFalta();
  const acciones = document.createElement("div");
  acciones.className = "barra-filtros";
  acciones.append(cambiar, falta);
  const resultado = document.createElement("p");
  resultado.className = "resultado";
  resultado.setAttribute("role", "status");
  const revisar = exigir(cambiar, falta, () => {
    if (!nueva.input.value) return textos.faltaClave;
    if (nueva.input.value !== repite.input.value) return textos.noCoinciden + ".";
    if (!marca.checked) return textos.faltaMarca;
    return "";
  });
  [nueva.input, repite.input].forEach((campo) => campo.addEventListener("input", revisar));
  marca.addEventListener("change", revisar);
  cambio.append(hCambio, ayudaCambio, nueva.etiqueta, repite.etiqueta, casilla, acciones, resultado);

  cambiar.addEventListener("click", async () => {
    const total = destinos.length || 1;
    if (!(await confirmarSiNo({
      titulo: tx.confirmarTitulo,
      texto: tx.confirmarTexto.replace("{n}", String(total)),
      aceptar: tx.confirmarBoton,
      peligro: true
    }))) return;
    cambiar.disabled = true;
    resultado.className = "resultado";
    resultado.textContent = tx.cambiando;
    const respuesta = await fetch("/api/v1/password", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ password: nueva.input.value, confirm: repite.input.value, acknowledged: marca.checked })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    if (!respuesta.ok) {
      resultado.className = "resultado aviso";
      resultado.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
      revisar();
      return;
    }
    nueva.input.value = "";
    repite.input.value = "";
    marca.checked = false;
    revisar();
    resultado.className = "resultado aviso-ok";
    resultado.textContent = tx.hecho + (cuerpo.notice ? " " + cuerpo.notice : "");
    if (await confirmarSiNo({ titulo: tx.reimprimirTitulo, texto: tx.reimprimirTexto + (cuerpo.notice ? " " + cuerpo.notice : ""), aceptar: tx.reimprimirBoton, cancelar: tx.ahoraNo })) {
      imprimirHoja(equipo(), destinos);
    }
  });

  caja.append(h2, ayuda, aviso, hoja, cambio);
  return caja;
}

// Plataforma web: estado de la conexión, alta con la clave de licencia y envío manual.
function crearPlataforma() {
  const tx = textos.plataforma;
  const caja = document.createElement("article");
  caja.className = "bloque plataforma";
  const h2 = document.createElement("h2");
  h2.textContent = tx.titulo;
  const ayuda = document.createElement("p");
  ayuda.className = "nota";
  ayuda.textContent = tx.ayuda;

  const fila = document.createElement("div");
  fila.className = "plataforma-estado";
  const pastilla = document.createElement("span");
  pastilla.className = "pastilla";
  const explicacion = document.createElement("p");
  explicacion.className = "nota";
  explicacion.setAttribute("role", "status");
  fila.append(pastilla, explicacion);

  const datos = document.createElement("dl");
  datos.className = "datos plataforma-datos";

  const aviso = document.createElement("p");
  aviso.className = "nota";
  aviso.hidden = true;
  aviso.textContent = tx.altaFichero;

  // Dirección
  const campoDireccion = document.createElement("label");
  campoDireccion.className = "campo-columna";
  campoDireccion.append(document.createTextNode(tx.direccion));
  const direccion = document.createElement("input");
  direccion.type = "text";
  direccion.inputMode = "url";
  direccion.placeholder = tx.direccionPista;
  direccion.autocomplete = "off";
  direccion.spellcheck = false;
  campoDireccion.append(direccion);
  const guardarDireccion = document.createElement("button");
  guardarDireccion.type = "button";
  guardarDireccion.className = "secundario";
  guardarDireccion.textContent = tx.guardarDireccion;

  // Clave
  const campoClave = document.createElement("label");
  campoClave.className = "campo-columna";
  campoClave.append(document.createTextNode(tx.clave));
  const clave = document.createElement("input");
  clave.type = "text";
  clave.placeholder = tx.clavePista;
  clave.autocomplete = "off";
  clave.spellcheck = false;
  campoClave.append(clave);
  const conectar = document.createElement("button");
  conectar.type = "button";
  conectar.textContent = tx.conectar;
  const falta = notaFalta();

  const enviar = document.createElement("button");
  enviar.type = "button";
  enviar.className = "secundario";
  enviar.textContent = tx.enviarAhora;

  const rutas = filaOpcion(tx.rutas, false, () => {});
  const rutasInput = rutas.querySelector("input");
  const rutasAyuda = document.createElement("p");
  rutasAyuda.className = "nota";
  rutasAyuda.textContent = tx.rutasAyuda;

  const resultado = document.createElement("p");
  resultado.className = "resultado";
  resultado.setAttribute("role", "status");

  const accionesDireccion = document.createElement("div");
  accionesDireccion.className = "barra-filtros";
  accionesDireccion.append(guardarDireccion);
  const accionesClave = document.createElement("div");
  accionesClave.className = "barra-filtros";
  accionesClave.append(conectar, falta, enviar);
  caja.append(h2, ayuda, fila, datos, aviso, campoDireccion, accionesDireccion, campoClave, accionesClave, rutas, rutasAyuda, resultado);

  let vista = null;
  let ultimoJson = "";
  let direccionGuardada = "";

  const revisar = exigir(conectar, falta, () => {
    if (!direccion.value.trim()) return tx.faltaDireccion;
    if (!clave.value.trim()) return tx.faltaClave;
    return "";
  });
  direccion.addEventListener("input", revisar);
  clave.addEventListener("input", revisar);

  function fila2(etiqueta, valor) {
    const bloque = document.createElement("div");
    const dt = document.createElement("dt");
    dt.textContent = etiqueta;
    const dd = document.createElement("dd");
    dd.textContent = valor;
    bloque.append(dt, dd);
    datos.append(bloque);
  }

  function pintar() {
    const estado = vista.state;
    pastilla.textContent = tx.estados[estado] || estado;
    pastilla.className = "pastilla " + (estado === "connected" ? "ok" : (estado === "not_configured" || estado === "not_enrolled") ? "" : "aviso");
    const cuando = vista.last_sync_at ? fechaRelativa(vista.last_sync_at) : tx.nunca;
    explicacion.textContent = (tx.ayudasEstado[estado] || "").replace("{cuando}", cuando);
    datos.replaceChildren();
    if (vista.enrolled) {
      if (vista.company) fila2(tx.empresa, vista.company);
      if (vista.license && vista.license.max_machines) {
        fila2(tx.equipos, tx.equiposValor.replace("{usados}", vista.license.used_machines).replace("{max}", vista.license.max_machines));
      }
      fila2(tx.ultima, cuando);
      fila2(tx.pendientes, String(vista.pending || 0));
    }
    datos.hidden = !datos.children.length;
    aviso.hidden = !vista.alta_file;
    if (document.activeElement !== direccion) direccion.value = vista.server_url || "";
    direccionGuardada = vista.server_url || "";
    rutasInput.checked = !!vista.send_paths;
    enviar.disabled = !vista.enrolled;
    // Con el equipo ya conectado, el campo de la clave solo hace falta si hay que volver a conectar.
    const necesitaClave = !vista.enrolled || estado === "needs_enroll";
    campoClave.hidden = !necesitaClave;
    conectar.hidden = !necesitaClave;
    revisar();
    if (!necesitaClave) falta.hidden = true;
  }

  async function recargar() {
    try {
      const respuesta = await fetch("/api/v1/cloud");
      if (!respuesta.ok) {
        if (!vista) explicacion.textContent = tx.errorLeer;
        return;
      }
      const texto = await respuesta.text();
      if (texto === ultimoJson) return;
      ultimoJson = texto;
      vista = JSON.parse(texto);
      pintar();
    } catch (e) {
      if (!vista) explicacion.textContent = tx.errorLeer;
    }
  }

  async function llamar(metodo, ruta, cuerpo) {
    const respuesta = await fetch(ruta, {
      method: metodo,
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: cuerpo ? JSON.stringify(cuerpo) : undefined
    });
    const datosRespuesta = await respuesta.json().catch(() => ({}));
    return { ok: respuesta.ok, cuerpo: datosRespuesta };
  }

  function error(r) {
    return (r.cuerpo.fields || [r.cuerpo.error || textos.error]).join(" ");
  }

  guardarDireccion.addEventListener("click", async () => {
    const nueva = direccion.value.trim();
    if (vista && vista.enrolled && nueva.replace(/\/+$/, "") !== direccionGuardada.replace(/\/+$/, "")) {
      const sigue = await confirmarSiNo({ titulo: tx.cambiarDireccionTitulo, texto: tx.cambiarDireccionTexto, aceptar: tx.cambiarDireccionBoton, peligro: true });
      if (!sigue) return;
    }
    guardarDireccion.disabled = true;
    const r = await llamar("PUT", "/api/v1/cloud/settings", { server_url: nueva });
    guardarDireccion.disabled = false;
    resultado.className = r.ok ? "resultado aviso-ok" : "resultado aviso";
    resultado.textContent = r.ok ? tx.direccionGuardada : error(r);
    ultimoJson = "";
    recargar();
  });

  conectar.addEventListener("click", async () => {
    conectar.disabled = true;
    resultado.className = "resultado";
    resultado.textContent = tx.conectando;
    const cuerpo = { credential: clave.value.trim() };
    if (direccion.value.trim() !== direccionGuardada) cuerpo.server_url = direccion.value.trim();
    const r = await llamar("POST", "/api/v1/cloud/enroll", cuerpo);
    if (!r.ok) {
      resultado.className = "resultado aviso";
      resultado.textContent = error(r);
      revisar();
      return;
    }
    clave.value = "";
    resultado.className = "resultado aviso-ok";
    resultado.textContent = tx.conectado;
    mostrarToast(tx.conectado, true);
    ultimoJson = "";
    recargar();
  });

  enviar.addEventListener("click", async () => {
    enviar.disabled = true;
    resultado.className = "resultado";
    resultado.textContent = tx.enviando;
    const r = await llamar("POST", "/api/v1/cloud/sync");
    if (!r.ok) {
      resultado.className = "resultado aviso";
      resultado.textContent = error(r);
      enviar.disabled = false;
      return;
    }
    setTimeout(() => { resultado.textContent = ""; ultimoJson = ""; recargar(); }, 3500);
    setTimeout(() => { enviar.disabled = !(vista && vista.enrolled); }, 3500);
  });

  rutasInput.addEventListener("change", async () => {
    const r = await llamar("PUT", "/api/v1/cloud/settings", { send_paths: rutasInput.checked });
    if (!r.ok) {
      rutasInput.checked = !rutasInput.checked;
      resultado.className = "resultado aviso";
      resultado.textContent = error(r);
    }
  });

  caja.querySelector("h2").after(ayuda);
  recargar();
  const temporizador = setInterval(() => {
    if (!document.body.contains(caja)) { clearInterval(temporizador); return; }
    if (!document.getElementById("pantalla-ajustes").hidden) recargar();
  }, 5000);
  return caja;
}
