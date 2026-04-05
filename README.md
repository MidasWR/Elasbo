# Elsabo

TUI-утилита для **массовой замены одного PHP entrypoint** (например `index.php` или `cl.php`) в корнях сайтов **FastPanel**. Список сайтов берётся по SSH с командой `mogwai sites list`. После заливки выполняется **HTTPS-проверка** (эвристики против «заглушек» nginx/apache и слишком короткого ответа); при провале — **откат** из бэкапа.

Репозиторий: [github.com/MidasWR/Elasbo](https://github.com/MidasWR/Elasbo)

---

## Быстрая установка (одна команда)

Нужны `curl` и tarball под вашу ОС (Linux или macOS; `amd64` / `arm64`). **Токен GitHub не требуется** — используется публичный API релизов.

Установка бинарника в `~/.local/bin` (по умолчанию):

```bash
curl -fsSL https://raw.githubusercontent.com/MidasWR/Elasbo/main/scripts/install.sh | bash
```

Установка и сразу запуск интерфейса:

```bash
curl -fsSL https://raw.githubusercontent.com/MidasWR/Elasbo/main/scripts/install.sh | bash -s -- --run
```

Системная установка (нужны права на каталог):

```bash
curl -fsSL https://raw.githubusercontent.com/MidasWR/Elasbo/main/scripts/install.sh | sudo PREFIX=/usr/local/bin bash
```

Если релиза ещё нет или скачивание не удалось, скрипт попробует **`go install github.com/MidasWR/Elasbo/cmd/elsabo@latest`** (нужен [Go](https://go.dev/dl/) 1.25+).

Убедитесь, что каталог установки в `PATH`, например:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Запуск:

```bash
elsabo
# или явный конфиг:
elsabo -config ~/.config/elsabo/config.yaml
```

---

## Эксплуатация: что делает программа

1. Подключается по **SSH** к одному или нескольким хостам панели (см. настройки и `ssh_targets`).
2. На каждом хосте выполняет **`mogwai sites list`**, парсит ID, `ServerName`, `DOCUMENT_ROOT`, параметры SSH/SFTP для сайта.
3. Вы в TUI отмечаете сайты и активную **клоаку** (шаблон PHP из локального vault).
4. По **Enter** на вкладке Run для каждого сайта: **бэкап текущего файла → SFTP-загрузка нового → HTTPS verify → при ошибке откат**.

---

## Требования

| Компонент | Описание |
|-----------|----------|
| Локально | Бинарник из релиза **или** Go 1.25+ для сборки / `go install`. |
| Панель | SSH-доступ к хосту(ам) FastPanel, выполнение `mogwai sites list`. |
| SFTP | Запись в `DOCUMENT_ROOT` каждого сайта (часто отдельные пользователи на сайт — настройте ключи или окружение панели). |
| Сеть | Исходящий HTTPS с вашей машины к доменам сайтов (проверка после замены). |

**Аутентификация SSH:** агент (`SSH_AUTH_SOCK`), ключи в настройках или стандартные `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`; в UI и YAML можно задать **пароль** (глобально и/или на строку в bulk-редакторе целей — см. ниже).

---

## Конфигурация

Путь по умолчанию: **`~/.config/elsabo/config.yaml`**. Переопределение: флаг **`-config /path/to/config.yaml`**. При первом запуске, если файла нет, создаётся шаблон.

Пример структуры (значения по вкусу):

```yaml
ssh_host: panel.example.com
ssh_user: root
ssh_port: 22
# Пароль в файле — только если допустимо по политике; иначе переменная окружения:
# ssh_password: ""
ssh_strict_host: false
ssh_identity_file: ""
ssh_known_hosts: ""
mogwai_cmd: mogwai sites list
default_entry: index.php
vault_dir: /home/you/.config/elsabo/vault

# Несколько панелей / серверов (опционально). Пусто — используются ssh_host + ssh_user.
ssh_targets:
  - host: 10.0.0.1
    user: root
    port: 22
    password: ""
  - host: other.panel
    user: deploy

# Теги доменов для фильтра во вкладке Domains (клавиша t — быстрый фильтр FR)
domain_tags:
  example.com: [FR, test]

verify:
  timeout: 25s
  user_agent: "Mozilla/5.0 (compatible; Elsabo/1.0)"
  min_body_bytes: 80
  max_redirects: 5
  try_www: true
  extra_stub_snippets: []
```

**Переменная окружения `ELSABO_SSH_PASSWORD`:** если задана непустая, подменяет `ssh_password` из файла (удобно для общих машин).

**`ssh_strict_host: true`** — строгая проверка known_hosts (нужен корректный `ssh_known_hosts`).

---

## Интерфейс (вкладки и клавиши)

Общее:

| Клавиша | Действие |
|---------|----------|
| **Tab** / **1–4** | Переключение вкладок: Domains, Cloaks, Settings, Run |
| **←** / **Shift+Tab** | Вкладка влево |
| **→** | Вкладка вправо |
| **Esc** / **Ctrl+C** | Выход (во время выполнения задания Esc не прерывает прогон — дождитесь окончания) |

### Domains (1)

| Клавиша | Действие |
|---------|----------|
| **r** | Обновить список сайтов с панели |
| **↑** **↓** / PgUp/PgDn | Курсор |
| **Пробел** | Выбрать/снять сайт |
| **a** | Выбрать все |
| **A** | Инвертировать выбор |
| **t** | Переключить фильтр по тегу (пусто ↔ пример `FR`) |
| **g** | Редактировать теги для домена под курсором (через запятую, Enter — сохранить в конфиг) |
| **i** | Имя entrypoint для этой сессии (не только `default_entry`) |
| **n** | Перейти на вкладку Run |

### Cloaks (2)

Vault хранит копии PHP-файлов с метками; одна запись помечается активной для задания.

| Клавиша | Действие |
|---------|----------|
| **↑** **↓** | Курсор |
| **Enter** / **Пробел** | Сделать выбранную клоаку активной |
| **n** | Новая клоака: метка → большой редактор кода (**Ctrl+S** сохранить) |
| **f** | Добавить из файла: путь ( **f** в поле пути — выбор файла), затем метка, Enter |
| **t** | Пустой PHP по метке (шаблон) |
| **d** | Удалить запись под курсором |
| **b** | Bulk-редактор: список UUID — **Ctrl+S** оставляет только перечисленные, остальное удаляется |
| **Esc** | Выйти из пошагового добавления / редакторов |

### Settings (3)

Полям соответствуют поля YAML; стрелки **↑↓** переключают фокус между полями; **h u w p i k e m** — быстрый прыжок к полям (host, user, password, port, identity, known_hosts, default entry, mogwai).

| Клавиша | Действие |
|---------|----------|
| **Ctrl+S** | Сохранить конфиг |
| **b** | Bulk SSH targets: строки `user@host` или `user@host:port`, опционально **Tab и пароль** на строку; **Ctrl+S** — сохранить и обновить список сайтов |
| **y** / **o** | Выбор файла для identity / known_hosts (browser) |
| **Esc** | Снять фокус с поля |

### Run (4)

| Клавиша | Действие |
|---------|----------|
| **Enter** | Запустить замену по всем выбранным сайтам с активной клоакой |

В логе по каждому сайту видно успех/ошибка (в т.ч. откат при провале verify).

---

## HTTP-проверка после замены

Запросы к `https://{ServerName}/` и при `verify.try_www: true` к `https://www.{ServerName}/`. Отказ, если:

- не 2xx (включая явную обработку **429**);
- тело короче `min_body_bytes`;
- в теле есть известные сигнатуры заглушек или ваши строки в `verify.extra_stub_snippets`.

Настройка — секция **`verify:`** в YAML.

---

## Сборка из исходников

```bash
git clone https://github.com/MidasWR/Elasbo.git
cd elsabo
make build
./dist/elsabo
```

Тесты:

```bash
make test
```

Снимок артефактов (как у GoReleaser, без публикации):

```bash
make snapshot   # dist/
```

Локально **`make release`** без `GITHUB_TOKEN` делает тот же **snapshot** в `dist/`. Для публикации GitHub Release GoReleaser ожидает **семвер-тег на текущем коммите** (`HEAD`), например `v0.1.0`:

```bash
git tag v0.1.0
git push origin v0.1.0
gh auth login    # один раз; токен хранит gh, отдельный GITHUB_TOKEN не обязателен
make release-upload
```

Если тега на `HEAD` нет, `make release-upload` завершится с подсказкой (цель `ensure-release-tag` в Makefile).

Перед публикацией GitHub Release **не должно быть незакоммиченных изменений** (`git status` пустой): иначе GoReleaser завершится с `git is in a dirty state`. Сначала `git add -A && git commit …`, при необходимости **новый тег** на этом коммите (`v0.1.1`), затем `make release-upload`. Локально это проверяет цель `ensure-clean-git`.

**Если `403 Resource not accessible by personal access token` при `release-upload`:** токен не имеет прав на создание релизов в `MidasWR/Elasbo`.

- **Classic PAT:** включите scope **`repo`** (приватный репозиторий) или минимум **`public_repo`**, если репозиторий публичный; убедитесь, что аккаунт токена может пушить в репозиторий.
- **Fine-grained PAT:** выберите репозиторий **Elasbo**, разрешение **Contents → Read and write** (релизы и вложения идут через Contents API).
- Организация с **SAML SSO:** в настройках токена нажмите **Configure SSO** / авторизуйте для организации **MidasWR**.
- Проще: **`gh auth login`** с доступом к репозиторию и затем `make release-upload` (Makefile подставит `gh auth token`).

CI: push тега `v1.2.3` запускает [`.github/workflows/release.yml`](.github/workflows/release.yml) с `GITHUB_TOKEN` репозитория.

---

## Структура проекта

```
cmd/elsabo/          # точка входа
internal/config/     # YAML, теги доменов, ssh_targets, bulk-парсер
internal/sshutil/    # SSH + SFTP
internal/fastpanel/  # парсер mogwai sites list
internal/cloaks/     # vault + manifest
internal/verify/     # HTTP-проверки
internal/replace/    # бэкап, заливка, откат
internal/tui/        # разработка на Bubble Tea
reports/             # заметки по итерациям
scripts/install.sh   # curl-инсталлятор для релизов
```

---

## Ответственное использование

Вы обязаны соблюдать законы, правила хостинга и рекламных сетей. Репозиторий автоматизирует замену файлов и проверку HTTP; дальнейшее использование — ваша ответственность.
