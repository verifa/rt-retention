#!/usr/bin/env bash

echo "[RT-RETENTION] Cleaning..."
rm -rf build/
echo "[RT-RETENTION] Building..."
go build -o build/rt-retention
echo "[RT-RETENTION] Testing..."
build/rt-retention --help
echo "[RT-RETENTION] Done."
