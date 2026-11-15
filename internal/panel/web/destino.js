function pintarDestino() {
  const raiz = document.getElementById("pantalla-destino");
  raiz.replaceChildren();
  const titulo = tituloPagina(textos.menu.destino, textos.destinoAyuda);
  const tipo = document.createElement("div");
  const local = radio("local", textos.disco, true);
  const red = radio("unc", textos.red, false);
  tipo.append(local.etiqueta, red.etiqueta);
  const ayuda = document.createElement("p");
  ayuda.className = "nota";
  const campo = document.createElement("label");
  campo.className = "campo-columna";
  campo.append(document.createTextNode(textos.rutaDestino));
  const ruta = document.createElement("input");
  ruta.type = "text";
  campo.append(ruta);
  const notaDisco = document.createElement("p");
  notaDisco.className = "nota";
  notaDisco.hidden = true;
  const ahora = document.createElement("p");
  ahora.className = "nota";
  ahora.hidden = true;
  let rutaGuardada = "";
  const explorador = crearExplorador({ modo: "elegir", alElegir: (carpeta) => { ruta.value = carpeta; alCambiarRuta(); } });
  const explorar = explorador.raiz;
  const resultado = document.createElement("p");
  resultado.className = "resultado";
  const probar = document.createElement("button");
  probar.type = "button";
  probar.className = "secundario";
  probar.textContent = textos.probar;
  const guardar = document.createElement("button");
  guardar.type = "button";
  guardar.textContent = textos.guardar;


  tipo.className = "opciones-tipo";
  const acciones = document.createElement("div");
  acciones.className = "barra-filtros acciones-formulario";
  const faltaDestino = notaFalta();
  const cancelarCambio = document.createElement("button");
  cancelarCambio.type = "button";
  cancelarCambio.className = "secundario";
  cancelarCambio.textContent = textos.cancelar;
  acciones.append(probar, guardar, cancelarCambio, faltaDestino);
  const revisarDestino = exigir(guardar, faltaDestino, () => (ruta.value.trim() ? "" : textos.faltaDestino));
  const alCambiarRuta = () => { revisarDestino(); probar.disabled = !ruta.value.trim(); };
  ruta.addEventListener("input", alCambiarRuta);
  const cajaDestino = document.createElement("article");
  cajaDestino.className = "bloque";
  const hDestino = document.createElement("h2");
  hDestino.textContent = textos.tituloUbicacion;
  // Con un destino ya configurado se ve solo un resumen; el selector se despliega con «Cambiar carpeta».
  const resumen = document.createElement("div");
  resumen.className = "destino-resumen";
  const iconoResumen = document.createElement("span");
  iconoResumen.className = "destino-resumen-icono";
  iconoResumen.innerHTML = iconoSvg("hard-drive");
  const textoResumen = document.createElement("div");
  textoResumen.className = "destino-resumen-texto";
  const rutaResumen = document.createElement("strong");
  const detalleResumen = document.createElement("span");
  detalleResumen.className = "nota";
  textoResumen.append(rutaResumen, detalleResumen, notaDisco, ahora);
  const cambiar = document.createElement("button");
  cambiar.type = "button";
  cambiar.className = "secundario";
  cambiar.textContent = textos.cambiarCarpeta;
  cambiar.setAttribute("aria-expanded", "false");
  cambiar.setAttribute("aria-controls", "editor-destino");
  resumen.append(iconoResumen, textoResumen, cambiar);
  const editor = document.createElement("div");
  editor.id = "editor-destino";
  editor.hidden = true;
  editor.append(tipo, ayuda, campo, explorar, acciones, resultado);
  cajaDestino.append(hDestino, resumen, editor);
  // La contraseña ya no se cambia aquí: protege todos los destinos y vive en Ajustes → Seguridad.
  const cajaEnlaceClave = document.createElement("article");
  cajaEnlaceClave.className = "bloque enlace-clave";
  const textoEnlaceClave = document.createElement("p");
  textoEnlaceClave.className = "nota";
  textoEnlaceClave.textContent = textos.seguridad.enDestinos;
  const irAClave = document.createElement("button");
  irAClave.type = "button";
  irAClave.className = "enlace";
  irAClave.textContent = textos.seguridad.enlace;
  irAClave.addEventListener("click", () => mostrarPantalla("ajustes"));
  cajaEnlaceClave.append(textoEnlaceClave, irAClave);
  cajaDestino.querySelector("h2").textContent = textos.copias.principal;
  raiz.append(titulo, cajaDestino, crearCopiasAdicionales(), cajaEnlaceClave);

  function modo() {
    return red.input.checked ? "unc" : "local";
  }
  function pintarModo() {
    const redActiva = modo() === "unc";
    ayuda.textContent = redActiva ? textos.ayudaRed : textos.ayudaDisco;
    explorar.hidden = redActiva;
    if (!redActiva && !editor.hidden) explorador.cargarSiHaceFalta();
  }
  function pintarResumen(datos) {
    rutaResumen.textContent = datos.resolved || datos.path || "";
    const partes = [];
    if (datos.label) partes.push(datos.label);
    partes.push(datos.connected ? (datos.free_bytes > 0 ? tamano(datos.free_bytes) + " libres" : textos.conectado) : textos.desconectado);
    detalleResumen.textContent = partes.join(" · ");
  }
  function desplegar() {
    editor.hidden = false;
    cambiar.setAttribute("aria-expanded", "true");
    pintarModo();
    ruta.focus();
  }
  function plegar(devolverFoco) {
    editor.hidden = true;
    cambiar.setAttribute("aria-expanded", "false");
    if (devolverFoco) cambiar.focus();
  }
  cambiar.addEventListener("click", () => { if (editor.hidden) desplegar(); else plegar(true); });
  cancelarCambio.addEventListener("click", () => {
    ruta.value = rutaGuardada;
    resultado.textContent = "";
    resultado.className = "resultado";
    alCambiarRuta();
    plegar(true);
  });
  document.addEventListener("keydown", function alEscape(evento) {
    if (!document.body.contains(editor)) { document.removeEventListener("keydown", alEscape); return; }
    if (evento.key === "Escape" && !editor.hidden && editor.contains(document.activeElement) && !document.querySelector(".modal-fondo")) {
      cancelarCambio.click();
    }
  });
  local.input.addEventListener("change", pintarModo);
  red.input.addEventListener("change", pintarModo);

  fetch("/api/v1/destination").then(async (respuesta) => {
    if (!respuesta.ok) {
      resultado.textContent = textos.error;
      return;
    }
    const datos = await respuesta.json();
    ruta.value = datos.path || "";
    rutaGuardada = datos.path || "";
    alCambiarRuta();
    if (datos.kind === "unc") red.input.checked = true;
    else local.input.checked = true;
    notaDisco.hidden = !datos.volume_id;
    notaDisco.textContent = textos.discoId;
    if (datos.resolved && datos.resolved !== datos.path) {
      ahora.hidden = false;
      ahora.textContent = textos.ahoraEn + " " + datos.resolved;
    }
    pintarResumen(datos);
    pintarModo();
    if (!rutaGuardada) desplegar();
  }).catch(() => {
    resultado.textContent = textos.error;
  });

  probar.addEventListener("click", async () => {
    probar.disabled = true;
    resultado.className = "resultado";
    resultado.textContent = textos.probando;
    const respuesta = await fetch("/api/v1/destination/test", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ path: ruta.value })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    probar.disabled = false;
    resultado.className = cuerpo.ok ? "resultado aviso-ok" : "resultado";
    resultado.textContent = cuerpo.message || cuerpo.error || textos.error;
  });

  guardar.addEventListener("click", async () => {
    const limpia = (valor) => String(valor || "").trim().replace(/[\\/]+$/, "").toLowerCase();
    if (rutaGuardada && limpia(ruta.value) !== limpia(rutaGuardada)) {
      const sigue = await confirmarSiNo({
        titulo: textos.cambiarDestinoTitulo,
        texto: textos.cambiarDestinoTexto.replace("{nueva}", ruta.value.trim()).replace("{anterior}", rutaGuardada),
        aceptar: textos.cambiarDestinoBoton,
        peligro: true
      });
      if (!sigue) return;
    }
    guardar.disabled = true;
    const textoGuardar = guardar.textContent;
    guardar.textContent = textos.preparandoDestino;
    resultado.className = "resultado";
    resultado.textContent = "";
    const respuesta = await fetch("/api/v1/destination", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ path: ruta.value })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    guardar.disabled = false;
    guardar.textContent = textoGuardar;
    if (!respuesta.ok) {
      resultado.className = "resultado aviso";
      resultado.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
      return;
    }
    resultado.className = "resultado aviso-ok";
    resultado.textContent = textos.guardado;
    mostrarToast(textos.guardado, true);
    const otra = await fetch("/api/v1/destination");
    if (!otra.ok) return;
    const datos = await otra.json();
    rutaGuardada = datos.path || "";
    notaDisco.hidden = !datos.volume_id;
    notaDisco.textContent = textos.discoId;
    if (datos.resolved && datos.resolved !== datos.path) {
      ahora.hidden = false;
      ahora.textContent = textos.ahoraEn + " " + datos.resolved;
    } else {
      ahora.hidden = true;
    }
    pintarResumen(datos);
    plegar(true);
  });
}

function radio(valor, texto, marcado) {
  const etiqueta = document.createElement("label");
  const input = document.createElement("input");
  input.type = "radio";
  input.name = "tipo-destino";
  input.value = valor;
  input.checked = marcado;
  etiqueta.append(input, document.createTextNode(texto));
  return { etiqueta, input };
}
