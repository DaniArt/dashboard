# QA Dashboard — Локальный запуск

## Что нужно

1. Docker + Docker Compose
2. Токены доступа (Jira, Confluence, GitLab)

## Быстрый старт

1. Скопируйте `.env.example` в `.env` и заполните токены:

```bash
cp .env.example .env
# Отредактируйте .env — впишите свои токены
```

2. Запустите:

```bash
docker-compose up --build -d
```

3. Откройте в браузере: http://localhost:8080

## Остановка

```bash
docker-compose down
```

## Примечания

- Данные обновляются каждые 5 минут из Jira, GitLab, Confluence
- VPN-посещаемость: загрузите xlsx через интерфейс (раздел "Посещаемость")
- AI Анализ отключён в этой сборке
- Для изменения списка сотрудников отредактируйте `backend/config.yaml`
