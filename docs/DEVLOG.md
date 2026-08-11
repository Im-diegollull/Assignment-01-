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

---

## Fase 3 — CRUD de los 4 modelos

- **Tiempo aproximado:** 3h 30m

- **Qué se hizo:**
  - `internal/models`: `Author`, `Book`, `Review`, `Sale`, cada uno con
    `Validate() Errors`, más `validation.go` con los chequeos compartidos
    (obligatorio, largo máximo, fecha ISO, año razonable, no negativo).
  - `internal/store`: un sub-store por agregado con `List/Get/Create/Update/Delete`,
    el sentinel `ErrNotFound`, el helper `inTx` para las operaciones
    multi-tabla y `Page` para la paginación de los listados.
  - `internal/handlers`: 28 rutas (7 por recurso), el helper `form` para parsear
    POST y acumular errores, y `render` con re-dibujo del formulario ante datos
    inválidos.
  - 11 plantillas nuevas: list/show/form por recurso más los parciales
    `pagination` y `fieldError`.
  - 9 tests de store nuevos, incluyendo los del recálculo de ventas.

- **CRUD y vistas:** los 4 modelos tienen list, show, new, edit y delete. Los
  listados paginan de a 20 con `?page=N`. La ficha del autor lista sus libros; la
  del libro lista sus reseñas y sus ventas por año, con accesos directos a
  `/reviews/new?book_id=N` y `/sales/new?book_id=N`.

- **Problemas encontrados:**
  1. **Un campo homónimo del struct queda tapado al embeber.** `models.Review`
     tenía un campo `Review string`, y el view model `ReviewWithBook` embebe
     `models.Review`. En ese caso `r.Review` resuelve al struct embebido y no al
     texto, así que `rows.Scan(&r.Review, ...)` compilaba pero fallaba en
     runtime con:
     ```
     sql: Scan error on column index 2, name "COALESCE(r.review, '')":
     unsupported Scan, storing driver.Value type string into type *models.Review
     ```
     Se renombró el campo a `Text` (la columna sigue llamándose `review`).
  2. **Un proceso viejo del servidor secuestró el puerto de prueba.** La primera
     corrida del smoke test dio 404 en las 28 rutas nuevas mientras que `/`
     respondía 200: un binario de la Fase 2, huérfano de una prueba anterior,
     seguía escuchando en `:8099`. Como `curl` devuelve exit 0 igual ante un 404,
     el bucle de espera lo dio por bueno. Se verificó con
     `lsof -nP -iTCP:8123 -sTCP:LISTEN` que el PID que escucha es el del binario
     recién compilado.
  3. **`http.Values` no existe**; el tipo es `url.Values`, de `net/url`.
  4. Un autocorrector del editor convirtió `''` en comillas tipográficas dentro
     de un comentario de `author.go`. Ahí era inocuo, pero el mismo reemplazo
     dentro de una cadena SQL habría roto la query en silencio. Se revisó todo
     el repo con `grep -rn $'[‘’“”]' --include=*.go --include=*.sql`.

