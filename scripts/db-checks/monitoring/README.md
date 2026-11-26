# Мониторинг PostgreSQL

Подробный запуск описан в [основной инструкции](../README.md).

```bash
bash scripts/db-checks/load.sh all STAY_ALIVE
```

Стенд автоматически настраивает datasource Prometheus и дашборд Grafana из файлов:

- `prometheus.yml` — опрос postgres_exporter каждые 5 секунд;
- `grafana/provisioning` — автоматическая настройка Grafana;
- `grafana/dashboards/postgres.json` — шесть панелей базового мониторинга.

Grafana: http://localhost:3000/d/postgres-basics, вход `admin` / `grafana_test_only`.
Prometheus: http://localhost:9090. Exporter: http://localhost:9187/metrics.
Порты 3000, 9090 и 9187 доступны только с этого компьютера.

## Панели

1. Доступность PostgreSQL для exporter.
2. Подключения к тестовой БД.
3. INSERT, UPDATE и DELETE строк в секунду.
4. COMMIT и ROLLBACK в секунду.
5. Чтение блоков и попадания в кеш PostgreSQL в секунду.
6. Обнаруженные deadlock за пять минут.

Изменение строк не равно физической записи на диск. Чтение вне shared_buffers может
обслуживаться кешем ОС. Счётчики включают подготовку данных и запросы мониторинга.
Обычное ожидание блокировки не увеличивает deadlocks. Для rate нужны минимум два измерения.

## Логи

PostgreSQL пишет в stderr подключения, отключения, ошибки, запросы дольше 500 мс
и ожидания блокировок дольше секунды. Префикс содержит время, PID, пользователя,
БД, приложение и адрес клиента.

Снимки логов сохраняются при завершении проверок и сценариев, до удаления контейнеров:
`arifacts/postgres/<проект>/postgres-*.log`. Получить снимок работающего стенда вручную:

```bash
bash scripts/db-checks/collect-logs.sh ИМЯ_ПРОЕКТА
```

Это снимки, а не live-поток. Новые настройки Compose применяются при новом запуске стенда.

`monitoring/load.sh` оставлен как вспомогательная демонстрация INSERT/UPDATE/DELETE
на уже запущенном стенде; полноценные сценарии pgbench запускаются через основной `load.sh`.
