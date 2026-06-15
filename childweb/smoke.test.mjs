import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const indexHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
const appJS = await readFile(new URL('./app.js', import.meta.url), 'utf8').catch(() => '');

test('P1 loads the Stitch application through an ES module entrypoint', () => {
  assert.match(indexHTML, /<script\s+type="module"\s+src="\.\/app\.js"><\/script>/);
  assert.doesNotMatch(indexHTML, /<script>\s*\/\/ ============================\s*\/\/ SPA Router/);
});

test('P1 reuses the verified web API client without rewriting it', async () => {
  const childClient = await readFile(new URL('./api/client.js', import.meta.url), 'utf8');
  const verifiedClient = await readFile(new URL('../web/api/client.js', import.meta.url), 'utf8');

  assert.equal(childClient, verifiedClient);
});

test('P1 config trims and persists backend connection settings', async () => {
  const { DEFAULT_CONFIG, loadConfig, saveConfig } = await import('./config.js');
  const values = new Map([
    ['anban_childweb_base_url', ' http://127.0.0.1:8090/ '],
    ['anban_childweb_access_code', ' demo-code '],
  ]);
  const storage = {
    getItem(key) {
      return values.has(key) ? values.get(key) : null;
    },
    setItem(key, value) {
      values.set(key, value);
    },
  };

  assert.equal(DEFAULT_CONFIG.deviceId, '9c:13:9e:8b:af:28');
  assert.deepEqual(loadConfig(storage), {
    baseURL: 'http://127.0.0.1:8090',
    accessCode: 'demo-code',
    deviceId: '9c:13:9e:8b:af:28',
  });

  const saved = saveConfig({
    baseURL: ' https://anban.example.com/ ',
    accessCode: ' next-code ',
    deviceId: ' aa:bb:cc:dd:ee:ff ',
  }, storage);

  assert.deepEqual(saved, {
    baseURL: 'https://anban.example.com',
    accessCode: 'next-code',
    deviceId: 'aa:bb:cc:dd:ee:ff',
  });
  assert.equal(values.get('anban_childweb_device_id'), 'aa:bb:cc:dd:ee:ff');
});

test('P1 unsupported features always use the required Chinese notice', async () => {
  const { NOT_IMPLEMENTED_MESSAGE, notImplemented } = await import('./not-implemented.js');
  const notices = [];

  assert.equal(NOT_IMPLEMENTED_MESSAGE, '该功能未实现');
  assert.equal(notImplemented('附件', notices.push.bind(notices)), '该功能未实现');
  assert.deepEqual(notices, ['该功能未实现']);
});

test('P1 declares detail edit state for ES module strict mode', () => {
  assert.match(appJS, /var _detailEditTarget;/);
});

test('P2 formats login failures without hiding an invalid access code', async () => {
  const { formatLoginError } = await import('./integration-core.js');

  assert.equal(formatLoginError({ status: 401 }), '访问码错误，请重新输入');
  assert.equal(formatLoginError(new Error('Failed to fetch')), '无法连接安伴服务，请检查后端地址');
  assert.equal(formatLoginError({ message: '服务繁忙', status: 503 }), '服务繁忙（503）');
});

test('P2 validates login through device status before persisting the session', async () => {
  const { DEFAULT_CONFIG } = await import('./config.js');

  assert.equal(DEFAULT_CONFIG.baseURL, 'http://127.0.0.1:8090');
  assert.match(appJS, /await candidateClient\.getStatus\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /updateAnbanConfig\(\{ accessCode \}\)/);
  assert.match(appJS, /localStorage\.setItem\('anban_session', '1'\)/);
});

test('P3 derives a compact home status from backend device state', async () => {
  const { buildHomeStatus, formatRelativeTime } = await import('./integration-core.js');
  const now = new Date('2026-06-15T12:00:00.000Z');

  assert.equal(formatRelativeTime('2026-06-15T11:55:00.000Z', now), '5分钟前');
  assert.deepEqual(buildHomeStatus({
    online: true,
    lastInteractionAt: '2026-06-15T11:55:00.000Z',
  }, now), {
    online: true,
    label: '在线',
    title: '设备在线',
    description: '最近互动 5分钟前',
    updatedAt: '刚刚更新',
  });
  assert.equal(buildHomeStatus({ online: false }, now).title, '设备暂时离线');
});

