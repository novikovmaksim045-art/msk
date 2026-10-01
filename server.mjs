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

Твоя задача:

1. Определить товар.
2. Если информации недостаточно — задать уточняющие вопросы.
3. Если информации достаточно — предложить вероятные коды ТН ВЭД.
4. Проверить возможную необходимость:
- сертификата;
- декларации;
- технических регламентов ЕАЭС;
- ПП РФ №2425;
- СГР;
- иных разрешительных документов.
5. Определить возможность оформления отказного письма.

Нельзя определять возможность отказного письма только по коду ТН ВЭД.

Учитывай:
- назначение;
- материал;
- состав;
- возрастную категорию;
- конструкцию;
- принцип работы;
- область применения;
- комплектность.

Если обязательное подтверждение соответствия предварительно не требуется,
укажи:

"Возможно оформить отказное письмо"

и предложи:

"Отправить заявку эксперту МСК-Сертификат для оформления отказного письма".

Если информации недостаточно — задавай конкретные вопросы.

Не выдумывай нормативные документы и номера пунктов.
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

      response_format: {
        type: 'json_schema',
        json_schema: {
          name: 'tnved_analysis',
          strict: true,
          schema: {
            type: 'object',
            additionalProperties: false,
            properties: {
              status: {
                type: 'string',
                enum: ['need_more_info', 'preliminary_result']
              },

              product_card: {
                type: 'object',
                additionalProperties: false,
                properties: {
                  name: { type: 'string' },
                  purpose: { type: 'string' },
                  materials: { type: 'string' },
                  construction: { type: 'string' },
                  power: { type: 'string' },
                  age_group: { type: 'string' },
                  scope: { type: 'string' },
                  completeness: { type: 'string' }
                },
                required: [
                  'name',
                  'purpose',
                  'materials',
                  'construction',
                  'power',
                  'age_group',
                  'scope',
                  'completeness'
                ]
              },

              questions: {
                type: 'array',
                items: { type: 'string' }
              },

              tnved: {
                type: 'array',
                items: {
                  type: 'object',
                  additionalProperties: false,
                  properties: {
                    code: { type: 'string' },
                    confidence: {
                      type: 'string',
                      enum: ['high', 'medium', 'low']
                    },
                    reason: { type: 'string' }
                  },
                  required: ['code', 'confidence', 'reason']
                }
              },

              regulatory: {
                type: 'array',
                items: {
                  type: 'object',
                  additionalProperties: false,
                  properties: {
                    area: { type: 'string' },
                    result: {
                      type: 'string',
                      enum: [
                        'likely_applies',
                        'likely_not_applies',
                        'need_check'
                      ]
                    },
                    reason: { type: 'string' }
                  },
                  required: ['area', 'result', 'reason']
                }
              },

              refusal_letter: {
                type: 'object',
                additionalProperties: false,
                properties: {
                  result: {
                    type: 'string',
                    enum: [
                      'likely_possible',
                      'likely_not_possible',
                      'need_more_info'
                    ]
                  },
                  reason: { type: 'string' }
                },
                required: ['result', 'reason']
              },

              application_cta: { type: 'string' },
              client_text: { type: 'string' }
            },

            required: [
              'status',
              'product_card',
              'questions',
              'tnved',
              'regulatory',
              'refusal_letter',
              'application_cta',
              'client_text'
            ]
          }
        }
      },

      temperature: 0.1,
      max_tokens: 3500
    });

    const outputText =
      response.choices?.[0]?.message?.content || '';

    let parsed;

    try {
      parsed = JSON.parse(outputText);
    } catch {
      return res.status(502).json({
        error: 'ИИ вернул ответ в неожиданном формате.',
        raw: outputText
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
