# МСК-Сертификат — агент ТН ВЭД / отказное письмо (YandexGPT)

Версия для размещения на Vercel и работы через Yandex Cloud AI Studio / YandexGPT.

## Что делает агент

1. Принимает описание товара.
2. При нехватке данных задаёт уточняющие вопросы.
3. Предлагает вероятные коды ТН ВЭД ЕАЭС.
4. Предварительно проверяет применимость ТР ЕАЭС, ПП РФ 2425, СГР и иных обязательных требований.
5. Если препятствий не выявлено, сообщает: «Возможно оформить отказное письмо».
6. Предлагает отправить заявку эксперту МСК-Сертификат на оформление.

## Что нужно для Vercel

В проекте Vercel добавьте две Environment Variables:

- `YANDEX_API_KEY` — API-ключ сервисного аккаунта Yandex Cloud.
- `YANDEX_FOLDER_ID` — ID папки Yandex Cloud, в которой доступен AI Studio.

После добавления переменных сделайте Redeploy проекта.

## Как получить данные Yandex Cloud

1. Войдите в Yandex Cloud: https://console.yandex.cloud/
2. Создайте облако/папку либо выберите существующую.
3. Скопируйте ID папки — это значение для `YANDEX_FOLDER_ID`.
4. Создайте сервисный аккаунт для агента.
5. Выдайте ему роль, позволяющую обращаться к AI Studio / Foundation Models.
6. Создайте API-ключ сервисного аккаунта. Для минимальных прав используйте scope `yc.ai.languageModels.execute` либо подходящий scope AI Studio.
7. Секретную часть ключа скопируйте в `YANDEX_API_KEY`.

Ключ нельзя помещать в `public/index.html` или другой клиентский код.

## Локальный запуск

```bash
npm install
cp .env.example .env
# заполните YANDEX_API_KEY и YANDEX_FOLDER_ID
npm start
```

После запуска откройте http://localhost:3000

## Vercel

Проект использует Express. После загрузки папки в Vercel:

1. Deploy.
2. Settings -> Environment Variables.
3. Добавьте `YANDEX_API_KEY` и `YANDEX_FOLDER_ID`.
4. Deployments -> последний deployment -> Redeploy.

## Модель

По умолчанию используется URI:

`gpt://<YANDEX_FOLDER_ID>/yandexgpt/latest`

Сервер обращается к OpenAI-совместимому API Yandex Cloud по адресу `https://ai.api.cloud.yandex.net/v1`.
