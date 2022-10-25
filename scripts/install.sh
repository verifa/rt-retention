#!/usr/bin/env bash

echo "[RT-RETENTION] Installing..."
mkdir -p ~/.jfrog/plugins/rt-retention/bin
cp build/rt-retention ~/.jfrog/plugins/rt-retention/bin
echo "[RT-RETENTION] Done."
