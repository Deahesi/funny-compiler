# Компилятор Funny — HW1/HW2

Лексер и парсер языка Funny на Go. Нужен **Go 1.27.1 или новее** (см. `go.mod`).

Лексер разбивает код на токены, а парсер строит дерево программы — AST.
**Программы Funny пока не исполняются.** Проверки имён, типов, контрактов
и границ массивов тоже пока нет. Успешный разбор не означает, что программа
полностью правильная.

Подробнее:
- [Описание языка](docs/funny.ru.md).
- [Грамматика](docs/grammar.ebnf).
- [Как устроен парсер](docs/parser.md).

## Запуск

Выполняйте команды из папки `compiler/`.

```sh
# Справка
 go run ./cmd/compiler --help

# Разобрать файл и вывести AST в формате JSON
 go run ./cmd/compiler parse examples/example.funny

# Сохранить AST в файл
 go run ./cmd/compiler parse examples/example.funny -o ast.json

# Прочитать код из стандартного ввода
 go run ./cmd/compiler parse - < examples/example.funny
```

Флаг `-o` (или `--out`) задаёт файл для AST. Без него AST выводится в stdout.
Ошибки выводятся в stderr в формате `файл:строка:столбец: сообщение`.

| Результат | AST | Код выхода |
| --- | --- | --- |
| Ошибок нет | Полное дерево | 0 |
| Ошибки синтаксиса | Частичное дерево | 1 |
| Ошибка лексера | Дерево не создаётся | 1 |

При ошибке `go run` также выводит `exit status 1`. Это сообщение Go.
Диагностики не входят в JSON AST; через Go API их можно получить отдельно.

## Примеры

- `examples/example.funny` — простая функция с контрактом.
- `examples/features.funny` — основные возможности языка.
- `examples/arrays.funny` — работа с копией массива.
- `examples/errors.funny` — код с намеренными ошибками.

```sh
go run ./cmd/compiler parse examples/features.funny -o features.ast.json
go run ./cmd/compiler parse examples/arrays.funny -o arrays.ast.json
go run ./cmd/compiler parse examples/errors.funny -o errors.ast.json
```

Последняя команда выводит несколько ошибок, сохраняет частичный AST
и завершается с кодом 1. Ожидаемые результаты:
[текст ошибок](examples/errors.diagnostics.txt),
[ошибки в JSON](examples/errors.diagnostics.json),
[частичный AST](examples/errors.ast.json).

## Автомат лексера (HW1)

```sh
go run ./cmd/compiler dfa -o dfa.json
```

Команда строит и уменьшает автомат лексера, сохраняет таблицу переходов
и показывает число состояний до и после уменьшения.
Без `-o` результат сохраняется в `dfa.json`.
Команда `parse` строит автомат сама и не читает этот файл.

## Сборка и тесты

```sh
go build -o ./bin/compiler ./cmd/compiler
./bin/compiler parse example.funny -o ast.json
./bin/compiler dfa -o dfa.json

go test ./...
go test -race ./...
go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -parallel=1 -fuzztime=3s
```

Тесты проверяют лексер, парсер, AST, сообщения об ошибках и команды.
Последняя команда проверяет парсер на случайных данных в течение трёх секунд.
Это тесты компилятора, а не исполнение программ Funny.

## Папки проекта

```text
cmd/compiler/       — запуск программы
internal/cli/       — команды и работа с файлами
internal/lexer/     — разбор кода на токены
internal/automats/  — автоматы лексера
internal/source/    — позиции в коде и сообщения об ошибках
internal/ast/       — дерево программы
internal/parser/    — синтаксический разбор
```
