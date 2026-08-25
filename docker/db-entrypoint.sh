#!/bin/sh
# Entrypoint del servicio "db": inicializa el volumen compartido con "app" y
# termina. No queda un proceso corriendo después: SQLite no tiene servidor,
# así que no hay nada de red que este contenedor deba seguir sirviendo, y
# "app" solo necesita saber que este paso ya terminó bien
# (depends_on: condition: service_completed_successfully en el compose).
set -eu

echo "[db] aplicando esquema en $DB_PATH"
/app/server -migrate

echo "[db] sembrando datos de prueba (no hace nada si ya hay datos)"
if ! /app/seed; then
  echo "[db] seed no corrió: lo más probable es que el volumen ya tuviera datos de una corrida anterior. Si no era eso, revisa el log de arriba."
fi

echo "[db] volumen listo en $DB_PATH."
