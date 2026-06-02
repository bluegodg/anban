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

test('API client saves greeting schedule with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ deviceId: 'dev-001', slots: [{ label: 'morning', time: '08:00' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.updateGreetingSchedule({
    deviceId: 'dev-001',
    slots: [
      { label: 'morning', time: '08:00', enabled: true, tonePreset: 'warm' },
      { label: 'noon', time: '12:30', enabled: true, tonePreset: 'casual' },
      { label: 'evening', time: '18:00', enabled: true, tonePreset: 'warm' },
    ],
  });

  assert.equal(request.url, 'http://anban.local/api/greetings/schedule');
  assert.equal(request.options.method, 'PUT');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(JSON.parse(request.options.body).slots.length, 3);
  assert.equal(result.deviceId, 'dev-001');
});

test('API client fetches greeting schedule with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ deviceId: 'dev-001', slots: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  await client.getGreetingSchedule({ deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/greetings/schedule?deviceId=dev-001');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
});

test('child web lets children configure greeting schedule', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(html, /greetingScheduleForm/);
  assert.match(html, /morningGreetingTime/);
  assert.match(app, /client\(\)\.updateGreetingSchedule/);
  assert.match(app, /问候时段已保存/);
});

test('API client sends scheduled reminder payload', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ reminderId: 9, status: 'scheduled' }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.createReminder({
    deviceId: 'dev-001',
    scheduledAt: '2026-06-01T09:01:30.000Z',
    content: '测血压',
    category: 'med',
  });

  assert.equal(request.url, 'http://anban.local/api/reminders');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.deepEqual(JSON.parse(request.options.body), {
    deviceId: 'dev-001',
    scheduledAt: '2026-06-01T09:01:30.000Z',
    content: '测血压',
    category: 'med',
  });
  assert.equal(result.reminderId, 9);
});

test('child web shows reminder creation result returned by backend', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /提醒已创建：\$\{reminder\.content\}/);
});

test('API client cancels reminders with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ reminderId: 9, status: 'canceled' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.deleteReminder(9);

  assert.equal(request.url, 'http://anban.local/api/reminders/9');
  assert.equal(request.options.method, 'DELETE');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(result.status, 'canceled');
});

test('child web exposes reminder cancel client capability', async () => {
  const client = await readFile(new URL('./api/client.js', import.meta.url), 'utf8');

  assert.match(client, /deleteReminder/);
});

test('API client fetches device status with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ deviceId: 'dev-001', online: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.getStatus({ deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/device/status?deviceId=dev-001');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(result.online, true);
});

test('child web refreshes backend status before listing messages', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /client\(\)\.getStatus/);
  assert.match(app, /renderBackendStatus/);
});

test('API client updates family profile with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ deviceId: 'dev-001', fields: { nickname: '妈' } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.updateProfile({
    deviceId: 'dev-001',
    fields: {
      name: '王秀英',
      nickname: '妈',
      children: ['小明'],
      grandchildren: ['小宝'],
      hobbies: ['豫剧'],
      schedule: '早睡早起',
      health: '高血压',
      taboos: ['甜食'],
    },
  });

  assert.equal(request.url, 'http://anban.local/api/profile');
  assert.equal(request.options.method, 'PUT');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(JSON.parse(request.options.body).fields.nickname, '妈');
  assert.equal(result.fields.nickname, '妈');
});

test('child web submits profile to backend instead of local draft only', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /client\(\)\.updateProfile/);
  assert.match(app, /画像已同步/);
});
