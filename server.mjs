import 'dotenv/config';
import express from 'express';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import OpenAI from 'openai';

const app = express();

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const port = process.env.PORT || 3000;

const folderId = (process.env.YANDEX_FOLDER_ID || '').trim();
const apiKey = (process.env.YANDEX_API_KEY || '').trim();

const client = new OpenAI({
  apiKey,
  baseURL: 'https://ai.api.cloud.yandex.net/v1'
});

app.use(express.json({ limit: '2mb' }));
app.use(express.static(path.join(__dirname, 'public')));

app.get('/', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

app.get('/api/health', (req, res) => {
  res.json({
    ok: true,
    yandexApiKeyConfigured: Boolean(apiKey),
    yandexFolderIdConfigured: Boolean(folderId),
    folderIdLength: folderId.length,
    folderIdLast4: folderId.slice(-4)
  });
});

// Диагностика YandexGPT прямо через браузер
app.get('/api/debug-yandex', async (req, res) => {
  try {
    const description =
      String(req.query.description || 'шланг').trim();

    const response = await client.chat.completions.create({
      model: `gpt://${folderId}/yandexgpt/latest`,

      messages: [
        {
          role: 'system',
          content: `
Ты эксперт по классификации товаров по ТН ВЭД.

Ответь на русском языке.

Пока не нужно возвращать JSON.

Для товара пользователя:
1. Скажи, что это за товар.
2. Определи, достаточно ли информации для классификации.
3. Если информации недостаточно — задай уточняющие вопросы.
`
        },
        {
          role: 'user',
          content: description
        }
      ],

      temperature: 0.1,
      max_tokens: 1200
    });

    return res.json({
      ok: true,
      folderId,
      response
    });

  } catch (err) {
    console.error(err);

    return res.status(500).json({
      ok: false,
      error: err?.message || String(err),
      status: err?.status || null,
      code: err?.code || null
    });
  }
});

// Старый интерфейс пока тоже оставляем рабочим
app.post('/api/analyze', async (req, res) => {
  try {
    const description =
      String(req.body?.description || '').trim();

    if (!description) {
      return res.status(400).json({
        error: 'Опишите товар.'
      });
    }

    const response = await client.chat.completions.create({
      model: `gpt://${folderId}/yandexgpt/latest`,

      messages: [
        {
          role: 'system',
          content: `
Ты эксперт по классификации товаров по ТН ВЭД.

Ответь на русском языке.

Для товара пользователя:
1. Определи товар.
2. Укажи, достаточно ли данных.
3. Если данных недостаточно, задай уточняющие вопросы.
`
        },
        {
          role: 'user',
          content: description
        }
      ],

      temperature: 0.1,
      max_tokens: 1200
    });

    return res.json({
      debug: true,
      response
    });

  } catch (err) {
    return res.status(500).json({
      error: err?.message || String(err)
    });
  }
});

app.listen(port, () => {
  console.log(`TN VED agent started on port ${port}`);
});

export default app;