- **Decisiones tomadas y alternativas descartadas:**
  - **Los inputs del HTML se llaman igual que el campo del struct**
    (`name="NumberOfSales"`, no `name="number_of_sales"`). Así la clave del
    input, la del mapa de errores y el nombre del campo son la misma cadena y no
    hace falta una tabla de traducción entre las tres. Es menos idiomático en
    HTML y es lo que se paga a cambio.
  - **Un `map[string]string` de errores por campo** en vez de una lista de
    mensajes: la plantilla necesita poner el error debajo de su input.
  - **Los errores de parseo le ganan a los de validación** cuando caen en el
    mismo campo. Si el usuario escribe "abc" donde va un número, el valor llega
    como 0 y el dominio diría "no puede ser negativo", que es un mensaje
    engañoso; el de parseo describe el problema real.
  - **Los formularios inválidos responden 422** (Unprocessable Entity) y
    re-renderizan con lo ya escrito, en vez de redirigir y perder los datos.
    Los POST exitosos responden **303 See Other**, para que recargar la página
    siguiente no reenvíe el formulario.
  - **`ErrNotFound` como sentinel del store**, traducido a 404 solo en el
    handler con `errors.Is`. `store` no conoce códigos HTTP. `UPDATE` y `DELETE`
    sobre un id inexistente devuelven `ErrNotFound` mirando `RowsAffected`, para
    que se comporten igual que un `Get` fallido.
  - **Un id no numérico en la URL es 404 y no 400**: `/authors/abc` simplemente
    no corresponde a ningún recurso.
  - **El recálculo de `number_of_sales` corre dentro de la transacción** que
    modifica `sales_by_year`, no después. Si la venta se mueve de un libro a
    otro se recalculan **los dos**: recalcular solo el destino dejaría al libro
    de origen contando ventas que ya no le pertenecen.
  - **El año duplicado se detecta con un `SELECT` previo dentro de la
    transacción**, no interpretando el mensaje del `UNIQUE`. Evita acoplar el
    store al texto de error del driver y permite devolver un mensaje por campo.
    El `UNIQUE` del esquema queda como red de seguridad.
  - **`ORDER BY name COLLATE NOCASE`** en los listados: sin eso SQLite ordena
    por bytes y manda todas las mayúsculas antes que las minúsculas.
  - **JS solo en los `confirm()` de borrado.** Es el único JavaScript del
    proyecto. La alternativa sin JS era una página intermedia de confirmación
    por cada recurso: 4 rutas y 4 plantillas más para el mismo resultado.
  - Se **descartó** un handler CRUD genérico por reflexión para los 4 recursos.
    La duplicación entre `authors.go` y `books.go` es real pero honesta, y cada
    recurso terminó necesitando algo propio (el `<select>` de autores, la
    preselección por `?book_id=`, el error de año duplicado).

- **Verificación:** `gofmt -l .` sin salida, `go vet ./...` limpio, 15 tests
  en verde (6 de `database`, 9 de `store`). Smoke test de extremo a extremo
  contra el servidor real, sobre una base temporal: **39 comprobaciones, 0
  fallas**, sin un solo `level=ERROR` en el log. Cubre los 4 listados, alta,
  edición y borrado, los 404, la validación con re-render conservando lo
  escrito, la paginación (25 autores → 20 + 5, "21–25 de 25"), el rechazo del
  año duplicado y el recálculo del total del libro en las cuatro situaciones
  (dos altas → 3800, edición → 2800, borrado → 500, rechazo → sin cambios).

---

## Fase 4 — Seed de datos

- **Tiempo aproximado:** 1h 45m

- **Qué se hizo:**
  - `cmd/seed` con tres flags: `-db`, `--reset` y `-seed` (semilla, default 42).
  - `corpus.go`: listas de vocabulario (nombres, apellidos, países, sustantivos
    y lugares para títulos, plantillas de resumen, reseñas por tramo de puntaje).
  - `generate.go`: el generador, con toda la aleatoriedad concentrada en un
    único `*rand.Rand`.
  - `internal/store/bulk.go`: `CreateMany` por entidad, `Count`, `Reset` y
    `RecalculateAllBookSales`.
  - Dataset resultante: **50 autores, 300 libros, 1640 reseñas
    (1–10 por libro), 2542 filas de ventas (5–12 años por libro)**.

- **Origen de los datos:** inventados y generados proceduralmente combinando
  listas de vocabulario. **No se consulta ninguna API externa** ni se copian
  datos reales; el profesor confirmó que la forma de poblar la base es decisión
  del grupo. La ventaja para la corrección es que el dataset es reproducible:
  con la semilla 42, cualquiera del grupo ve exactamente los mismos rankings.

