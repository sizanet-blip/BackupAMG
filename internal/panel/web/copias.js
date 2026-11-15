// Copias adicionales: la sección de la pantalla Destinos. Solo administradores.
// Lo que se ve y se dice sale de textos.copias; los datos, de /api/v1/replicas.

function crearCopiasAdicionales() {
  const tx = textos.copias;
  const seccion = document.createElement("article");
  seccion.className = "bloque copias";
  let datos = null;
  let ultimoJson = "";
  let ocupado = false; // hay un asistente o una edición abierta: no se redibuja debajo
  let temporizador = null;

  const cabeza = document.createElement("div");
  cabeza.className = "bloque-cabeza";
  const titulos = document.createElement("div");
  const h2 = document.createElement("h2");
  h2.textContent = tx.titulo;
  const ayuda = document.createElement("p");
  ayuda.className = "nota";
  ayuda.textContent = tx.ayuda;
  titulos.append(h2, ayuda);
  const anadir = document.createElement("button");
  anadir.type = "button";
  anadir.textContent = tx.anadir;
  cabeza.append(titulos, anadir);

  const consejo = document.createElement("p");
  consejo.className = "consejo";
  consejo.hidden = true;
  const lista = document.createElement("div");
  lista.className = "copias-lista";
  const zona = document.createElement("div"); // aquí vive el asistente o el editor
  seccion.append(cabeza, consejo, lista, zona);

  lista.textContent = tx.leyendo;
  anadir.disabled = true;

  function etiquetaTipo(c) {
    if (c.type === "unc" || c.kind === "unc" || c.kind === "remote") return tx.tipoRed;
    if (c.kind === "removable") return tx.tipoUsb;
    return tx.tipoLocal;
  }

  function claseTipo(c) {
    if (c.type === "unc" || c.kind === "unc" || c.kind === "remote") return "red";
    return c.kind === "removable" ? "usb" : "disco";
  }

  function dias(n) {
    return n === 1 ? "día" : "días";
  }

  function estadoVisible(c) {
    if (c.running) return "running";
    return c.state;
  }

  function pastilla(c) {
    const estado = estadoVisible(c);
    const nodo = document.createElement("span");
    const clases = { ok: "ok", pending: c.stale ? "aviso" : "", offline: c.stale ? "aviso" : "", failed: "error", disabled: "", running: "curso" };
    nodo.className = "pastilla " + (clases[estado] || "");
    nodo.textContent = tx.estados[estado] || estado;
    return nodo;
  }

  // Consejo de la regla 3-2-1: informativo, nunca alarma.
  function pintarConsejo() {
    const reales = (datos.replicas || []).filter((c) => c.enabled);
    if (!(datos.replicas || []).length) {
      consejo.textContent = tx.consejoUna;
      consejo.hidden = false;
      return;
    }
    const alDia = reales.filter((c) => c.state === "ok" && !c.running);
    const tipos = new Set([claseTipo(datos.primary || {})]);
    alDia.forEach((c) => tipos.add(claseTipo(c)));
    const fuera = (datos.primary && claseTipo(datos.primary) !== "disco") || alDia.some((c) => claseTipo(c) !== "disco");
    let frase;
    if (!alDia.length) {
      frase = tx.consejoNinguna;
    } else {
      frase = tx.consejoCopias.replace("{n}", alDia.length + 1).replace("{m}", tipos.size);
      if (tipos.size === 1) frase = frase.replace("tipos de disco", "tipo de disco");
      frase += fuera ? tx.consejoFueraOk : tx.consejoFuera;
      if (alDia.length < reales.length) frase += tx.consejoPendientes;
    }
    consejo.textContent = frase;
    consejo.hidden = false;
  }

  function lineaTexto(clase, texto) {
    const p = document.createElement("p");
    p.className = clase;
    p.textContent = texto;
    return p;
  }

  function tarjeta(c) {
    const nodo = document.createElement("div");
    nodo.className = "copia-adicional";
    const fila = document.createElement("div");
    fila.className = "bloque-cabeza";
    const nombre = document.createElement("h3");
    nombre.textContent = c.name;
    fila.append(nombre, pastilla(c));
    nodo.append(fila);
    nodo.append(lineaTexto("nota", etiquetaTipo(c) + " · " + (c.resolved && c.resolved !== c.path ? c.resolved : c.path)));
    nodo.append(lineaTexto("nota", c.last_success_at ? tx.ultima.replace("{cuando}", fechaRelativa(c.last_success_at)) : tx.nunca));
    if (c.connected && c.free_bytes > 0) nodo.append(lineaTexto("nota", tx.libres.replace("{x}", tamano(c.free_bytes))));

    // Qué pasa y qué hacer, con palabras sencillas.
    let causa = "";
    if (c.running) causa = tx.actualizando;
    else if (!c.enabled) causa = tx.pausa;
    else if (c.state === "offline") causa = tx.sinConexion;
    else if (c.state === "failed") causa = c.message ? tx.errorCausa.replace("{causa}", c.message) : tx.errorSin;
    else if (c.state === "pending") causa = c.interrupted > 0 ? tx.prioridad : tx.pendiente;
    if (causa) nodo.append(lineaTexto("causa-copia", causa));
    if (c.stale && c.enabled) nodo.append(lineaTexto("causa-copia aviso-ambar", tx.retraso.replace("{n}", c.days_behind).replace("{dias}", dias(c.days_behind))));

    const resultado = document.createElement("p");
    resultado.className = "resultado";
    resultado.setAttribute("role", "status");

    const acciones = document.createElement("div");
    acciones.className = "barra-filtros acciones-copia";
    const boton = (texto, clase, alPulsar) => {
      const b = document.createElement("button");
      b.type = "button";
      if (clase) b.className = clase;
      b.textContent = texto;
      b.addEventListener("click", alPulsar);
      acciones.append(b);
      return b;
    };
    const actualizar = boton(tx.actualizar, "secundario", async () => {
      actualizar.disabled = true;
      const r = await fetch("/api/v1/replicas/" + encodeURIComponent(c.id) + "/run", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: "{}" });
      const cuerpo = await r.json().catch(() => ({}));
      if (!r.ok) {
        resultado.className = "resultado aviso";
        resultado.textContent = cuerpo.error || textos.error;
        actualizar.disabled = false;
        return;
      }
      mostrarToast(tx.actualizacionIniciada, true);
      recargar();
    });
    actualizar.disabled = !c.enabled || c.running || ocupado;
    const probar = boton(tx.probar, "secundario", async () => {
      probar.disabled = true;
      resultado.className = "resultado";
      resultado.textContent = tx.probando;
      const r = await fetch("/api/v1/replicas/test", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: JSON.stringify({ path: c.path }) });
      const cuerpo = await r.json().catch(() => ({}));
      probar.disabled = false;
      resultado.className = cuerpo.ok ? "resultado aviso-ok" : "resultado aviso";
      resultado.textContent = cuerpo.message || cuerpo.error || textos.error;
    });
    boton(tx.editar, "secundario", () => abrirEditor(c)).disabled = ocupado;
    boton(tx.quitar, "peligro", async () => {
      const sigue = await confirmarSiNo({
        titulo: tx.quitarTitulo,
        texto: tx.quitarTexto.replace("{nombre}", c.name),
        aceptar: tx.quitarBoton,
        peligro: true
      });
      if (!sigue) return;
      const r = await fetch("/api/v1/replicas/" + encodeURIComponent(c.id), { method: "DELETE", headers: { "X-CSRF-Token": csrf } });
      const cuerpo = await r.json().catch(() => ({}));
      if (!r.ok) {
        resultado.className = "resultado aviso";
        resultado.textContent = cuerpo.error || textos.error;
        return;
      }
      mostrarToast(tx.quitada, true);
      recargar();
    });
    nodo.append(acciones, resultado);
    return nodo;
  }

  function pintar() {
    lista.replaceChildren();
    const copias = datos.replicas || [];
    pintarConsejo();
    if (!copias.length) {
      const vacio = document.createElement("div");
      vacio.className = "copias-vacio";
      const t = document.createElement("p");
      t.className = "fuerte";
      t.textContent = tx.vacio;
      vacio.append(t);
      lista.append(vacio);
    } else {
      copias.forEach((c) => lista.append(tarjeta(c)));
    }
    anadir.disabled = ocupado;
  }

  async function recargar() {
    try {
      const r = await fetch("/api/v1/replicas");
      if (!r.ok) {
        if (!datos) lista.textContent = tx.errorLeer;
        return;
      }
      const texto = await r.text();
      if (texto === ultimoJson) return;
      ultimoJson = texto;
      datos = JSON.parse(texto);
      if (!ocupado) pintar();
    } catch (e) {
      if (!datos) lista.textContent = tx.errorLeer;
    }
  }

  // ---- Asistente de 3 pasos --------------------------------------------------------------
  function abrirAsistente() {
    ocupado = true;
    anadir.disabled = true;
    pintar();
    const form = { tipo: "disco", nombre: "", path: "", cuando: "after_backup", hora: "23:00", probado: "" };
    let paso = 0;
    const caja = document.createElement("div");
    caja.className = "copias-asistente";
    zona.replaceChildren(caja);
    const explorador = crearExplorador({ modo: "elegir", alElegir: (carpeta) => { form.path = carpeta; paso2(); } });

    function cerrar() {
      ocupado = false;
      zona.replaceChildren();
      pintar();
    }

    function pie(atras, siguiente) {
      const nodo = document.createElement("div");
      nodo.className = "pie-paso";
      const izquierda = document.createElement("div");
      izquierda.className = "barra-filtros";
      const cancelar = document.createElement("button");
      cancelar.type = "button";
      cancelar.className = "secundario";
      cancelar.textContent = tx.cancelar;
      cancelar.addEventListener("click", cerrar);
      izquierda.append(cancelar);
      if (atras) {
        const volver = document.createElement("button");
        volver.type = "button";
        volver.className = "secundario";
        volver.textContent = tx.atras;
        volver.addEventListener("click", atras);
        izquierda.append(volver);
      }
      nodo.append(izquierda, ...siguiente);
      return nodo;
    }

    function cabecera() {
      const pista = document.createElement("div");
      pista.className = "pasos-track pasos-3";
      const nombres = document.createElement("div");
      nombres.className = "pasos-nombres pasos-3";
      tx.pasos.forEach((nombre, i) => {
        const tramo = document.createElement("span");
        const etiq = document.createElement("span");
        etiq.textContent = (i + 1) + " · " + nombre;
        if (i < paso) { tramo.className = "hecho"; etiq.className = "hecho"; }
        else if (i === paso) { tramo.className = "activo"; etiq.className = "activo"; }
        pista.append(tramo);
        nombres.append(etiq);
      });
      return [pista, nombres];
    }

    function opcion(valor, titulo, texto) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "casilla-carpeta opcion-grande" + (form.tipo === valor ? " activa" : "");
      b.setAttribute("aria-pressed", form.tipo === valor ? "true" : "false");
      const t = document.createElement("strong");
      t.textContent = titulo;
      const d = document.createElement("span");
      d.className = "nota";
      d.textContent = texto;
      b.append(t, d);
      b.addEventListener("click", () => {
        if (form.tipo !== valor) { form.path = ""; form.probado = ""; }
        form.tipo = valor;
        paso1();
      });
      return b;
    }

    function paso1() {
      paso = 0;
      const titulo = document.createElement("h3");
      titulo.textContent = tx.paso1;
      const opciones = document.createElement("div");
      opciones.className = "opciones-grandes";
      opciones.append(opcion("disco", tx.opcionDisco, tx.opcionDiscoAyuda), opcion("red", tx.opcionRed, tx.opcionRedAyuda));
      const siguiente = document.createElement("button");
      siguiente.type = "button";
      siguiente.textContent = tx.siguiente;
      siguiente.addEventListener("click", paso2);
      caja.replaceChildren(...cabecera(), titulo, opciones, pie(null, [siguiente]));
    }

    function paso2() {
      paso = 1;
      const titulo = document.createElement("h3");
      titulo.textContent = tx.paso2;
      const campoNombre = document.createElement("label");
      campoNombre.className = "campo-columna";
      campoNombre.append(document.createTextNode(tx.nombre));
      const nombre = document.createElement("input");
      nombre.type = "text";
      nombre.maxLength = 60;
      nombre.value = form.nombre || (form.tipo === "red" ? tx.nombreRed : tx.nombreDisco);
      nombre.addEventListener("input", () => { form.nombre = nombre.value; });
      campoNombre.append(nombre);
      const campoRuta = document.createElement("label");
      campoRuta.className = "campo-columna";
      campoRuta.append(document.createTextNode(tx.carpeta));
      const ruta = document.createElement("input");
      ruta.type = "text";
      ruta.value = form.path;
      ruta.placeholder = form.tipo === "red" ? "\\\\servidor\\copias" : "F:\\Copias";
      campoRuta.append(ruta);
      const falta = notaFalta();
      const siguiente = document.createElement("button");
      siguiente.type = "button";
      siguiente.textContent = tx.siguiente;
      const revisar = exigir(siguiente, falta, () => (ruta.value.trim() ? "" : tx.faltaCarpeta));
      ruta.addEventListener("input", () => {
        form.path = ruta.value.trim();
        form.probado = "";
        revisar();
      });
      siguiente.addEventListener("click", () => { form.nombre = nombre.value.trim(); form.path = ruta.value.trim(); paso3(); });
      const partes = [...cabecera(), titulo, campoNombre, campoRuta];
      if (form.tipo === "disco") {
        partes.push(explorador.raiz);
        explorador.cargarSiHaceFalta();
      }
      const filaPie = pie(paso1, [falta, siguiente]);
      caja.replaceChildren(...partes, filaPie);
    }

    function paso3() {
      paso = 2;
      const titulo = document.createElement("h3");
      titulo.textContent = tx.paso3;
      const resumen = document.createElement("p");
      resumen.className = "fuerte";
      resumen.textContent = (form.nombre || tx.nombreDisco) + " · " + form.path;

      const cuando = document.createElement("fieldset");
      cuando.className = "cuando-copia";
      const leyenda = document.createElement("legend");
      leyenda.textContent = tx.cuando;
      const cada = radioCopia("cuando-copia", "after_backup", tx.cuandoCada, form.cuando === "after_backup");
      const dia = radioCopia("cuando-copia", "daily_at", tx.cuandoDia, form.cuando === "daily_at");
      const hora = document.createElement("input");
      hora.type = "time";
      hora.value = form.hora;
      hora.setAttribute("aria-label", tx.cuandoDia);
      hora.disabled = form.cuando !== "daily_at";
      dia.etiqueta.append(hora);
      cuando.append(leyenda, cada.etiqueta, dia.etiqueta);
      const alCambiarCuando = () => {
        form.cuando = dia.input.checked ? "daily_at" : "after_backup";
        hora.disabled = !dia.input.checked;
      };
      cada.input.addEventListener("change", alCambiarCuando);
      dia.input.addEventListener("change", alCambiarCuando);
      hora.addEventListener("change", () => { form.hora = hora.value || "23:00"; });

      const misma = lineaTexto("nota", tx.misma);
      const primera = lineaTexto("nota", tx.primera);
      const resultado = document.createElement("p");
      resultado.className = "resultado";
      resultado.setAttribute("role", "status");
      const probar = document.createElement("button");
      probar.type = "button";
      probar.className = "secundario";
      probar.textContent = tx.probar;
      const guardarBtn = document.createElement("button");
      guardarBtn.type = "button";
      guardarBtn.textContent = tx.anadir;
      const falta = notaFalta();
      const revisar = exigir(guardarBtn, falta, () => (form.probado === form.path ? "" : tx.faltaProbar));
      probar.addEventListener("click", async () => {
        probar.disabled = true;
        form.probado = "";
        revisar();
        resultado.className = "resultado";
        resultado.textContent = tx.probando;
        const r = await fetch("/api/v1/replicas/test", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: JSON.stringify({ path: form.path }) });
        const cuerpo = await r.json().catch(() => ({}));
        probar.disabled = false;
        resultado.className = cuerpo.ok ? "resultado aviso-ok" : "resultado aviso";
        resultado.textContent = cuerpo.message || cuerpo.error || textos.error;
        if (cuerpo.ok) form.probado = form.path;
        revisar();
      });
      guardarBtn.addEventListener("click", async () => {
        guardarBtn.disabled = true;
        const texto = guardarBtn.textContent;
        guardarBtn.textContent = tx.anadiendo;
        resultado.className = "resultado";
        resultado.textContent = "";
        const cuerpoEnvio = { name: form.nombre, path: form.path, when: form.cuando, time: form.cuando === "daily_at" ? form.hora : "" };
        const r = await fetch("/api/v1/replicas", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: JSON.stringify(cuerpoEnvio) });
        const cuerpo = await r.json().catch(() => ({}));
        guardarBtn.textContent = texto;
        if (!r.ok) {
          resultado.className = "resultado aviso";
          resultado.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
          revisar();
          return;
        }
        mostrarToast(tx.anadida, true);
        ocupado = false;
        zona.replaceChildren();
        ultimoJson = "";
        recargar();
      });
      const filaPie = pie(paso2, [falta, probar, guardarBtn]);
      caja.replaceChildren(...cabecera(), titulo, resumen, cuando, misma, primera, resultado, filaPie);
    }

    paso1();
    caja.scrollIntoView({ block: "nearest" });
  }

  // ---- Editar: nombre, pausa y cuándo --------------------------------------------------
  function abrirEditor(c) {
    ocupado = true;
    anadir.disabled = true;
    pintar();
    const caja = document.createElement("div");
    caja.className = "copias-asistente";
    const titulo = document.createElement("h3");
    titulo.textContent = tx.editarTitulo;
    const campoNombre = document.createElement("label");
    campoNombre.className = "campo-columna";
    campoNombre.append(document.createTextNode(tx.nombre));
    const nombre = document.createElement("input");
    nombre.type = "text";
    nombre.maxLength = 60;
    nombre.value = c.name;
    campoNombre.append(nombre);
    const carpeta = lineaTexto("nota", tx.carpeta + ": " + c.path + ". " + tx.carpetaFija);
    const activa = filaOpcion(tx.activa, c.enabled, () => {});
    const activaInput = activa.querySelector("input");
    const cuando = document.createElement("fieldset");
    cuando.className = "cuando-copia";
    const leyenda = document.createElement("legend");
    leyenda.textContent = tx.cuando;
    const cada = radioCopia("cuando-editar", "after_backup", tx.cuandoCada, c.when !== "daily_at");
    const dia = radioCopia("cuando-editar", "daily_at", tx.cuandoDia, c.when === "daily_at");
    const hora = document.createElement("input");
    hora.type = "time";
    hora.value = c.time || "23:00";
    hora.setAttribute("aria-label", tx.cuandoDia);
    hora.disabled = c.when !== "daily_at";
    dia.etiqueta.append(hora);
    cuando.append(leyenda, cada.etiqueta, dia.etiqueta);
    const sincronizar = () => { hora.disabled = !dia.input.checked; };
    cada.input.addEventListener("change", sincronizar);
    dia.input.addEventListener("change", sincronizar);
    const resultado = document.createElement("p");
    resultado.className = "resultado";
    resultado.setAttribute("role", "status");
    const cancelar = document.createElement("button");
    cancelar.type = "button";
    cancelar.className = "secundario";
    cancelar.textContent = tx.cancelar;
    const guardarBtn = document.createElement("button");
    guardarBtn.type = "button";
    guardarBtn.textContent = tx.guardarCambios;
    const pie = document.createElement("div");
    pie.className = "pie-paso";
    pie.append(cancelar, guardarBtn);
    caja.append(titulo, campoNombre, carpeta, activa, cuando, resultado, pie);
    zona.replaceChildren(caja);
    caja.scrollIntoView({ block: "nearest" });
    cancelar.addEventListener("click", () => { ocupado = false; zona.replaceChildren(); pintar(); });
    guardarBtn.addEventListener("click", async () => {
      guardarBtn.disabled = true;
      const envio = {
        name: nombre.value.trim() || c.name,
        enabled: activaInput.checked,
        when: dia.input.checked ? "daily_at" : "after_backup",
        time: dia.input.checked ? (hora.value || "23:00") : ""
      };
      const r = await fetch("/api/v1/replicas/" + encodeURIComponent(c.id), { method: "PUT", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: JSON.stringify(envio) });
      const cuerpo = await r.json().catch(() => ({}));
      guardarBtn.disabled = false;
      if (!r.ok) {
        resultado.className = "resultado aviso";
        resultado.textContent = (cuerpo.fields || [cuerpo.error || textos.error]).join(" ");
        return;
      }
      mostrarToast(tx.guardada, true);
      ocupado = false;
      zona.replaceChildren();
      ultimoJson = "";
      recargar();
    });
  }

  function radioCopia(nombre, valor, texto, marcado) {
    const etiqueta = document.createElement("label");
    etiqueta.className = "radio-copia";
    const input = document.createElement("input");
    input.type = "radio";
    input.name = nombre;
    input.value = valor;
    input.checked = marcado;
    etiqueta.append(input, document.createTextNode(texto));
    return { etiqueta, input };
  }

  anadir.addEventListener("click", abrirAsistente);
  recargar();
  // Se refresca sola mientras la pantalla está abierta; se para al salir de ella.
  temporizador = setInterval(() => {
    if (!document.body.contains(seccion)) { clearInterval(temporizador); return; }
    if (!document.getElementById("pantalla-destino").hidden) recargar();
  }, 4000);
  return seccion;
}
