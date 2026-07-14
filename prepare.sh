#!/bin/bash
# Подготовка local-build: копирует backend, vpn-app, frontend из основного проекта
# Frontend создаётся БЕЗ AI Анализа

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "📦 Подготовка local-build..."

# Backend
echo "  → Копируем backend/"
rm -rf "$SCRIPT_DIR/backend"
cp -r "$PROJECT_DIR/backend" "$SCRIPT_DIR/backend"

# VPN App
echo "  → Копируем vpn-app/"
rm -rf "$SCRIPT_DIR/vpn-app"
cp -r "$PROJECT_DIR/vpn-app" "$SCRIPT_DIR/vpn-app"

# Frontend
echo "  → Копируем frontend/ (без AI Анализа)"
rm -rf "$SCRIPT_DIR/frontend"
cp -r "$PROJECT_DIR/frontend" "$SCRIPT_DIR/frontend"

# Удаляем пункт навигации AI Анализ из HTML
python3 -c "
import re
with open('$SCRIPT_DIR/frontend/index.html', 'r') as f:
    html = f.read()

# Удаляем nav-item для ai-analysis
html = re.sub(r'\s*<a href=\"#\" class=\"nav-item\" data-page=\"ai-analysis\".*?</a>', '', html, flags=re.DOTALL)

# Удаляем page-ai-analysis блок
html = re.sub(r'\s*<!-- Page: AI Analysis -->.*?</div>\s*</div>', '', html, flags=re.DOTALL)
# Fallback: удаляем по id
html = re.sub(r'\s*<div class=\"page hidden\" id=\"page-ai-analysis\">.*?</div>\s*</main>', '\n        </main>', html, flags=re.DOTALL)

with open('$SCRIPT_DIR/frontend/index.html', 'w') as f:
    f.write(html)
"

# Удаляем AI Analysis функции из app.js
python3 -c "
with open('$SCRIPT_DIR/frontend/js/app.js', 'r') as f:
    js = f.read()

# Убираем строку навигации на ai-analysis
js = js.replace(\"if (page === 'ai-analysis') loadAIAnalysis();\", '')

# Убираем функции loadAIAnalysis и runAIAnalysis и formatAIResponse
import re
js = re.sub(r'// === AI ANALYSIS ===.*', '', js, flags=re.DOTALL)

with open('$SCRIPT_DIR/frontend/js/app.js', 'w') as f:
    f.write(js)
"

echo ""
echo "✅ Готово! Структура local-build подготовлена."
echo ""
echo "Дальнейшие шаги:"
echo "   1. cp .env.example .env"
echo "   2. Заполните токены в .env"
echo "   3. docker-compose up --build -d"
echo "   4. Откройте http://localhost:8080"