- **Problemas encontrados:**
  1. **Un helper de capitalización que sorteaba dos veces.** Estaba escrito como
     `strings.ToUpper(pick(g, titleNouns)[:1]) + pick(g, titleNouns)[1:]`, con
     dos llamadas a `pick`: tomaba la primera letra de una palabra y el resto de
     **otra**. Produjo títulos como *"Aértigo en las afueras"* (la `A` de
     "atlas" + "értigo" de "vértigo") y *"Raro en Bahía Negra"* (`R` de "reloj"
     + "aro" de "faro"). Se reemplazó por una función `capitalize(s)` que recibe
     la cadena ya elegida.
  2. **Cortar UTF-8 por byte.** El mismo helper usaba `s[:1]`, que con palabras
     acentuadas parte el carácter al medio. `capitalize` convierte a `[]rune`.
  3. **Concordancia de género.** Los nombres de pila mezclan masculino y
     femenino y el generador no sabe cuál le tocó a cada autor, así que salían
     descripciones como *"Benjamín Ossandón — Traductora y narradora dedicado
     a…"*. Se dejaron solo formas invariables ("novelista", "cronista") y se
     reescribieron los rasgos gendered (*"Se lo asocia con…"* →
     *"Su nombre se asocia con…"*). Lo mismo en los resúmenes: se sacaron los
     participios que concuerdan con el personaje (*"deja a una topógrafa
     varado"*).
  4. **Contracciones del español.** Los lugares del corpus llevan artículo
     ("el sur", "el barrio Franklin"), así que las plantillas producían *"volver
     a el barrio Franklin"*. Se agregó `contractPrepositions`, que reemplaza
     `" a el "` por `" al "` y `" de el "` por `" del "`. El espacio final del
     patrón es lo que evita romper "de ella" o "a ellos".
  5. **Resúmenes que empezaban en minúscula**, porque varias plantillas abren
     con el personaje (*"una traductora acepta…"*). Se capitaliza la frase ya
     armada.

- **Decisiones tomadas y alternativas descartadas:**
  - **El seed no escribe SQL.** Sembrar con los `Create` de a uno serían ~4500
    transacciones, y cada COMMIT en SQLite es un fsync. Se agregó
    `internal/store/bulk.go` con un `CreateMany` por entidad: una transacción y
    un prepared statement reutilizado para todas las filas. El seed completo
    tarda **~0.3 s**. La alternativa —dejar que `cmd/seed` arme sus propios
    INSERT— habría roto la regla de que todo el SQL vive en `store`.
  - **`Reset` también limpia `sqlite_sequence`.** Es donde AUTOINCREMENT guarda
    el último id entregado. Sin eso, volver a sembrar con la misma semilla da
    los mismos datos pero con ids corridos, y los ids aparecen en las URLs.
  - **Sin `--reset`, el seed se niega a correr** si la base tiene datos, en vez
    de insertar encima. Correrlo dos veces sin querer duplicaría los 300 libros.
  - **Dos rasgos ocultos por libro**, `quality` y `popularity`, que no se
    persisten y solo sesgan la generación. Sin un sesgo por libro, todos los
    promedios de puntaje convergerían a 3 y el top 10 de la §5.2 sería un empate
    masivo resuelto por el desempate. `popularity` es exponencial y se eleva al
    cuadrado, para que haya pocas superventas y una cola larga de libros
    discretos: las ventas van de **801 a 797.208**.
  - **Ventas decrecientes desde el año de publicación** (cae entre 45% y 75%
    por año, con ruido), en vez de uniformes. Es lo que hace que la pregunta del
    top 5 por año tenga sentido.
  - **Ventana de publicación 1962–2020**, no 1900–2026. Los libros siguen
    vendiendo 5–12 años después de publicarse, así que un año calendario
    cualquiera tiene decenas de libros compitiendo y el "top 5 del año" es
    selectivo de verdad: **109 de 300 libros (36%)** entran, ni todos ni casi
    ninguno.
  - **Reparto despareja de libros por autor** (primero uno a cada autor, el
    resto por sorteo con pesos): la tabla de la §5.1 tiene autores de 1 libro y
    autores de 17. Con reparto uniforme, ordenar por "N° de libros" no mostraría
    nada.

- **Verificación:**
  - Requisitos del enunciado, sobre la base sembrada: 50 autores, 300 libros,
    reseñas por libro entre 1 y 10 sin libros huérfanos, años de ventas por
    libro entre 5 y 12, y **0 libros con `number_of_sales` distinto de
    `SUM(sales_by_year.sales)`**. `PRAGMA foreign_key_check` sin violaciones y
    0 reseñas con score fuera de 1–5.
  - **Determinismo:** dos bases sembradas por separado con la semilla 42 dan el
    mismo SHA-1 sobre el volcado ordenado de las 4 tablas
    (`1f4f9b0d…`); con `-seed 7` el hash cambia.
  - **Idempotencia:** `--reset` sobre una base ya sembrada reproduce ese mismo
    hash, ids incluidos. Sin `--reset` sale con código 1, el mensaje
    `la base ya tiene datos (50 autores, 300 libros, …); usa --reset` y deja las
    300 filas intactas.
  - App levantada contra los datos sembrados: los 4 listados y las páginas
    profundas (`/authors?page=3`, `/books?page=15`) responden 200, sin un solo
    `level=ERROR` en el log.
