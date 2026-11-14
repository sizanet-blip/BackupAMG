function pintarAsistente(info) {
  const raiz = document.getElementById("pantalla-asistente");
  raiz.hidden = false;
  const datos = {
    paso: 0,
    path: "",
    probado: "",
    sources: [],
    excludes: ["~$*", "*.tmp"],
    times: ["13:30"],
    weekdays: [1, 2, 3, 4, 5, 6, 7],
    password: "",
    confirm: "",
    ack: false
  };
  dibujar();

  function dibujar() {
    raiz.replaceChildren();
    const marca = document.createElement("div");
    marca.className = "marca marca-centro";
    const imagen = document.createElement("img");
    imagen.src = "/marca.svg";
    imagen.width = 40;
    imagen.height = 40;
    imagen.alt = "";
    const nombre = document.createElement("span");
    nombre.textContent = textos.marca;
    marca.append(imagen, nombre);
    const pista = document.createElement("div");
    pista.className = "pasos-track";
    const nombres = document.createElement("div");
    nombres.className = "pasos-nombres";
    textos.pasos.forEach((nombre, i) => {
      const tramo = document.createElement("span");
      const etiq = document.createElement("span");
      etiq.textContent = (i + 1) + " · " + nombre;
      if (i < datos.paso) {
        tramo.className = "hecho";
        etiq.className = "hecho";
      } else if (i === datos.paso) {
        tramo.className = "activo";
        etiq.className = "activo";
      }
      pista.append(tramo);
      nombres.append(etiq);
    });
    const cuerpo = document.createElement("div");
    const aviso = document.createElement("p");
    aviso.className = "resultado";
    raiz.append(marca, pista, nombres, cuerpo, aviso);
    if (datos.paso === 0) pasoDestino(cuerpo, aviso);
    if (datos.paso === 1) pasoClave(cuerpo, aviso);
    if (datos.paso === 2) pasoCopiar(cuerpo, aviso);
    if (datos.paso === 3) pasoCuando(cuerpo, aviso);
    if (datos.paso === 4) pasoResumen(cuerpo, aviso);
  }

  function pasoDestino(cuerpo, aviso) {
    const ayuda = document.createElement("p");
    ayuda.className = "nota";
    ayuda.textContent = textos.ayudaDisco;
    const redAyuda = document.createElement("p");
    redAyuda.className = "nota";
    redAyuda.textContent = textos.ayudaRed;
    const campo = document.createElement("label");
    campo.append(document.createTextNode(textos.rutaDestino));
    const ruta = document.createElement("input");
    ruta.type = "text";
    ruta.value = datos.path;
    campo.append(ruta);
    const explorar = document.createElement("div");
    explorar.className = "lista";
    const probar = document.createElement("button");
    probar.type = "button";
    probar.className = "secundario";
    probar.textContent = textos.probar;
    const siguiente = document.createElement("button");
    siguiente.type = "button";
    siguiente.textContent = textos.siguiente;
    cuerpo.append(ayuda, redAyuda, campo, explorar, probar, siguiente);
    abrir("");
    probar.addEventListener("click", async () => {
      datos.path = ruta.value.trim();
      datos.probado = "";
      probar.disabled = true;
      aviso.textContent = textos.probando;
      const respuesta = await fetch("/api/v1/destination/test", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({ path: datos.path })
      });
      const cuerpoRes = await respuesta.json().catch(() => ({}));
      probar.disabled = false;
      aviso.className = cuerpoRes.ok ? "resultado aviso-ok" : "resultado";
      aviso.textContent = cuerpoRes.message || cuerpoRes.error || textos.error;
      if (cuerpoRes.ok) datos.probado = datos.path;
    });
    siguiente.addEventListener("click", () => {
      datos.path = ruta.value.trim();
      if (!datos.path || datos.probado !== datos.path) {
        aviso.className = "resultado";
        aviso.textContent = textos.probar;
        return;
      }
      datos.paso = 1;
      dibujar();
    });
    async function abrir(path) {
      explorar.replaceChildren();
      const respuesta = await fetch("/api/v1/folders?path=" + encodeURIComponent(path));
      if (!respuesta.ok) return;
      const carpetas = await respuesta.json();
      if (path) {
        const usar = document.createElement("button");
        usar.type = "button";
        usar.className = "secundario";
        usar.textContent = textos.usarCarpeta + " " + path;
        usar.addEventListener("click", () => { ruta.value = path; datos.probado = ""; });
        explorar.append(usar);
      }
      (carpetas || []).forEach((carpeta) => {
        const boton = document.createElement("button");
        boton.type = "button";
        boton.className = "secundario";
        boton.textContent = carpeta.name || carpeta.path;
        boton.addEventListener("click", () => abrir(carpeta.path));
        explorar.append(boton);
      });
    }
  }

  function pasoClave(cuerpo, aviso) {
    const titulo = document.createElement("h2");
    titulo.textContent = textos.creaClave;
    const ayuda = document.createElement("p");
    ayuda.className = "nota";
    ayuda.textContent = textos.creaClaveAyuda;
    const nueva = secreto(textos.nuevaClave, datos.password);
    const fuerza = document.createElement("div");
    fuerza.className = "fuerza";
    for (let i = 0; i < 4; i++) fuerza.append(document.createElement("i"));
    const nivel = document.createElement("em");
    fuerza.append(nivel);
    const repite = secreto(textos.repiteClave, datos.confirm);
    const coinciden = document.createElement("p");
    coinciden.className = "nota";
    const peligro = document.createElement("div");
    peligro.className = "caja-aviso";
    const peligroTitulo = document.createElement("strong");
    peligroTitulo.textContent = textos.avisoClaveTitulo;
    const peligroCuerpo = document.createElement("p");
    peligroCuerpo.textContent = textos.avisoClaveCuerpo;
    peligro.append(peligroTitulo, peligroCuerpo);
    const hoja = document.createElement("div");
    hoja.className = "hoja-caja";
    const hojaTitulo = document.createElement("p");
    hojaTitulo.textContent = textos.hojaTitulo;
    const descargar = document.createElement("button");
    descargar.type = "button";
    descargar.className = "secundario";
    descargar.textContent = textos.hoja;
    const imprimir = document.createElement("button");
    imprimir.type = "button";
    imprimir.className = "secundario";
    imprimir.textContent = textos.imprimir;
    hoja.append(hojaTitulo, imprimir, descargar);
    const casilla = document.createElement("label");
    const marca = document.createElement("input");
    marca.type = "checkbox";
    marca.checked = datos.ack;
    casilla.append(marca, document.createTextNode(textos.guardadaClave));
    const pie = document.createElement("div");
    pie.className = "pie-paso";
    const atras = botonAtras();
    const siguiente = document.createElement("button");
    siguiente.type = "button";
    siguiente.textContent = textos.continuar;
    pie.append(atras, siguiente);
    cuerpo.append(titulo, ayuda, nueva.etiqueta, fuerza, repite.etiqueta, coinciden, peligro, hoja, casilla, pie);
    function revisar() {
      const clave = nueva.input.value;
      const n = fuerzaClave(clave);
      fuerza.className = "fuerza n" + n;
      nivel.textContent = clave ? textos.fuerzaClave[n] : "";
      if (!repite.input.value) {
        coinciden.textContent = "";
        return;
      }
      const igual = clave === repite.input.value;
      coinciden.textContent = igual ? "✓ " + textos.coinciden : textos.noCoinciden;
      coinciden.className = igual ? "nota aviso-ok" : "nota aviso";
    }
    nueva.input.addEventListener("input", revisar);
    repite.input.addEventListener("input", revisar);
    revisar();
    function fuerzaClave(clave) {
      let n = 0;
      if (clave.length >= 8) n++;
      if (clave.length >= 12) n++;
      if (/[a-z]/.test(clave) && /[A-Z]/.test(clave)) n++;
      if (/\d/.test(clave) && /[^A-Za-z0-9]/.test(clave)) n++;
      return n;
    }
    descargar.addEventListener("click", () => descargarHoja(info.hostname, datos.path));
    imprimir.addEventListener("click", () => imprimirHoja(info.hostname, datos.path));
    siguiente.addEventListener("click", () => {
      datos.password = nueva.input.value;
      datos.confirm = repite.input.value;
      datos.ack = marca.checked;
      if (!datos.password || datos.password !== datos.confirm || !datos.ack) {
        aviso.textContent = textos.avisoClave;
        return;
      }
      datos.paso = 2;
      dibujar();
    });
  }

  function pasoCopiar(cuerpo, aviso) {
    const lista = document.createElement("div");
    lista.className = "lista";
    datos.sources.forEach((ruta) => {
      const fila = document.createElement("div");
      fila.className = "fila";
      const nombre = document.createElement("span");
      nombre.textContent = ruta;
      const quitar = document.createElement("button");
      quitar.type = "button";
      quitar.className = "secundario";
      quitar.textContent = textos.quitar;
      quitar.addEventListener("click", () => {
        datos.sources = datos.sources.filter((item) => item !== ruta);
        dibujar();
      });
      fila.append(nombre, quitar);
      lista.append(fila);
    });
    const accesos = document.createElement("div");
    accesos.className = "lista";
    fetch("/api/v1/shortcuts").then(async (respuesta) => {
      if (!respuesta.ok) return;
      const items = await respuesta.json();
      (items || []).forEach((item) => {
        const boton = document.createElement("button");
        boton.type = "button";
        boton.className = "secundario";
        const nombre = (textos.atajos && textos.atajos[item.id]) || item.id;
        boton.textContent = nombre + " · " + item.user;
        boton.addEventListener("click", () => {
          if (!datos.sources.includes(item.path)) datos.sources.push(item.path);
          dibujar();
        });
        accesos.append(boton);
      });
    });
    const explorar = document.createElement("div");
    explorar.className = "lista";
    const exclusiones = document.createElement("div");
    textos.plantillas.forEach((plantilla) => {
      const etiqueta = document.createElement("label");
      const caja = document.createElement("input");
      caja.type = "checkbox";
      caja.checked = datos.excludes.includes(plantilla.patron);
      caja.addEventListener("change", () => {
        if (caja.checked && !datos.excludes.includes(plantilla.patron)) datos.excludes.push(plantilla.patron);
        if (!caja.checked) datos.excludes = datos.excludes.filter((item) => item !== plantilla.patron);
      });
      etiqueta.append(caja, document.createTextNode(plantilla.texto));
      exclusiones.append(etiqueta);
    });
    cuerpo.append(lista, accesos, explorar, exclusiones, botonAtras(), botonSiguiente(() => {
      if (!datos.sources.length) {
        aviso.textContent = textos.sinCarpetas;
        return false;
      }
      return true;
    }));
    abrirCarpetas("");
    async function abrirCarpetas(path) {
      explorar.replaceChildren();
      const respuesta = await fetch("/api/v1/folders?path=" + encodeURIComponent(path));
      if (!respuesta.ok) return;
      const carpetas = await respuesta.json();
      (carpetas || []).forEach((carpeta) => {
        const fila = document.createElement("div");
        fila.className = "fila";
        const abrir = document.createElement("button");
        abrir.type = "button";
        abrir.className = "secundario";
        abrir.textContent = carpeta.name || carpeta.path;
        abrir.addEventListener("click", () => abrirCarpetas(carpeta.path));
        const anadir = document.createElement("button");
        anadir.type = "button";
        anadir.textContent = textos.anadirCarpeta;
        anadir.addEventListener("click", () => {
          if (!datos.sources.includes(carpeta.path)) datos.sources.push(carpeta.path);
          dibujar();
        });
        fila.append(abrir, anadir);
        explorar.append(fila);
      });
    }
  }

  function pasoCuando(cuerpo, aviso) {
    const horas = document.createElement("div");
    horas.className = "horas";
    datos.times.forEach((hora) => {
      const quitar = document.createElement("button");
      quitar.type = "button";
      quitar.className = "secundario";
      quitar.textContent = hora + " · " + textos.quitar;
      quitar.addEventListener("click", () => {
        datos.times = datos.times.filter((item) => item !== hora);
        dibujar();
      });
      horas.append(quitar);
    });
    const nueva = document.createElement("input");
    nueva.type = "text";
    nueva.placeholder = "13:30";
    const anadir = document.createElement("button");
    anadir.type = "button";
    anadir.className = "secundario";
    anadir.textContent = textos.anadirHora;
    anadir.addEventListener("click", () => {
      const hora = nueva.value.trim();
      if (!/^\d{2}:\d{2}$/.test(hora)) {
        aviso.textContent = textos.sinHora;
        return;
      }
      if (!datos.times.includes(hora)) datos.times.push(hora);
      dibujar();
    });
    const dias = document.createElement("div");
    textos.dias.forEach((nombre, i) => {
      const etiqueta = document.createElement("label");
      const caja = document.createElement("input");
      caja.type = "checkbox";
      caja.checked = datos.weekdays.includes(i + 1);
      caja.addEventListener("change", () => {
        if (caja.checked) datos.weekdays.push(i + 1);
        else datos.weekdays = datos.weekdays.filter((dia) => dia !== i + 1);
      });
      etiqueta.append(caja, document.createTextNode(nombre));
      dias.append(etiqueta);
    });
    cuerpo.append(horas, nueva, anadir, dias, botonAtras(), botonSiguiente(() => {
      if (!datos.times.length) {
        aviso.textContent = textos.sinHora;
        return false;
      }
      if (!datos.weekdays.length) {
        aviso.textContent = textos.sinDias;
        return false;
      }
      return true;
    }));
  }

  function pasoResumen(cuerpo, aviso) {
    const nota = document.createElement("p");
    nota.className = "nota";
    nota.textContent = textos.resumenAyuda;
    const lista = document.createElement("div");
    lista.className = "lista";
    [datos.path].concat(datos.sources).concat(datos.times).forEach((linea) => {
      const p = document.createElement("p");
      p.textContent = linea;
      lista.append(p);
    });
    const empezar = document.createElement("button");
    empezar.type = "button";
    empezar.textContent = textos.empezar;
    cuerpo.append(nota, lista, botonAtras(), empezar);
    empezar.addEventListener("click", async () => {
      empezar.disabled = true;
      const respuesta = await fetch("/api/v1/setup", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({
          path: datos.path,
          sources: datos.sources,
          excludes: datos.excludes,
          times: datos.times,
          weekdays: datos.weekdays,
          password: datos.password,
          confirm: datos.confirm,
          acknowledged: datos.ack
        })
      });
      const cuerpoRes = await respuesta.json().catch(() => ({}));
      if (!respuesta.ok) {
        empezar.disabled = false;
        aviso.textContent = (cuerpoRes.fields || [cuerpoRes.error || textos.error]).join(" ");
        return;
      }
      datos.password = "";
      datos.confirm = "";
      location.reload();
    });
  }

  function botonAtras() {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.className = "secundario";
    boton.textContent = textos.atras;
    boton.addEventListener("click", () => { datos.paso -= 1; dibujar(); });
    return boton;
  }

  function botonSiguiente(vale) {
    const boton = document.createElement("button");
    boton.type = "button";
    boton.textContent = textos.siguiente;
    boton.addEventListener("click", () => {
      if (vale && !vale()) return;
      datos.paso += 1;
      dibujar();
    });
    return boton;
  }
}

