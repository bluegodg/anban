import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('child web shell exposes the expected PRD controls', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  for (const token of [
    'accessCode',
    'deviceId',
    'statusPanel',
    'messageForm',
    'greetingButton',
    'reminderForm',
    'profileForm',
  ]) {
    assert.match(html, new RegExp(token), `missing ${token}`);
  }
});

test('API client sends child access code and message payload', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ messageId: 7, status: 'played' }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.sendMessage({
    deviceId: 'dev-001',
    text: '妈，我到家了',
    fromName: '小明',
  });

  assert.equal(request.url, 'http://anban.local/api/messages');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.deepEqual(JSON.parse(request.options.body), {
    deviceId: 'dev-001',
    text: '妈，我到家了',
    fromName: '小明',
  });
  assert.equal(result.messageId, 7);
});

test('child web shows greeting text returned by backend', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /问候已触发：\$\{greeting\.text\}/);
});
