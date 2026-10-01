import 'dotenv/config';
import express from 'express';
import fs from 'node:fs';
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

const rules = JSON.parse(
  fs.readFileSync(new URL('./rules.json', import.meta.url), 'utf8')
);

app.use(express.json({ limit: '2mb' }));
app.use(express.static(path.join(__dirname, 'public')));

app.get('/', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

const SYSTEM = `
Ты эксперт МСК-Сертификат по предварительной классификации товаров.

Работай в два этапа.

Если данных недостаточно:
- не подбирай код окончательно;
- задай короткие уточняющие вопросы;
- выясни назначение, материал, конструкцию, область применения, возрастную категорию, наличие питания и комплектность.

Если данных достаточно:
- предложи наиболее вероятный код ТН ВЭД;
- при необходимости укажи альтернативные варианты;
- проверь возможную необходимость обязательной сертификации или декларирования;
- проверь возможное применение технических регламентов ЕАЭС;
- проверь ПП РФ №2425;
- проверь необходимость СГР и иных разрешительных документов;
- отдельно оцени возможность оформления отказного письма.

Нельзя определять возможность отказного письма только по коду ТН ВЭД.

Если обязательное подтверждение соответствия предварительно не требуется, напиши:
"Возможно оформить отказное письмо".

После этого предложи:
"Отправить заявку эксперту МСК-Сертификат для оформления отказного письма".

Не выдумывай нормативные документы, номера решений, пункты или ГОСТы.

Ответ верни ТОЛЬКО в JSON следующей структуры:

{
  "status": "need_more_info" или "preliminary_result",
  "product_card": {
    "name": "",
    "purpose": "",
    "materials": "",
    "construction": "",
    "power": "",
    "age_group": "",
    "scope": "",
    "completeness": ""
  },
  "questions": [],
  "tnved": [
    {
      "code": "",
      "confidence": "high",
      "reason": ""
    }
  ],
  "regulatory": [
    {
      "area": "",
      "result": "likely_applies",
      "reason": ""
    }
  ],
  "refusal_letter": {
    "result": "likely_possible",
    "reason": ""
  },
  "application_cta": "",
  "client_text": ""
}
`;

function localHints(text) {
  const t = text.toLowerCase();

  return rules.rules
    .filter(r =>
      (r.when?.keywords || []).some(k => t.includes(k))
    )
    .map(r => ({
      id: r.id,
      ask: r.ask,
      comment: r.comment
    }));
}

function parseModelJson(value) {
  if (!value) {
    throw new Error('Пустой ответ модели');
  }

  // Иногда SDK уже может вернуть объект
  if (typeof value === 'object') {
    return value;
  }

  let text = String(value).trim();

  // Убираем markdown-блоки
  text = text
    .replace(/^```json\s*/i, '')
    .replace(/^```\s*/i, '')
    .replace(/\s*```$/i, '')
    .trim();

  // Сначала пробуем обычный JSON
  try {
    return JSON.parse(text);
  } catch {}

  // Если модель добавила текст вокруг JSON — вытаскиваем объект
  const firstBrace = text.indexOf('{');
  const lastBrace = text.lastIndexOf('}');

  if (firstBrace !== -1 && lastBrace > firstBrace) {
    const jsonPart = text.slice(firstBrace, lastBrace + 1);

    try {
      return JSON.parse(jsonPart);
    } catch {}
  }

  throw new Error('Не удалось разобрать JSON модели');
}

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
    const {
      description,
      answers = {}
    } = req.body || {};

    if (!description || String(description).trim().length < 3) {
      return res.status(400).json({
        error: 'Опишите товар подробнее.'
      });
    }

    if (!apiKey || !folderId) {
      return res.status(500).json({
        error: 'Не настроены YANDEX_API_KEY или YANDEX_FOLDER_ID.'
      });
    }

    const hints = localHints(String(description));

    const payload = {
      description: String(description).trim(),
      answers,
      internal_rule_hints: hints,
      rules_version: rules.version
    };

    const response = await client.chat.completions.create({
      model: `gpt://${folderId}/yandexgpt/latest`,

      messages: [
        {
          role: 'system',
          content: SYSTEM
        },
        {
          role: 'user',
          content: JSON.stringify(payload, null, 2)
        }
      ],

      temperature: 0.1,
      max_tokens: 3500
    });

    const message = response.choices?.[0]?.message;

    const rawContent = message?.content ?? '';

    console.log('RAW MODEL RESPONSE:', rawContent);

    let parsed;

    try {
      parsed = parseModelJson(rawContent);
    } catch (parseError) {
      console.error('JSON parse error:', parseError);
      console.error('RAW:', rawContent);

      return res.status(502).json({
        error: 'ИИ вернул ответ в неожиданном формате.',
        raw:
          typeof rawContent === 'string'
            ? rawContent
            : JSON.stringify(rawContent)
      });
    }

    parsed.disclaimer =
      'Результат сформирован автоматически на основании предоставленного описания товара.';

    return res.json(parsed);

  } catch (err) {
    console.error('Analyze error:', err);

    return res.status(500).json({
      error:
        err?.message ||
        'Произошла ошибка при анализе товара.'
    });
  }
});

app.listen(port, () => {
  console.log(`TN VED agent started on port ${port}`);
});

export default app;