function textoHoja(equipo, destino) {
  const fecha = new Intl.DateTimeFormat("es", { day: "2-digit", month: "2-digit", year: "numeric" }).format(new Date());
  return [
    "Hoja de recuperación de BackupAgent",
    "",
    "Equipo: " + (equipo || ""),
    "Destino: " + (destino || ""),
    "Fecha: " + fecha,
    "",
    "Contraseña del repositorio (anótala a mano; esta hoja no la incluye):",
    "",
    "________________________________",
    "",
    "Si se pierde la contraseña, las copias no se pueden recuperar.",
    "Guarda esta hoja fuera del equipo."
  ].join("\n");
}

function descargarHoja(equipo, destino) {
  const blob = new Blob([textoHoja(equipo, destino)], { type: "text/plain;charset=utf-8" });
  const enlace = document.createElement("a");
  enlace.href = URL.createObjectURL(blob);
  enlace.download = "recuperacion-backup.txt";
  enlace.click();
  URL.revokeObjectURL(enlace.href);
}

function imprimirHoja(equipo, destino) {
  const ventana = window.open("", "_blank");
  if (!ventana) return;
  const pre = ventana.document.createElement("pre");
  pre.textContent = textoHoja(equipo, destino);
  ventana.document.body.append(pre);
  ventana.print();
}

function secreto(texto, valor) {
  const etiqueta = document.createElement("label");
  etiqueta.append(document.createTextNode(texto));
  const input = document.createElement("input");
  input.type = "password";
  input.autocomplete = "new-password";
  input.value = valor || "";
  etiqueta.append(input);
  return { etiqueta, input };
}
