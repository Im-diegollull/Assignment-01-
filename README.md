# Book Review Web App — Grupo 3

App web de reseñas de libros. Arquitectura de Software, Universidad de los Andes.

| Ítem | Valor |
|---|---|
| Lenguaje | Go 1.22+ (desarrollado con 1.26.5) |
| Web | `net/http` de la stdlib — sin framework externo |
| Base de datos | SQLite |
| ORM | Ninguno: `database/sql` + SQL escrito a mano |
| Frontend | `html/template` server-rendered + CSS plano |

## Requisitos

Solo Go. El driver de SQLite es puro Go, así que no hace falta CGO ni un
toolchain de C.

```bash
go version   # >= 1.22
```

## Correr la app

```bash
go run ./cmd/server              # escucha en :8080
go run ./cmd/server -addr :3000  # otro puerto
go run ./cmd/server -db /tmp/x.db
```

Luego abrir http://localhost:8080

La base vive en `data/app.db` y el esquema se aplica solo al arrancar, así que
no hay pasos previos. Para aplicarlo sin levantar el servidor:

```bash
go run ./cmd/server -migrate
```

Para empezar de cero, basta con borrar el archivo:

```bash
rm -f data/app.db data/app.db-wal data/app.db-shm
```

## Poblar la base con datos de prueba

```bash
go run ./cmd/seed              # falla si la base ya tiene datos
go run ./cmd/seed --reset      # vacía las tablas y vuelve a sembrar
go run ./cmd/seed -seed 7      # otra semilla, otro dataset
```

Genera 50 autores, 300 libros, entre 1 y 10 reseñas por libro y entre 5 y 12
años de ventas por libro (~1640 reseñas y ~2540 filas de ventas). Tarda menos
de un segundo.

Los datos son **inventados y generados proceduralmente**: se combinan listas de
vocabulario, sin consultar ninguna API externa. Con la misma semilla el
resultado es idéntico, así que todo el grupo ve los mismos rankings en las
tablas.

## Comandos de desarrollo

```bash
gofmt -l .        # no debe imprimir nada
go vet ./...      # debe salir limpio
go build ./...
go test ./...
```

## Estructura

```
cmd/server/       arranque del servidor HTTP
cmd/seed/         generador de datos de prueba
internal/database conexión SQLite + schema.sql
internal/models   structs de dominio y su validación
internal/store    acceso a datos (todo el SQL vive acá)
internal/handlers capa HTTP: parseo, validación y render
internal/web      middleware genérico
web/templates     layout, parciales y páginas
web/static        CSS
docs/DEVLOG.md    bitácora de desarrollo
```

La dependencia va en un solo sentido: `handlers → store → database`, con
`models` en la base. No hay SQL en `handlers` ni `net/http` en `store`.

## Estado

- [x] Fase 1 — Scaffolding
- [x] Fase 2 — Base de datos
- [x] Fase 3 — CRUD de los 4 modelos
- [x] Fase 4 — Seed de datos
- [ ] Fase 5 — Queries + vistas de tablas
- [ ] Fase 6 — Búsqueda paginada
- [ ] Fase 7 — Pulido
