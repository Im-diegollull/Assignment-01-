# Book Review Web App — Grupo 3

Students:
Cristobal Gazitua
Diego Giordano
Diego LLull
Carlos Renocret


App web de reseñas de libros. 

| Ítem | Valor |
|---|---|
| Lenguaje | Go 1.22+ (desarrollado con 1.26.5) |
| Web | `net/http` de la stdlib — sin framework externo |
| Base de datos | SQLite |
| ORM | Ninguno: `database/sql` + SQL escrito a mano |
| Frontend | `html/template` server-rendered + CSS plano |
| Contenedores | Docker multi-stage + docker compose |
| Orquestación | Manifiestos de Kubernetes, probados en k3d local |

## Requisitos

Para correr la app directo con Go: solo Go. El driver de SQLite es puro Go,
así que no hace falta CGO ni un toolchain de C.

```bash
go version   # >= 1.22
```

Para correr con contenedores (opcional, ver más abajo): Docker y Docker
Compose. Para desplegar en Kubernetes local (opcional): `kubectl` y `k3d`
(o `minikube`).

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

El puerto y la ruta del archivo también se pueden fijar por variable de
entorno (`PORT`, `DB_PATH`) en vez de flag — es lo que usan los contenedores
más abajo. Un flag explícito siempre le gana al valor de la variable.

Hay un endpoint de salud, `GET /healthz`, que hace un ping real a la base
(200 si responde, 503 si no). Lo usan las probes de Kubernetes; también sirve
para chequear a mano que el servidor está vivo.

## Poblar la base con datos de prueba

```bash
go run ./cmd/seed              # falla si la base ya tiene datos
go run ./cmd/seed --reset      # vacía las tablas y vuelve a sembrar
go run ./cmd/seed -seed 7      # otra semilla, otro dataset
```

Genera 50 autores, 300 libros, entre 1 y 10 reseñas por libro y entre 5 y 12
años de ventas por libro (1633 reseñas y 2492 filas de ventas). Tarda menos
de un segundo.

Los datos son **inventados y generados proceduralmente**: se combinan listas de
vocabulario, sin consultar ninguna API externa. Con la misma semilla el
resultado es idéntico, así que todo el grupo ve los mismos rankings en las
tablas.

## Correr con Docker

La imagen es multi-stage: build con `golang:1.26.5-alpine` (coincide con la
versión de `go.mod`) y `CGO_ENABLED=0` porque el driver de SQLite es pure-Go,
sin gcc ni musl-dev; la imagen final es `alpine:3.20` corriendo como usuario
no-root. Templates, CSS y el esquema SQL van embebidos en el binario
(`//go:embed`), así que la imagen final no copia nada aparte de los dos
binarios (`server`, `seed`).

```bash
docker build -t bookreviews:local .
docker run --rm -v bookreviews_data:/data bookreviews:local -migrate  # aplica el esquema
docker run --rm --entrypoint /app/seed -v bookreviews_data:/data bookreviews:local  # siembra
docker run -d --name bookreviews-app -p 8080:8080 -v bookreviews_data:/data bookreviews:local
```

Luego abrir http://localhost:8080. El puerto y la ruta de la base se pueden
cambiar con `-e PORT=3000 -e DB_PATH=/data/otra.db`.

## Correr con Docker Compose

```bash
cp .env.example .env   # sin secretos: solo APP_PORT y DB_PATH
docker compose up
```

Dos servicios:

- **`db`** — corre `-migrate` y el seed una sola vez, y **termina** (exit 0).
  No expone ningún puerto: SQLite no tiene proceso servidor al que
  conectarse por red, así que este contenedor solo existe para inicializar
  el volumen. `app` espera a que `db` termine con éxito
  (`depends_on: condition: service_completed_successfully`), no a que quede
  levantado. Reiniciar el stack no duplica datos: el seed se niega a
  insertar de nuevo si el volumen ya tiene contenido.
- **`app`** — el servidor, publicado en `${APP_PORT}` (8080 por defecto).

Ambos montan el volumen nombrado `sqlite-data` en la misma ruta interna
(`/data`), así que el archivo persiste independiente del ciclo de vida de
cualquiera de los dos contenedores. El propio `docker-compose.yml` trae un
comentario explicando por qué SQLite no admite una separación
cliente-servidor real y qué implica eso para escalar `app` a más de una
réplica (spoiler: no se puede sin cambiar de motor — SQLite es de un solo
escritor por archivo).

## Desplegar en Kubernetes (k3d)

Los manifiestos viven en `k8s/`. Sin registry externo: la imagen se importa
directo al clúster local.

```bash
docker build -t bookreviews:local .
k3d cluster create book-review          # o el clúster que ya tengas
k3d image import bookreviews:local -c book-review

kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/pvc.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

kubectl -n book-review-app get pods            # debe quedar 1/1 Running
kubectl -n book-review-app get pvc             # debe quedar Bound

# el PVC nace vacío; sembrar una vez contra el pod ya levantado
POD=$(kubectl -n book-review-app get pod -l app.kubernetes.io/name=book-review-app -o jsonpath='{.items[0].metadata.name}')
kubectl -n book-review-app exec "$POD" -- /app/seed

kubectl -n book-review-app port-forward svc/book-review-app 8080:8080
```

Luego abrir http://localhost:8080.

El `Deployment` corre con `replicas: 1` a propósito, con un comentario en el
propio YAML explicando por qué: el PVC es `ReadWriteOnce` porque SQLite
tolera un solo escritor por archivo, así que subir el número de réplicas no
escala nada — corrompe la base. El PVC no depende del ciclo de vida del pod:
borrar el pod a mano (`kubectl delete pod`) hace que el ReplicaSet lo
recree solo, con los mismos datos, sin volver a sembrar.

Nota: al hacer `port-forward` contra un pod específico (en vez de contra el
`Service`, como en el comando de arriba), la conexión no se re-targetea sola
si ese pod muere y Kubernetes lo reemplaza — hay que reabrirlo.

## Comandos de desarrollo

```bash
gofmt -l .        # no debe imprimir nada
go vet ./...      # debe salir limpio
go build ./...
go test ./...
```

## Estructura

```
cmd/server/          arranque del servidor HTTP
cmd/seed/            generador de datos de prueba
internal/database    conexión SQLite + schema.sql
internal/models      structs de dominio y su validación
internal/store       acceso a datos (todo el SQL vive acá)
internal/handlers    capa HTTP: parseo, validación, render y /healthz
internal/web         middleware genérico
web/templates        layout, parciales y páginas
web/static           CSS
docs/DEVLOG.md       bitácora de desarrollo
Dockerfile           build multi-stage: build → db-admin → final
docker/              entrypoint del contenedor "db" (migrate + seed)
docker-compose.yml   servicios "app" y "db", volumen sqlite-data
.env.example         plantilla de variables (sin secretos)
k8s/                 namespace, configmap, secret, pvc, deployment, service
```

La dependencia va en un solo sentido: `handlers → store → database`, con
`models` en la base. No hay SQL en `handlers` ni `net/http` en `store`.

## Estado

- [x] Fase 1 — Scaffolding
- [x] Fase 2 — Base de datos
- [x] Fase 3 — CRUD de los 4 modelos
- [x] Fase 4 — Seed de datos
- [x] Fase 5 — Queries + vistas de tablas
- [x] Fase 6 — Búsqueda paginada
- [x] Fase 7 — Containerización (Docker + Docker Compose)
- [x] Fase 8 — Despliegue en Kubernetes (k3d)
- [ ] Fase 9 — Pulido
