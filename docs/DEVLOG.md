# DEVLOG — Book Review Web App (Grupo 3)

Bitácora de desarrollo: tiempos, problemas concretos y decisiones tomadas en
cada fase. Insumo para el informe; no es el informe.

Stack asignado: **Go** + **`net/http`** (stdlib) + **SQLite** + **sin ORM**
(`database/sql` y SQL escrito a mano).

---

## Fase 1 — Scaffolding

- **Tiempo aproximado:** 45m

- **Qué se hizo:**
  - `go mod init bookreviews` con Go 1.26.5. Módulo sin dependencias externas
    todavía: en esta fase el `go.mod` solo declara la versión del lenguaje.
  - Estructura de carpetas por capas: `cmd/{server,seed}`,
    `internal/{database,models,store,handlers,web}`, `web/{templates,static}`,
    `docs`, `data`.
  - `cmd/server/main.go`: parseo del flag `-addr`, logger `log/slog`, armado del
    grafo de dependencias en una función `run(addr, logger) error` y
    `http.Server` con timeouts explícitos (`ReadTimeout` 5s, `WriteTimeout` 10s,
    `IdleTimeout` 60s).
  - Ruteo con el `http.ServeMux` de la stdlib usando patrones con método:
    `mux.HandleFunc("GET /{$}", h.home)` y
    `mux.Handle("GET /static/", http.FileServerFS(web.Files))`.
  - Layout de plantillas: `base.html` con bloques `title` y `content`,
    `partials/nav.html`, y la página `pages/home.html`.
  - `web/static/style.css`: CSS plano (variables CSS, estilos de tabla y de
    formulario). Sin build step, sin framework, sin JS.
  - `internal/web/middleware.go`: `LogRequests` y `RecoverPanic` como
    `func(http.Handler) http.Handler`, encadenados con un `Chain` explícito en
    `main.go`.

- **Cómo se configuró y empezó a usar el "framework":**
  No hay framework: se usa `net/http` de la biblioteca estándar. La
  configuración se reduce a construir un `http.ServeMux`, registrar patrones y
  pasarlo como `Handler` de un `http.Server`. No hubo instalación, generador de
  proyecto ni archivo de configuración; el ruteo con verbo HTTP en el patrón
  (`"GET /{$}"`) existe en la stdlib desde Go 1.22, así que no hizo falta
  gorilla/mux ni chi.

- **Problemas encontrados:**
  1. **Go no estaba instalado en la máquina de desarrollo.** `go version`
     devolvía `zsh: command not found: go`. Se resolvió con
     `brew install go` → Go 1.26.5 (darwin/arm64).
  2. **`embed` falla con directorios vacíos.** Al declarar
     `//go:embed templates static` antes de crear el CSS, el compilador
     rechazó el build con
     `pattern static: cannot embed directory static: contains no embeddable files`.
     `//go:embed` exige que el patrón matchee al menos un archivo, así que el
     paquete `web` no compila hasta que existen los assets reales.
  3. **`ParseFS` con globs de parciales.** `"templates/partials/*.html"` produce
     `pattern matches no files` si la carpeta está vacía, mismo tipo de
     problema que el anterior pero en runtime en vez de en compilación. Por eso
     `nav.html` se creó junto con el layout y no después.

- **Decisiones tomadas y alternativas descartadas:**
  - **Un `*template.Template` por página, en un `map[string]*template.Template`**,
    en vez de un único árbol con todas las plantillas. Motivo: cada página
    define su propio bloque `content`; parseando todo junto, la última
    definición gana y todas las páginas renderizarían la misma. El mapa se
    arma una sola vez al arrancar, así que un error de sintaxis en un template
    revienta en el arranque y no en un request de producción.
  - **Render contra un `bytes.Buffer` antes de escribir al `ResponseWriter`.**
    Si `ExecuteTemplate` falla a mitad de camino, el cliente recibe un 500
    limpio en vez de un HTML truncado con un 200 ya enviado.
  - **Assets embebidos con `embed.FS`** en lugar de leer del disco. El binario
    queda autocontenido y se puede ejecutar desde cualquier directorio; el
    costo es tener que recompilar para ver un cambio de CSS.
  - **`http.FileServerFS` sin `http.StripPrefix`**: como los archivos están en
    `static/…` dentro del FS y la URL es `/static/…`, la ruta coincide y el
    prefijo no se toca. Se descartó `http.FileServer(http.FS(...))` + `StripPrefix`
    por ser una vuelta larga para el mismo resultado.
  - **`log/slog` de la stdlib** en vez de un wrapper propio de logging o de una
    librería externa (zap, logrus).

- **Verificación:** `gofmt -l .` sin salida, `go vet ./...` limpio,
  `go build ./...` OK. Servidor levantado en `:8099`: `GET /` → 200
  `text/html`, `GET /static/style.css` → 200 `text/css` (4507 bytes),
  `GET /nope` → 404.
