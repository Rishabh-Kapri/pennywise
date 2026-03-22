#!/bin/sh
# Seed the persistent volume with .parms files from the Docker image on first run.
# After retraining, the volume has the newer models and this script won't overwrite them.

VOLUME_DIR="/data"
APP_DIR="/app"

for parms_file in "$APP_DIR"/*.parms; do
    [ -f "$parms_file" ] || continue
    filename=$(basename "$parms_file")
    if [ ! -f "$VOLUME_DIR/$filename" ]; then
        echo "[entrypoint] Seeding $filename to volume"
        cp "$parms_file" "$VOLUME_DIR/$filename"
    else
        echo "[entrypoint] $filename already exists on volume, skipping"
    fi
done

# Ensure backups and data dirs exist on the volume
mkdir -p "$VOLUME_DIR/backups"
mkdir -p "$VOLUME_DIR/data"

exec python mlp_predict_server.py
