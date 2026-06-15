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