test('P3 normalizes recent backend history newest first', async () => {
  const { normalizeHistoryMessages } = await import('./integration-core.js');
  const result = normalizeHistoryMessages({ messages: [
    { role: 'user', text: '早上好', at: '2026-06-15T08:00:00Z' },
    { role: 'assistant', text: '记得吃药', at: '2026-06-15T08:01:00Z' },
    { role: 'assistant', text: '  ', at: '2026-06-15T08:02:00Z' },
  ] }, 2);

  assert.deepEqual(result.map((item) => [item.role, item.text]), [
    ['assistant', '记得吃药'],
    ['user', '早上好'],
  ]);
});

test('P3 home loads status and history through the shared client', () => {
  assert.match(appJS, /anbanClient\.getStatus\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.getHistory\(\{ deviceId: anbanConfig\.deviceId, limit: 10 \}\)/);
});

test('P4 merges backend conversation history and child messages into bubbles', async () => {
  const { buildConversationBubbles } = await import('./integration-core.js');
  const bubbles = buildConversationBubbles({
    history: { messages: [
      { role: 'user', text: '我很好', at: '2026-06-15T08:00:00Z' },
      { role: 'assistant', text: '那就好', at: '2026-06-15T08:01:00Z' },
    ] },
    messages: { messages: [
      { messageId: 'm1', text: '记得喝水', status: 'played', queuedAt: '2026-06-15T08:02:00Z' },
    ] },
  });

  assert.deepEqual(bubbles.map((item) => [item.side, item.text, item.status]), [
    ['left', '我很好', ''],
    ['left', '那就好', ''],
    ['right', '记得喝水', '已播报'],
  ]);
});

