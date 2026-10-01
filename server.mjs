import 'dotenv/config';
import express from 'express';
import fs from 'node:fs';
import OpenAI from 'openai';

const app = express();
const port = process.env.PORT || 3000;
const folderId = process.env.YANDEX_FOLDER_ID;
const apiKey = process.env.YANDEX_API_KEY;
const client = new OpenAI({
  apiKey,
  baseURL: 'https://ai.api.cloud.yandex.net/v1'
});
const rules = JSON.parse(fs.readFileSync(new URL('./rules.json', import.meta.url), 'utf8'));

app.use(express.json({ limit: '2mb' }));
app.use(express.static('public'));

const SYSTEM = `
Ты предварительный эксперт МСК-Сертификат по классификации товаров и оценке необходимости разрешительной документации.

Задача:
1) Сформировать карточку товара: назначение, материал/состав, конструкция, принцип работы, питание, возрастная группа, комплектность, сфера применения.
2) Если данных недостаточно — задать до 5 точных уточняющих вопросов. Не угадывать.
3) Если данных достаточно — предложить до 3 вероятных кодов ТН ВЭД ЕАЭС, указав основной вариант и аргументацию по классификационным признакам.
4) Отдельно оценить возможную применимость технических регламентов ЕАЭС, ПП РФ 2425, санитарно-гигиенических требований/СГР и других обязательных требований.
5) Отдельно оценить возможность оформления отказного письма.

Критически важно:
- Решение об отказном письме нельзя принимать только по коду ТН ВЭД.
- Если назначение, материал, возрастная категория, состав или принцип работы способны изменить вывод — запроси уточнение.
- Не выдумывай номера нормативных актов, решения ЕЭК, ГОСТы, пункты или официальные источники.
- Если данных недостаточно для вывода — не угадывай, а запроси уточнение.
- Если по имеющимся данным обязательные разрешительные документы не требуются и нет выявленных препятствий, прямо сообщи клиенту: «Возможно оформить отказное письмо».
- После положительного результата предложи отправить заявку эксперту МСК-Сертификат именно для оформления отказного письма. Эксперт не подтверждает вывод агента, а принимает заявку в оформление.

Верни ТОЛЬКО JSON без markdown в формате:
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
    {"code": "", "confidence": "high|medium|low", "reason": ""}
  ],
  "regulatory": [
    {"area": "ТР ЕАЭС / ПП 2425 / СГР / другое", "result": "likely_applies|likely_not_applies|need_check", "reason": ""}
  ],
  "refusal_letter": {
    "result": "likely_possible|likely_not_possible|need_more_info",
    "reason": ""
  },
  "application_cta": "",
  "client_text": "Короткий понятный ответ клиенту на русском языке"
}
`;

function localHints(text) {
  const t = text.toLowerCase();
  return rules.rules.filter(r => (r.when?.keywords || []).some(k => t.includes(k))).map(r => ({id:r.id, ask:r.ask, comment:r.comment}));
}

function extractJson(text) {
  const trimmed = text.trim().replace(/^```json\s*/i, '').replace(/```$/,'').trim();
  return JSON.parse(trimmed);
}

app.post('/api/analyze', async (req, res) => {
  try {
    const { description, answers = {} } = req.body || {};
    if (!description || String(description).trim().length < 3) {
      return res.status(400).json({ error: 'Опишите товар.' });
    }

    const hints = localHints(description);
    const payload = {
      description,
      answers,
      internal_rule_hints: hints,
      rules_version: rules.version
    };

    if (!apiKey || !folderId) {
      return res.status(500).json({ error: 'На сервере не настроены YANDEX_API_KEY и YANDEX_FOLDER_ID.' });
    }

    const response = await client.chat.completions.create({
      model: `gpt://${folderId}/yandexgpt/latest`,
      messages: [
        { role: 'system', content: SYSTEM },
        { role: 'user', content: JSON.stringify(payload, null, 2) }
      ],
      temperature: 0.1,
      max_tokens: 3500
    });

    const outputText = response.choices?.[0]?.message?.content || '';
    let parsed;
    try {
      parsed = extractJson(outputText);
    } catch {
      return res.status(502).json({ error: 'Модель вернула ответ в неожиданном формате.', raw: outputText });
    }

    parsed.disclaimer = 'Результат сформирован автоматически на основании предоставленного описания товара. Для оформления отказного письма отправьте заявку эксперту МСК-Сертификат.';
    res.json(parsed);
  } catch (err) {
    console.error(err);
    res.status(500).json({ error: err?.message || 'Ошибка анализа' });
  }
});

app.listen(port, () => console.log(`TN VED agent MVP: http://localhost:${port}`));
