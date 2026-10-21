# Юнит-тесты репозиториев

SQL-репозитории проверяются через sqlmock без PostgreSQL и переменных окружения.
Каждый кейс использует Arrange → Act → Assert. `newSQLMock` создаёт отдельную
БД для кейса и через `t.Cleanup` проверяет все ожидания, даже если тест завершился
досрочно через `Fatal`.

Проверяем точный SQL с нормализацией пробелов (`QueryMatcherEqual`), аргументы,
заполнение моделей, преобразование ошибок и порядок Begin → запросы → Commit
или Rollback. Эталонные запросы зафиксированы в `sql_queries_test.go` и не
читаются из реализации во время теста.

Для массивов `[]string` и `[]int64` используется тестовый `ValueConverter`:
он позволяет сравнить содержимое и порядок аргументов, которые стандартный
конвертер sqlmock не поддерживает. Кодирование массивов драйвером pgx этим
не проверяется.

Redis-репозиторий остаётся на miniredis: sqlmock работает только с database/sql.

Из корня проекта:

```sh
go test ./internal/repository/unit_tests -v
go test ./internal/repository/unit_tests \
  -coverpkg=./internal/repository \
  -coverprofile=/tmp/totallyswipes-repository.cover
go tool cover -func=/tmp/totallyswipes-repository.cover
```

Тесты реальной SQL-фильтрации, ограничений и сохранения истории находятся в
`../integration_tests`. Подставленные строки sqlmock не доказывают, что
PostgreSQL правильно отфильтровал данные или откатил изменения.
