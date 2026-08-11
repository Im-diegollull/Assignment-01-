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

---

## Fase 2 — Base de datos

- **Tiempo aproximado:** 1h

- **Qué se hizo:**
  - Primera y única dependencia externa del proyecto: `modernc.org/sqlite`
    v1.56.0. Arrastra `modernc.org/libc`, `modernc.org/memory`,
    `modernc.org/mathutil` y `golang.org/x/sys` como indirectas.
  - `internal/database/schema.sql`: DDL de las 4 tablas más los 4 índices sobre
    las FK, todo con `IF NOT EXISTS`. Embebido en el binario con `//go:embed`.
  - `internal/database/db.go`: `Open(ctx, path)` y `Migrate(ctx, db)`.
  - `cmd/server`: flags nuevos `-db` (ruta del archivo, default `data/app.db`) y
    `-migrate` (aplica el esquema y sale).
  - `internal/database/db_test.go`: 6 tests contra una base temporal
    (`t.TempDir()`), que cubren pragmas, idempotencia de la migración,
    rechazo de FK, borrado en cascada, `CHECK` del score y `UNIQUE` de ventas.

- **Cómo se conecta el "framework" con la base de datos:**
  No hay capa de abstracción ni ORM. `database/sql` de la stdlib expone una
  interfaz genérica y el driver la implementa; el driver se enchufa por
  *side-effect import* (`_ "modernc.org/sqlite"`), cuyo `init()` lo registra
  bajo el nombre `"sqlite"`. Desde ahí, `sql.Open("sqlite", dsn)` devuelve un
  `*sql.DB`, que es un **pool de conexiones**, no una conexión: no abre nada
  hasta el primer uso, por eso `Open` hace un `PingContext` explícito para que
  un path inválido falle al arrancar y no en el primer request.
  El binding entre HTTP y SQL es manual: `cmd/server/main.go` abre el `*sql.DB`
  y lo inyecta hacia abajo. No hay `net/http` dentro de `database`.

- **Problemas encontrados:**
  1. **Los pragmas van en el DSN, no como sentencias sueltas.** `*sql.DB` es un
     pool: un `db.Exec("PRAGMA foreign_keys = ON")` afecta solo a la conexión
     que el pool haya entregado en ese momento, y la siguiente consulta puede
     salir por otra conexión sin el pragma. El driver `modernc.org/sqlite`
     acepta `?_pragma=...` en el DSN y los aplica a **cada** conexión nueva, que
     es la única forma correcta.
  2. **SQLite ignora las foreign keys por defecto** (compatibilidad hacia atrás).
     Comprobado sobre la base ya migrada, con el CLI `sqlite3` 3.51.0:
     ```
     $ sqlite3 app.db "INSERT INTO books (author_id, name) VALUES (9999,'Huerfano');"
     insert ACEPTADO, filas=1
     $ sqlite3 app.db "PRAGMA foreign_keys=ON; INSERT INTO books (author_id,name) VALUES (9999,'Huerfano');"
     Error: stepping, FOREIGN KEY constraint failed (19)
     ```
     El mismo INSERT pasa o falla según el pragma. Como un DSN mal escrito no da
     error al abrir (el driver ignora un pragma que no reconoce) y la base
     quedaría aceptando referencias rotas en silencio, `Open` lee de vuelta
     `PRAGMA foreign_keys` y falla si no quedó en 1.
  3. **`url.PathEscape` no sirve para armar el DSN.** Al construir la cadena de
     conexión, escapaba las barras (`data/app.db` → `data%2Fapp.db`) y la ruta
     dejaba de resolver. El path se concatena tal cual; la limitación conocida
     es que un path con `?` o `#` se leería como query o fragmento.
  4. **`//go:embed` es una directiva, no un comentario.** Al limpiar comentarios
     de `db.go` se borró la línea `//go:embed schema.sql`, dejando
     `var schema string` vacío. El código **compila igual** y `Migrate` "tiene
     éxito" sin crear una sola tabla; el error recién aparece más tarde como
     `no such table: authors`. Se restauró la directiva y se agregó una guarda
     explícita en `Migrate` que falla si el esquema embebido viene vacío.

- **Decisiones tomadas y alternativas descartadas:**
  - **`SetMaxOpenConns(1)`.** SQLite admite un solo escritor; con una sola
    conexión el problema desaparece por construcción y nunca aparece
    `database is locked`. Se paga con serializar también las lecturas, lo que a
    esta escala (300 libros, un usuario) no se nota. La alternativa —varias
    conexiones más reintentos ante `SQLITE_BUSY`— agrega complejidad sin
    beneficio medible acá.
  - **DDL idempotente y aplicado en cada arranque**, en vez de un sistema de
    migraciones versionadas (goose, migrate). Con 4 tablas y un esquema fijo, un
    cambio se resuelve borrando `data/app.db` y volviendo a sembrar. El costo:
    no hay forma de evolucionar el esquema conservando datos, y es una decisión
    cara de revertir si el proyecto siguiera creciendo.
  - **`journal_mode=WAL`** en vez del `DELETE` por defecto: lectores y escritor
    no se bloquean entre sí. Efecto colateral visible: aparecen los archivos
    `app.db-wal` y `app.db-shm` junto a la base, y por eso están en `.gitignore`.
  - **Fecha como TEXT ISO-8601.** SQLite no tiene tipo DATE. `'YYYY-MM-DD'`
    ordena bien lexicográficamente y `strftime('%Y', ...)` extrae el año, que es
    lo que necesita la query del top 5 por año de publicación (§5.3).
  - **Índices explícitos sobre las FK.** SQLite indexa la PK pero no las claves
    foráneas; sin `idx_books_author` y compañía, tanto los JOIN de las tablas de
    §5 como el `ON DELETE CASCADE` hacen scan completo.

- **Verificación:** `go vet ./...` limpio y los 6 tests de
  `internal/database` en verde. Sobre la base real generada con
  `go run ./cmd/server -migrate`: `.tables` lista las 4 tablas, los 4 índices
  existen, `PRAGMA journal_mode` devuelve `wal`,
  `PRAGMA foreign_key_list(books)` muestra la FK a `authors` con `CASCADE` en
  delete, y `PRAGMA foreign_key_check` no reporta violaciones.