test('P4 loads and sends messages through the shared client', () => {
  assert.match(appJS, /anbanClient\.listMessages\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.getHistory\(\{ deviceId: anbanConfig\.deviceId, limit: 100 \}\)/);
  assert.match(appJS, /anbanClient\.sendMessage\(\{[\s\S]*deviceId: anbanConfig\.deviceId,[\s\S]*fromName: '家人',[\s\S]*text:/);
});

test('P4 unsupported message attachments use the unified notice', () => {
  assert.match(indexHTML, /onclick="notImplemented\('图片留言'\)"/);
  assert.match(indexHTML, /onclick="notImplemented\('语音留言'\)"/);
});

test('P5 computes the next one-time reminder in UTC', async () => {
  const { nextOccurrenceUTC } = await import('./integration-core.js');
  const today = new Date(nextOccurrenceUTC(8, 30, new Date(2026, 5, 15, 8, 0, 0)));
  const tomorrow = new Date(nextOccurrenceUTC(8, 30, new Date(2026, 5, 15, 8, 31, 0)));

  assert.equal(today.getDate(), 15);
  assert.equal(today.getHours(), 8);
  assert.equal(today.getMinutes(), 30);
  assert.equal(tomorrow.getDate(), 16);
  assert.equal(tomorrow.getHours(), 8);
  assert.equal(tomorrow.getMinutes(), 30);
});

test('P5 connects one-time reminder list, create, and delete APIs', () => {
  assert.match(appJS, /anbanClient\.listReminders\(\{ deviceId: anbanConfig\.deviceId, status: 'scheduled' \}\)/);
  assert.match(appJS, /anbanClient\.createReminder\(\{[\s\S]*scheduledAt:[\s\S]*content:[\s\S]*category:/);
  assert.match(appJS, /anbanClient\.deleteReminder\(reminderId\)/);
});

test('P5 defaults to one-time reminders and rejects unsupported controls', () => {
  assert.match(indexHTML, /id="freqDisplay">仅一次</);
  assert.match(indexHTML, /id="importantToggle" onclick="notImplemented\('重要提醒'\)"/);
  assert.match(appJS, /notImplemented\('重复提醒'\)/);
  assert.match(appJS, /notImplemented\('暂停提醒'\)/);
});

test('P6 maps Stitch profile data to backend fields without changing the contract', async () => {
  const { mapStitchProfileToFields } = await import('./integration-core.js');
  assert.deepEqual(mapStitchProfileToFields({
    name: '妈妈',
    hobbies: ['园艺'],
    habits: [{ icon: 'park', text: '每天散步' }],
    health: [{ name: '血压', detail: '每日测量' }],
    communicationDos: ['多聊家常'],
    communicationDonts: ['不要催促'],
    aiPortrait: '性格温和',
  }), {
    name: '妈妈',
    nickname: '妈妈',
    hobbies: ['园艺'],
    schedule: '每天散步\n沟通建议：多聊家常',
    health: 'AI画像：性格温和\n血压：每日测量',
    taboos: ['不要催促'],
  });
});

test('P6 maps backend fields back to the Stitch form and keeps local demographics', async () => {
  const { mapFieldsToStitchProfile } = await import('./integration-core.js');
  const result = mapFieldsToStitchProfile({
    name: '爸爸',
    hobbies: ['听戏'],
    schedule: '早上六点起床\n沟通建议：说慢一点',
    health: 'AI画像：开朗\n膝盖：避免久站',
    taboos: ['不要提旧伤'],
  }, { age: 75, livingSituation: '独居', occupation: '退休工人' });

  assert.equal(result.name, '爸爸');
  assert.equal(result.age, 75);
  assert.equal(result.habits[0].text, '早上六点起床');
  assert.deepEqual(result.communicationDos, ['说慢一点']);
  assert.equal(result.health[0].name, '膝盖');
  assert.equal(result.aiPortrait, '开朗');
});

test('P6 loads and saves profiles through the shared client', () => {
  assert.match(appJS, /anbanClient\.getProfile\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.updateProfile\(\{[\s\S]*deviceId: anbanConfig\.deviceId,[\s\S]*fields:/);
});

test('P7 removes the fixed phone shell and exposes PWA metadata', async () => {
  assert.doesNotMatch(indexHTML, /max-width:466px/);
  assert.doesNotMatch(indexHTML, /width:430px;height:932px/);
  assert.match(indexHTML, /rel="manifest" href="\.\/manifest\.webmanifest"/);
  assert.match(indexHTML, /name="apple-mobile-web-app-capable" content="yes"/);

  const manifest = JSON.parse(await readFile(new URL('./manifest.webmanifest', import.meta.url), 'utf8'));
  assert.equal(manifest.name, '安伴');
  assert.equal(manifest.display, 'standalone');
  assert.equal(manifest.theme_color, '#F78C6B');
  assert.deepEqual(manifest.icons.map((icon) => icon.sizes), ['192x192', '512x512']);
});

test('P7 service worker caches the shell but never caches API responses', async () => {
  const sw = await readFile(new URL('./sw.js', import.meta.url), 'utf8');
  assert.match(sw, /pathname\.startsWith\('\/api\/'\)/);
  assert.match(sw, /pathname\.startsWith\('\/api\/'\)[\s\S]*event\.respondWith\(fetch\(request\)\)/);
  assert.match(sw, /caches\.open/);
  assert.match(appJS, /navigator\.serviceWorker\.register\('\.\/sw\.js'\)/);
});

test('P7 PWA icons have the required PNG dimensions', async () => {
  for (const size of [192, 512]) {
    const png = await readFile(new URL(`./icons/icon-${size}.png`, import.meta.url));
    assert.equal(png.toString('ascii', 1, 4), 'PNG');
    assert.equal(png.readUInt32BE(16), size);
    assert.equal(png.readUInt32BE(20), size);
  }
});

test('P7 settings can update backend URL and device id', () => {
  assert.match(indexHTML, /id="settingsBaseURL"/);
  assert.match(indexHTML, /id="settingsDeviceId"/);
  assert.match(indexHTML, /id="saveConnectionBtn"/);
  assert.match(appJS, /updateAnbanConfig\(\{ baseURL: baseURL, deviceId: deviceId \}\)/);
});

test('P8 removes message and reminder mock storage paths', () => {
  assert.doesNotMatch(appJS, /anban_messages|anban_reminders|anban_reminder_history/);
  assert.doesNotMatch(appJS, /initHistoryMock|fetchWeather|updateRecentMessages|moveToHistory/);
});

test('P8 routes every visible unsupported entry through notImplemented', () => {
  for (const feature of ['忘记访问码', '新设备激活', '使用帮助', '联系客服', '环境状态']) {
    assert.match(indexHTML, new RegExp(`notImplemented\\('${feature}'\\)`));
  }
  assert.match(appJS, /window\.editDetailTime = function\(\) \{\s*return notImplemented\('编辑提醒'\);/);
});

test('P8 documents startup, deployment, and supported scope', async () => {
  const readme = await readFile(new URL('./README.md', import.meta.url), 'utf8');
  assert.match(readme, /npm start/);
  assert.match(readme, /http:\/\/127\.0\.0\.1:3001/);
  assert.match(readme, /HTTPS/);
  assert.match(readme, /已实现/);
  assert.match(readme, /未实现/);
});
