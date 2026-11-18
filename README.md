# Archepelag CLI

Командная строка Архипелага для чтения каталога продуктов через API v1.

## Установка из исходников

Из каталога `cli` выполните команду с Go 1.22 или новее.

```sh
go install .
```

Go установит `archepelag` в `GOBIN` или `GOPATH/bin`. Добавьте этот каталог в `PATH`.

## Подключение

В Архипелаге откройте «Настройки → Токены» (`/ui/settings/tokens`) и создайте персональный API-токен. CLI использует права владельца токена. В CI задайте `ARCHEPELAG_TOKEN` через секреты окружения. Для рабочего экземпляра используйте HTTPS.

```sh
export ARCHEPELAG_SERVER_URL=https://archepelag.example
read -rsp 'API token: ' ARCHEPELAG_TOKEN
printf '\n'
export ARCHEPELAG_TOKEN
```

В PowerShell 7.2 и новее переменные можно задать так.

```powershell
$env:ARCHEPELAG_SERVER_URL = 'https://archepelag.example'
$env:ARCHEPELAG_TOKEN = Read-Host 'API token' -MaskInput
```

## Продукты

```sh
archepelag products list
archepelag products list --search "платежи" --json
archepelag products get example-product --json
```

`list` проходит все страницы каталога. По умолчанию выводит таблицу с ID, ключом и именем, а с `--json` возвращает массив продуктов со всеми полями из API. `get` выводит краткую карточку, а с `--json` возвращает объект продукта целиком.

Ошибки идут в stderr. Код выхода `1` означает ошибку запроса или вывода, `2` означает ошибку аргументов или конфигурации.
