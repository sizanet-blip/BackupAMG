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
  const explorar = document.createElement("div");
  explorar.className = "lista";
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
  const casilla = document.createElement("label");
  const marca = document.createElement("input");
  marca.type = "checkbox";
  casilla.append(marca, document.createTextNode(textos.guardadaClave));
  const cambiar = document.createElement("button");
  cambiar.type = "button";
  cambiar.textContent = textos.cambiarClave;
  const resultadoClave = document.createElement("p");
  resultadoClave.className = "resultado";

  raiz.append(titulo, tipo, ayuda, campo, notaDisco, ahora, explorar, probar, guardar, resultado, clave, aviso, nueva.etiqueta, repite.etiqueta, casilla, cambiar, resultadoClave);

  function modo() {
    return red.input.checked ? "unc" : "local";
  }
  function pintarModo() {
    const redActiva = modo() === "unc";
    ayuda.textContent = redActiva ? textos.ayudaRed : textos.ayudaDisco;
    explorar.hidden = redActiva;
    if (!redActiva && !explorar.childElementCount) abrirCarpetaDestino("");
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
    guardar.disabled = true;
    const respuesta = await fetch("/api/v1/destination", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ path: ruta.value })
    });
    const cuerpo = await respuesta.json().catch(() => ({}));
    guardar.disabled = false;
    if (!respuesta.ok) {
      resultado.className = "resultado";
      resultado.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
      return;
    }
    resultado.className = "resultado aviso-ok";
    resultado.textContent = textos.guardado;
    const otra = await fetch("/api/v1/destination");
    if (!otra.ok) return;
    const datos = await otra.json();
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
    resultadoClave.className = "resultado aviso-ok";
    resultadoClave.textContent = textos.claveGuardada;
  });

  async function abrirCarpetaDestino(path) {
    explorar.replaceChildren();
    const respuesta = await fetch("/api/v1/folders?path=" + encodeURIComponent(path));
    if (!respuesta.ok) return;
    const carpetas = await respuesta.json();
    if (path) {
      const usar = document.createElement("button");
      usar.type = "button";
      usar.className = "secundario";
      usar.textContent = textos.usarCarpeta + " " + path;
      usar.addEventListener("click", () => { ruta.value = path; });
      const atras = document.createElement("button");
      atras.type = "button";
      atras.className = "secundario";
      atras.textContent = textos.abrir;
      const padre = path.replace(/[\\/][^\\/]+$/, "");
      atras.addEventListener("click", () => abrirCarpetaDestino(padre.length < 3 ? "" : padre));
      explorar.append(usar, atras);
    }
    (carpetas || []).forEach((carpeta) => {
      const boton = document.createElement("button");
      boton.type = "button";
      boton.className = "secundario";
      boton.textContent = carpeta.name || carpeta.path;
      boton.addEventListener("click", () => abrirCarpetaDestino(carpeta.path));
      explorar.append(boton);
    });
  }
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
  etiqueta.append(document.createTextNode(texto));
  const input = document.createElement("input");
  input.type = "password";
  input.autocomplete = "new-password";
  etiqueta.append(input);
  return { etiqueta, input };
}
