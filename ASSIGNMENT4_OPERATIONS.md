# Assignment 4: comandos y validacion tecnica

Este archivo es una guia operativa del codigo, no el informe de entrega.

## Configuracion

| Variable | Default local | Uso |
|---|---|---|
| `DB_PATH` | `data/app.db` | SQLite; compartido en los despliegues |
| `MEDIA_ROOT` | `data/media` | Directorio de imagenes |
| `USE_REVERSE_PROXY` | `false` | `true` desactiva `/static/` y `/media/` en Go |
| `REDIS_ADDR` | vacio | Cache; requerido cuando las sesiones usan Redis |
| `SESSION_BACKEND` | `off` | `redis` activa sesiones compartidas |
| `OPENSEARCH_URL` | vacio | Busqueda OpenSearch; vacio conserva SQL |
| `REINDEX_ON_START` | `true` | `false` en replicas que usan el Job/servicio de indexacion |
| `APP_HOST` | `app.localhost` | Dominio de Caddy |
| `HTTP_PORT`, `HTTPS_PORT` | `80`, `443` | Puertos publicados en Compose |

Los archivos subidos se validan como PNG/JPEG/GIF, hasta 5 MiB y 20 megapixeles. Se guardan con nombre SHA-256 y escritura atomica. Las fichas de libros y autores tienen un formulario para subir/reemplazar la imagen. Los archivos reemplazados se conservan para no romper URLs cacheadas; no hay recolector automatico.

Las sesiones conservan el ultimo libro visitado, duran 24 horas y usan Redis DB 1. La cache anterior usa DB 0, por lo que vaciarla no elimina sesiones. La cookie es HttpOnly, SameSite=Lax y Secure cuando se habilita el proxy. A y B no activan sesiones; C y D si. `/session` permite inspeccionar el estado de la propia sesion, sin exponer su identificador.

## Compose

Ejecutar desde la raiz del repositorio. Usar una variante a la vez con el mismo nombre de proyecto. Antes de cambiar, ejecutar `down` con los archivos de la variante actual, sin `-v`, para conservar los datos.

```sh
# A: app y SQLite, sin proxy
docker compose -p assignment4 -f docker-compose.yml up -d --build

# B: app, SQLite y Caddy
docker compose -p assignment4 -f docker-compose.yml -f compose.edge.yml up -d --build

# C: app, SQLite, Caddy, Redis y OpenSearch
docker compose -p assignment4 -f compose.full.yml up -d --build

# D: tres apps y el stack completo
docker compose -p assignment4 -f compose.full.yml -f compose.scale.yml up -d --build
```

A usa `http://localhost:8080`; B/C/D usan `https://app.localhost`. Solo Caddy publica puertos en B/C/D. El volumen `sqlite-data` contiene la base compartida y `media-data` las imagenes. SQLite WAL requiere procesos en el mismo host; esta implementacion escala instancias en un nodo, no nodos de base de datos. Los escritores siguen serializados por SQLite.

El servicio `index` reconstruye OpenSearch antes de iniciar las apps de C/D. Las replicas no borran ni reconstruyen el indice al arrancar. Para una reconstruccion posterior, detener primero las apps y ejecutar de nuevo el servicio `index`. Los cambios normales de libros/resenas siguen actualizando el indice desde los handlers existentes.

## TLS y archivos estaticos

Caddy emite certificados con su CA local. El navegador necesita confiar explicitamente en esa CA para evitar la advertencia de certificado; no se modifica automaticamente el almacen de confianza del sistema.

```sh
mkdir -p validation-results
docker compose -p assignment4 -f compose.full.yml cp \
  caddy:/data/caddy/pki/authorities/local/root.crt validation-results/compose-root.crt
curl --cacert validation-results/compose-root.crt \
  --resolve app.localhost:443:127.0.0.1 https://app.localhost/healthz
go run ./cmd/verify -ca validation-results/compose-root.crt
sh scripts/verify-failover.sh
```

El verificador escribe una imagen de prueba en el libro y autor con ID 1: usarlo sobre el dataset de pruebas. Comprueba TLS sin omitir su validacion, tres instancias, sesiones, uploads y cabeceras de cache. El script de fallo detiene `app2`, conserva una cookie previa, verifica el servicio con dos instancias y vuelve a iniciarla.

