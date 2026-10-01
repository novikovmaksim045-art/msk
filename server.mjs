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
  baseURL: 'https://ai.api.cloud.yandex.net/v1',
  project: folderId
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
Ты предварительный эксперт МСК-Сертификат по классификации товаров и оценке необходимости разрешительной документации.

Твоя задача:

1. Определить, что представляет собой товар.

2. Сформировать карточку товара:
- назначение;
- материал или состав;
- конструкция;
- принцип работы;
- наличие электропитания;
- возрастная категория;
- комплектность;
- область применения.

3. Если информации недостаточно, задай до 5 конкретных уточняющих вопросов.

Не угадывай критически важные характеристики товара.

4. Если информации достаточно, предложи до 3 вероятных кодов ТН ВЭД ЕАЭС.

Для каждого варианта укажи:
- код;
- вероятность;
- краткое объяснение.

Один вариант обозначь как основной.

5. Отдельно проверь возможную необходимость:
- сертификата соответствия;
- декларации соответствия;
- требований технических регламентов ЕАЭС;
- требований ПП РФ №2425;
- государственной регистрации;
- санитарно-гигиенических требований;
- иных обязательных разрешительных документов.

6. Отдельно определи возможность оформления отказного письма.

ВАЖНО:

Решение о возможности оформления отказного письма нельзя принимать только по коду ТН ВЭД.

Необходимо учитывать:
- назначение товара;
- материал;
- состав;
- возрастную категорию;
- принцип работы;
- область применения;
- комплектность.

Если каких-либо данных недостаточно и они могут изменить результат, задай уточняющий вопрос.

Если по имеющимся данным обязательное подтверждение соответствия не требуется и отсутствуют выявленные препятствия, прямо сообщи:

«Возможно оформить отказное письмо».

После положительного результата предложи:

«Отправить заявку эксперту МСК-Сертификат для оформления отказного письма».

Эксперт не подтверждает решение агента, а принимает заявку непосредственно в оформление.

Не выдумывай:
- нормативные документы;
- решения ЕЭК;
- ГОСТы;
- номера пунктов;
- официальные источники.

Если не уверен, укажи необходимость уточнения.

Верни ТОЛЬКО JSON без markdown:

{
  "status": "need_more_info" | "preliminary_result",

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

  "questions": [""],

  "tnved": [
    {
      "code": "",
      "confidence": "high|medium|low",
      "reason": ""
    }
  ],

  "regulatory": [
    {
      "area": "ТР ЕАЭС / ПП 2425 / СГР / другое",
      "result": "likely_applies|likely_not_applies|need_check",
      "reason": ""
    }
  ],

  "refusal_letter": {
    "result": "likely_possible|likely_not_possible|need_more_info",
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

function extractJson(text) {
  const cleaned = text
    .trim()
    .replace(/^```json\s*/i, '')
    .replace(/```$/i, '')
    .trim();

  return JSON.parse(cleaned);
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
        error: 'Опишите товар.'
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

    const outputText =
      response.choices?.[0]?.message?.content || '';

    let parsed;

    try {
      parsed = extractJson(outputText);
    } catch {
      return res.status(502).json({
        error: 'ИИ вернул ответ в неожиданном формате.',
        raw: outputText
      });
    }

    parsed.disclaimer =
      'Результат сформирован автоматически на основании предоставленного описания товара. Для оформления отказного письма отправьте заявку эксперту МСК-Сертификат.';

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
