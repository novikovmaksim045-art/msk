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

app.post('/api/analyze', async (req, res) => {
  try {
    const { description } = req.body || {};

    if (!description || String(description).trim().length < 2) {
      return res.status(400).json({
        error: 'Опишите товар.'
      });
    }

    if (!apiKey || !folderId) {
      return res.status(500).json({
        error: 'Не настроены YANDEX_API_KEY или YANDEX_FOLDER_ID.'
      });
    }

    const response = await client.chat.completions.create({
      model: `gpt://${folderId}/yandexgpt/latest`,

      messages: [
        {
          role: 'system',
          content: `
Ты эксперт по классификации товаров.

Ответь на русском языке.

Сейчас режим диагностики.

Для товара пользователя:
1. Кратко определи, что это за товар.
2. Укажи, каких данных не хватает для точного подбора кода ТН ВЭД.
3. Задай уточняющие вопросы.

Не обязательно возвращать JSON.
`
        },
        {
          role: 'user',
          content: String(description).trim()
        }
      ],

      temperature: 0.1,
      max_tokens: 1200
    });

    console.log('FULL YANDEX RESPONSE:');
    console.log(JSON.stringify(response, null, 2));

    return res.json({
      debug: true,
      folderId,
      choices: response.choices || null,
      full_response: response
    });

  } catch (err) {
    console.error('YANDEX ERROR:');
    console.error(err);

    return res.status(500).json({
      error: err?.message || String(err),
      name: err?.name || null,
      status: err?.status || null,
      code: err?.code || null
    });
  }
});

app.listen(port, () => {
  console.log(`TN VED debug agent started on port ${port}`);
});

export default app;