`/static/*` sale de la imagen de Caddy con `Cache-Control: public, max-age=3600`. `/media/*` sale del volumen compartido con `max-age=31536000, immutable`. Esto usa cache HTTP del cliente y servicio directo de archivos; no agrega un cache de respuestas dinamicas a Caddy. Cambiar una imagen produce una URL nueva. En A Go sirve ambos tipos de archivo.

## Kubernetes local

Requiere Docker, kubectl y kind. El script usa exclusivamente el contexto `kind-assignment4`, un cluster local dedicado con un nodo. Puertos: HTTPS `8443`, HTTP `8081`.

```sh
GOBIN="$PWD/bin" go install sigs.k8s.io/kind@v0.33.0
sh scripts/k8s-local.sh
kubectl --context kind-assignment4 -n book-review-app exec deployment/caddy -- \
  cat /data/caddy/pki/authorities/local/root.crt > validation-results/k8s-root.crt
go run ./cmd/verify -url https://app.localhost:8443 -ca validation-results/k8s-root.crt
```

El script aplica primero servicios, PVC y un Job de seed/indexacion; solo despues inicia las tres replicas y Caddy. En redespliegues detiene temporalmente las apps para reconstruir el indice. No aplicar indiscriminadamente `kubectl apply -f k8s/`: `kind.yaml` es configuracion de kind y el Job debe finalizar antes de arrancar las replicas.

Todos los consumidores de SQLite e imagenes se fijan al nodo etiquetado `bookreviews-storage=local`. Los PVC ReadWriteOnce se comparten entre pods de ese nodo. No montar SQLite WAL sobre NFS ni repartir esas replicas entre varios nodos. Redis, OpenSearch y la CA tambien tienen PVC.

El Service de Kubernetes distribuye las conexiones hacia las apps. Caddy usa `keepalive off` en ese despliegue para que sus conexiones a ese Service no se mantengan ligadas a una sola replica. No se usan sesiones adhesivas.

## Carga y resultados brutos

Los casos representan solicitudes totales distribuidas uniformemente durante 300 segundos, no usuarios concurrentes: 1, 10, 100, 1000 y 5000. Se usa k6 `constant-arrival-rate`, una solicitud por iteracion y sin seguir redirecciones. Se comprueban el numero exacto de solicitudes, HTTP 200 y ausencia de iteraciones descartadas.

| Categoria | Endpoint predeterminado |
|---|---|
| Estatico | `/static/style.css`, o `STATIC_PATH=/media/<hash>.png` |
| Agregacion | `/books/top-selling` |
| Busqueda | `/search?q=isla` |
| Lectura dinamica | `/books/1` |

```sh
# Validacion corta del tooling; NO es el benchmark de entrega
DURATION_SECONDS=5 REQUEST_COUNTS=10 sh load-tests/run.sh C
DURATION_SECONDS=5 REQUEST_COUNTS=10 sh load-tests/run.sh D

# Matriz completa: 20 casos x 5 minutos por despliegue
sh load-tests/run.sh C
sh load-tests/run.sh D
```

El runner usa el proyecto aislado `assignment4-load`, puertos 18080/18443 y conserva sus volumenes entre C y D. Antes de cada variante detiene los contenedores de ese proyecto. Cada endpoint se calienta una vez fuera de la medicion. Para comparaciones finales, detener otros stacks/cargas (incluido el cluster kind), mantener el mismo dataset y los mismos recursos de Docker. Las mediciones cortas junto a otras cargas solo validan la instrumentacion.

Resultados en `load-tests/results/<fecha>/<C|D>/<endpoint>/<solicitudes>/`:

- `requests.json`: muestras k6, tiempos y codigos HTTP.
- `summary.json`: resumen estadistico y thresholds.
- `containers.log`: CPU, memoria y PIDs/hilos de Docker por contenedor, con timestamps.
- `processes.log`: PID, PPID, CPU, RSS en KiB, NLWP (hilos) y nombre por proceso.
- `case.txt`, `exit-code.txt`, `k6.log`: parametros y resultado del caso.

El muestreo de recursos usa `docker stats` y `docker top` con pausas de cinco segundos; el intervalo real incluye el tiempo de esas consultas. `%CPU` de `docker top` es el promedio de CPU durante la vida del proceso, mientras Docker informa uso de CPU del contenedor por intervalo. La matriz completa dura al menos 200 minutos, mas arranque y muestreo.

No se generan valores de rendimiento ni conclusiones automaticamente. El informe y la presentacion final corresponden a los estudiantes.
