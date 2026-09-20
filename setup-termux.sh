#!/data/data/com.termux/files/usr/bin/bash
# IT Operations Hub — Termux (native Android) setup
#
# Run this from inside Termux, from the directory this script lives in
# (the unzipped project root, containing backend/ and frontend/).
#
# This has NOT been run on a physical device by the author — it's a
# best-effort automation of the manual steps, written from a correct
# understanding of Termux's package manager and Go/Node toolchains, but
# unverified end-to-end on real hardware. Read the output as it runs; if
# a step fails, the error message will tell you which one and why.

set -e

echo "============================================"
echo "  IT Operations Hub — Termux setup"
echo "============================================"
echo

echo "[1/6] Installing packages (go, nodejs, sqlite, git, clang)..."
pkg update -y
pkg install -y golang nodejs sqlite git clang

echo
echo "[2/6] Requesting storage permission (optional — only needed if you"
echo "      want to access the app's data files from other Android apps,"
echo "      e.g. to copy a backup out via a file manager. The app itself"
echo "      works fine without this, using Termux's own home directory)..."
termux-setup-storage || echo "  (skipped — fine to skip if you don't need this)"

echo
echo "[3/6] Building backend natively for this device's architecture..."
cd backend
export CGO_ENABLED=1
go build -mod=vendor -o ../dist/ticketing-backend-android ./cmd/server
cd ..

echo
echo "[4/6] Installing frontend dependencies..."
cd frontend
npm install

echo
echo "[5/6] Building frontend..."
npm run build
cd ..

echo
echo "[6/6] Done."
echo
echo "To run it:"
echo "  1. In one Termux session:  ./dist/ticketing-backend-android"
echo "  2. In another Termux session (swipe from left edge, 'New session'):"
echo "       cd frontend && npm run start -- -p 3000"
echo "  3. Open http://localhost:3000 in your phone's own browser."
echo
echo "IMPORTANT — Android will kill Termux in the background to save"
echo "battery unless you stop it. In the Termux notification (pull down"
echo "the notification shade), tap 'Acquire wakelock', and disable"
echo "battery optimization for Termux in Android Settings > Apps >"
echo "Termux > Battery > Unrestricted. Without this, the server will"
echo "stop when you switch apps or lock the screen."
