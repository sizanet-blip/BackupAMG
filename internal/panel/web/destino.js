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

  const aviso = document.createElement("p");
  aviso.className = "peligro";
  aviso.textContent = textos.avisoClave;
  const clave = document.createElement("h2");
  clave.textContent = textos.cambiarClave;
  const nueva = secreto(textos.nuevaClave);
  const repite = secreto(textos.repiteClave);
  nueva.etiqueta.classList.add("campo-columna");
  repite.etiqueta.classList.add("campo-columna");
  const casilla = document.createElement("label");
  const marca = document.createElement("input");
  marca.type = "checkbox";
  casilla.append(marca, document.createTextNode(textos.guardadaClave));
  const cambiar = document.createElement("button");
  cambiar.type = "button";
  cambiar.className = "secundario";
  cambiar.textContent = textos.cambiarClave;
  const resultadoClave = document.createElement("p");
  resultadoClave.className = "resultado";

  tipo.className = "opciones-tipo";
  const acciones = document.createElement("div");
  acciones.className = "barra-filtros acciones-formulario";
  const faltaDestino = notaFalta();
  acciones.append(probar, guardar, faltaDestino);
  const revisarDestino = exigir(guardar, faltaDestino, () => (ruta.value.trim() ? "" : textos.faltaDestino));
  const alCambiarRuta = () => { revisarDestino(); probar.disabled = !ruta.value.trim(); };
  ruta.addEventListener("input", alCambiarRuta);
  const cajaDestino = document.createElement("article");
  cajaDestino.className = "bloque";
  const hDestino = document.createElement("h2");
  hDestino.textContent = textos.tituloUbicacion;
  cajaDestino.append(hDestino, tipo, ayuda, campo, notaDisco, ahora, explorar, acciones, resultado);
  const cajaClave = document.createElement("article");
  cajaClave.className = "bloque";
  clave.textContent = textos.tituloClave;
  const faltaClave = notaFalta();
  const accionClave = document.createElement("div");
  accionClave.className = "barra-filtros";
  accionClave.append(cambiar, faltaClave);
  const revisarClave = exigir(cambiar, faltaClave, () => {
    if (!nueva.input.value) return textos.faltaClave;
    if (nueva.input.value !== repite.input.value) return textos.noCoinciden + ".";
    if (!marca.checked) return textos.faltaMarca;
    return "";
  });
  [nueva.input, repite.input].forEach((campoClave) => campoClave.addEventListener("input", revisarClave));
  marca.addEventListener("change", revisarClave);
  cajaClave.append(clave, aviso, nueva.etiqueta, repite.etiqueta, casilla, accionClave, resultadoClave);
  raiz.append(titulo, cajaDestino, cajaClave);

  function modo() {
    return red.input.checked ? "unc" : "local";
  }
  function pintarModo() {
    const redActiva = modo() === "unc";
    ayuda.textContent = redActiva ? textos.ayudaRed : textos.ayudaDisco;
    explorar.hidden = redActiva;
    if (!redActiva) explorador.cargarSiHaceFalta();
  }
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
    pintarModo();
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
  });

  cambiar.addEventListener("click", async () => {
    if (!(await confirmarSiNo({
      titulo: textos.cambiarClaveTitulo,
      texto: textos.confirmarClave,
      aceptar: textos.cambiarClaveBoton,
      peligro: true
    }))) return;
    cambiar.disabled = true;
    const respuesta = await fetch("/api/v1/password", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ password: nueva.input.value, confirm: repite.input.value, acknowledged: marca.checked })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    cambiar.disabled = false;
    if (!respuesta.ok) {
      resultadoClave.className = "resultado";
      resultadoClave.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
      return;
    }
    nueva.input.value = "";
    repite.input.value = "";
    marca.checked = false;
    revisarClave();
    resultadoClave.className = "resultado aviso-ok";
    resultadoClave.textContent = textos.claveGuardada;
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

function secreto(texto) {
  const etiqueta = document.createElement("label");
  etiqueta.className = "campo-columna";
  etiqueta.append(document.createTextNode(texto));
  const input = document.createElement("input");
  input.type = "password";
  input.autocomplete = "new-password";
  etiqueta.append(input);
  return { etiqueta, input };
}
