#!/bin/bash
# Запуск QA Dashboard без Docker
# Требования: Go 1.23+, Python 3.10+
# Перед запуском: заполните .env файл токенами

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Загружаем переменные из .env
if [ -f "$SCRIPT_DIR/.env" ]; then
    export $(grep -v '^#' "$SCRIPT_DIR/.env" | xargs)
    echo "✅ Токены загружены из .env"
else
    echo "❌ Файл .env не найден! Скопируйте .env.example → .env и заполните токены"
    exit 1
fi

echo ""
echo "🚀 Запуск QA Dashboard..."
echo ""

# 1. Запускаем VPN Flask app в фоне
echo "  → Запуск VPN-сервиса (Python/Flask)..."
cd "$SCRIPT_DIR/vpn-app"
pip3 install -q -r requirements.txt 2>/dev/null
python3 app.py &
VPN_PID=$!
echo "    PID: $VPN_PID"

# 2. Собираем и запускаем Go backend
echo "  → Сборка backend (Go)..."
cd "$SCRIPT_DIR/backend"
go build -o server ./cmd/server
echo "  → Запуск backend..."
./server &
BACKEND_PID=$!
echo "    PID: $BACKEND_PID"

echo ""
echo "════════════════════════════════════════"
echo "  ✅ Dashboard запущен!"
echo "  🌐 Открыть: http://localhost:8080"
echo ""
echo "  Для остановки нажмите Ctrl+C"
echo "════════════════════════════════════════"
echo ""

# Ожидание Ctrl+C
trap "echo ''; echo 'Останавливаем...'; kill $VPN_PID $BACKEND_PID 2>/dev/null; exit 0" INT TERM

wait
