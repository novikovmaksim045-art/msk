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

let rules = { version: '1.0', rules: [] };

try {
  rules = JSON.parse(
    fs.readFileSync(new URL('./rules.json', import.meta.url), 'utf8')
  );
} catch (e) {
  console.log('rules.json not loaded');
}

app.use(express.json({ limit: '2mb' }));
app.use(express.static(path.join(__dirname, 'public')));

app.get('/', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

app.get('/api/health', (req, res) => {
  res.json({
    ok: true,
    yandexApiKeyConfigured: Boolean(apiKey),
    yandexFolderIdConfigured: Boolean(folderId)
  });
});

const SYSTEM = `
Ты ИИ-агент компании МСК-Сертификат.

Твоя задача — предварительно определить код ТН ВЭД товара,
проверить необходимость обязательной разрешительной документации
и определить возможность оформления отказного письма.

Работай как эксперт по классификации продукции.

ВАЖНЫЙ ПОРЯДОК РАБОТЫ.

ШАГ 1.

Сначала установи, достаточно ли информации о товаре.

Для классификации могут иметь значение:

- назначение;
- материал;
- состав;
- конструкция;
- принцип работы;
- наличие электрического питания;
- возрастная категория;
- область применения;
- комплектность;
- является ли товар самостоятельным изделием или частью оборудования.

Если данных недостаточно, НЕ ПЫТАЙСЯ угадывать код.

Задай пользователю конкретные уточняющие вопросы.

Максимум 5 вопросов за один раз.

ШАГ 2.

Если информации достаточно:

Предложи наиболее вероятный код ТН ВЭД ЕАЭС.

При наличии реальной неопределенности можно предложить до 3 вариантов.

Для каждого варианта укажи краткую причину классификации.

ШАГ 3.

Проверь возможное применение:

- технических регламентов ЕАЭС;
- обязательной сертификации;
- обязательного декларирования;
- Постановления Правительства РФ № 2425;
- государственной регистрации;
- санитарных требований;
- иных обязательных разрешительных документов.

ШАГ 4.

Отдельно оцени возможность оформления отказного письма.

НЕЛЬЗЯ определять возможность отказного письма только по коду ТН ВЭД.

Необходимо учитывать фактические характеристики и назначение товара.

Если по имеющимся сведениям товар предварительно не подпадает
под обязательное подтверждение соответствия, результат должен быть:

"Возможно оформить отказное письмо"

После этого предложи:

"Отправить заявку эксперту МСК-Сертификат для оформления отказного письма"

Эксперт получает заявку именно НА ОФОРМЛЕНИЕ.
Не пиши, что эксперт должен подтверждать решение ИИ.

Если товар явно требует обязательный разрешительный документ,
не предлагай оформление отказного письма.

Не придумывай нормативные акты, номера пунктов, ГОСТы или решения ЕЭК.

ВЕРНИ ТОЛЬКО JSON.

Структура ответа:

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

Допустимые значения:

status:
- need_more_info
- preliminary_result

confidence:
- high
- medium
- low

regulatory.result:
- likely_applies
- likely_not_applies
- need_check

refusal_letter.result:
- likely_possible
- likely_not_possible
- need_more_info

Никакого текста до JSON.
Никакого текста после JSON.
Не используй markdown.
Не используй блоки \`\`\`json.
`;

function localHints(text) {
  const t = String(text).toLowerCase();

  return (rules.rules || [])
    .filter(r =>
      (r.when?.keywords || []).some(k =>
        t.includes(String(k).toLowerCase())
      )
    )
    .map(r => ({
      id: r.id,
      ask: r.ask,
      comment: r.comment
    }));
}

function parseJsonAnswer(content) {

  if (!content) {
    throw new Error('Пустой ответ YandexGPT');
  }

  if (typeof content === 'object') {
    return content;
  }

  let text = String(content).trim();

  // Убираем markdown на случай, если модель всё-таки его добавила.
  text = text
    .replace(/^```json\s*/i, '')
    .replace(/^```\s*/i, '')
    .replace(/\s*```$/i, '')
    .trim();

  try {
    return JSON.parse(text);
  } catch (e) {
    // Ищем JSON внутри ответа.
  }

  const start = text.indexOf('{');
  const end = text.lastIndexOf('}');

  if (start !== -1 && end > start) {
    const possibleJson = text.slice(start, end + 1);

    return JSON.parse(possibleJson);
  }

  throw new Error('Ответ модели не содержит JSON');
}

app.post('/api/analyze', async (req, res) => {

  try {

    const {
      description,
      answers = {}
    } = req.body || {};

    const productDescription =
      String(description || '').trim();

    if (productDescription.length < 2) {
      return res.status(400).json({
        error: 'Опишите товар.'
      });
    }

    if (!apiKey || !folderId) {
      return res.status(500).json({
        error:
          'Не настроены YANDEX_API_KEY или YANDEX_FOLDER_ID.'
      });
    }

    const hints = localHints(productDescription);

    const userData = {
      product: productDescription,
      previous_answers: answers,
      internal_hints: hints
    };

    const response =
      await client.chat.completions.create({

        model:
          `gpt://${folderId}/yandexgpt/latest`,

        messages: [
          {
            role: 'system',
            content: SYSTEM
          },
          {
            role: 'user',
            content:
              JSON.stringify(userData, null, 2)
          }
        ],

        temperature: 0.05,
        max_tokens: 3000
      });

    const content =
      response.choices?.[0]?.message?.content;

    console.log(
      'YANDEX RAW:',
      content
    );

    let result;

    try {

      result = parseJsonAnswer(content);

    } catch (parseError) {

      console.error(
        'PARSE ERROR:',
        parseError
      );

      console.error(
        'RAW CONTENT:',
        content
      );

      return res.status(502).json({
        error:
          'Не удалось обработать ответ ИИ.',
        raw: content
      });
    }

    // Защита от неполного ответа модели.

    if (!result.status) {

      return res.status(502).json({
        error:
          'ИИ вернул неполный результат.',
        raw: result
      });
    }

    if (!Array.isArray(result.questions)) {
      result.questions = [];
    }

    if (!Array.isArray(result.tnved)) {
      result.tnved = [];
    }

    if (!Array.isArray(result.regulatory)) {
      result.regulatory = [];
    }

    if (!result.product_card) {
      result.product_card = {};
    }

    if (!result.refusal_letter) {

      result.refusal_letter = {
        result: 'need_more_info',
        reason:
          'Недостаточно информации для оценки.'
      };
    }

    // Если отказное возможно,
    // принудительно добавляем правильный CTA.

    if (
      result.refusal_letter.result ===
      'likely_possible'
    ) {

      result.application_cta =
        'Отправить заявку эксперту МСК-Сертификат для оформления отказного письма';
    }

    result.disclaimer =
      'Результат сформирован автоматически на основании предоставленной информации о товаре.';

    return res.json(result);

  } catch (err) {

    console.error(
      'ANALYZE ERROR:',
      err
    );

    return res.status(500).json({
      error:
        err?.message ||
        'Произошла ошибка при анализе товара.'
    });
  }
});

app.listen(port, () => {
  console.log(
    `MSK TN VED Agent started on port ${port}`
  );
});

export default app;
