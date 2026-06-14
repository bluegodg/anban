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

test('API client normalizes pasted backend base URL', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: '  http://anban.local///  ',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ messages: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  await client.listMessages({ deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/messages?deviceId=dev-001');
});

test('API client normalizes pasted child access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: '  demo-code  ',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ online: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  await client.getStatus({ deviceId: 'dev-001' });

  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
});

test('API client normalizes pasted device id query values', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ online: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  await client.getStatus({ deviceId: '  dev-001  ' });

  assert.equal(request.url, 'http://anban.local/api/device/status?deviceId=dev-001');
});

test('message draft normalizer truncates overlong text and reports child notice', async () => {
  const { MESSAGE_TEXT_LIMIT, normalizeMessageDraft } = await import('./message-input.js');
  const overlong = '妈'.repeat(MESSAGE_TEXT_LIMIT + 1);

  const result = normalizeMessageDraft(`  ${overlong}  `);

  assert.equal(Array.from(result.text).length, MESSAGE_TEXT_LIMIT);
  assert.equal(result.wasTruncated, true);
  assert.equal(result.notice, '留言已按 100 字发送');
});

test('message draft normalizer keeps short text without notice', async () => {
  const { normalizeMessageDraft } = await import('./message-input.js');

  const result = normalizeMessageDraft('  妈，我到家了  ');

  assert.deepEqual(result, {
    text: '妈，我到家了',
    wasTruncated: false,
    notice: '',
  });
});

test('reminder draft normalizer rejects non-future times', async () => {
  const { normalizeReminderDraft } = await import('./reminder-input.js');
  const now = new Date('2026-06-01T09:00');

  assert.deepEqual(normalizeReminderDraft({
    content: '  测血压  ',
    scheduledAtLocal: '2026-06-01T08:59',
    now,
  }), {
    content: '测血压',
    scheduledAt: '',
    valid: false,
    notice: '提醒时间要晚于现在',
  });
  assert.equal(normalizeReminderDraft({
    content: '测血压',
    scheduledAtLocal: '2026-06-01T09:00',
    now,
  }).notice, '提醒时间要晚于现在');
});

test('reminder draft normalizer rejects incomplete or invalid time input', async () => {
  const { normalizeReminderDraft } = await import('./reminder-input.js');

  assert.deepEqual(normalizeReminderDraft({
    content: '  ',
    scheduledAtLocal: '2026-06-01T09:01',
  }), {
    content: '',
    scheduledAt: '',
    valid: false,
    notice: '提醒内容和时间都要填写',
  });
  assert.equal(normalizeReminderDraft({
    content: '测血压',
    scheduledAtLocal: 'not-a-time',
  }).notice, '提醒时间格式无效');
});

test('reminder draft normalizer returns ISO time for future reminders', async () => {
  const { normalizeReminderDraft } = await import('./reminder-input.js');

  const result = normalizeReminderDraft({
    content: '  测血压  ',
    scheduledAtLocal: '2026-06-01T09:01',
    now: new Date('2026-06-01T09:00'),
  });

  assert.equal(result.content, '测血压');
  assert.equal(result.valid, true);
  assert.equal(result.notice, '');
  assert.equal(result.scheduledAt, new Date('2026-06-01T09:01').toISOString());
});

test('message state surfaces failed message returned by send API error', async () => {
  const { upsertMessage, upsertMessageFromSendError } = await import('./message-state.js');
  const pending = { messageId: 7, text: '妈，我下班路过老张家了', status: 'pending' };
  const older = { messageId: 3, text: '早一点的留言', status: 'played' };
  const failed = {
    messageId: 7,
    text: '妈，我下班路过老张家了',
    status: 'failed',
    errorMessage: 'manager unavailable',
  };

  const merged = upsertMessage([older, pending], { messageId: 9, text: '新留言', status: 'played' });
  assert.deepEqual(merged.map((item) => item.messageId), [9, 3, 7]);

  const unchanged = [older];
  assert.equal(upsertMessage(unchanged, {}), unchanged);
  assert.equal(upsertMessageFromSendError(unchanged, { payload: {} }), unchanged);

  const surfaced = upsertMessageFromSendError([pending, older], {
    payload: { message: failed },
  });

  assert.equal(surfaced[0].messageId, 7);
  assert.equal(surfaced[0].status, 'failed');
  assert.equal(surfaced[0].errorMessage, 'manager unavailable');
  assert.deepEqual(surfaced.map((item) => item.messageId), [7, 3]);
});

test('message send result formatter distinguishes queued messages', async () => {
  const { formatMessageSendResult } = await import('./message-result.js');

  assert.deepEqual(formatMessageSendResult({
    status: 'played',
    text: '妈，我到家了',
  }), {
    label: '在线',
    detail: '留言已播报',
    notice: '留言已播报：妈，我到家了',
  });

  assert.deepEqual(formatMessageSendResult({
    status: 'pending',
    text: '妈，我下班路过老张家了',
  }, { draftNotice: '留言已按 100 字发送' }), {
    label: '在线',
    detail: '留言已排队等待设备空闲',
    notice: '留言已排队：妈，我下班路过老张家了（留言已按 100 字发送）',
  });
});

test('child web applies message length notice before sending', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /normalizeMessageDraft/);
  assert.match(app, /draft\.notice/);
});

test('child web uses message send result formatter', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const messageHandler = app.slice(
    app.indexOf('els.messageForm.addEventListener'),
    app.indexOf('els.greetingButton.addEventListener'),
  );

  assert.match(app, /formatMessageSendResult/);
  assert.match(messageHandler, /const result = formatMessageSendResult\(message, \{ draftNotice: draft\.notice \}\);/);
  assert.match(messageHandler, /showNotice\(result\.notice\)/);
});

test('child web renders failed message returned by send error payload', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /upsertMessageFromSendError/);
  assert.match(app, /error\.payload\?\.message/);
  assert.match(app, /renderMessages\(\)/);
});

test('child web refreshes status card immediately after send result changes messages', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const messageHandler = app.slice(
    app.indexOf('els.messageForm.addEventListener'),
    app.indexOf('els.greetingButton.addEventListener'),
  );

  assert.match(messageHandler, /state\.messages = upsertMessage\(state\.messages, message\);\s*renderMessages\(\);\s*renderCurrentBackendStatus\(\);/);
  assert.match(messageHandler, /state\.messages = upsertMessageFromSendError\(state\.messages, error\);\s*renderMessages\(\);\s*renderCurrentBackendStatus\(\);/);
});

test('API error notice prefers backend reason over generic fallback', async () => {
  const { ApiError } = await import('./api/client.js');
  const { formatApiErrorNotice } = await import('./api-error-notice.js');

  assert.equal(
    formatApiErrorNotice(new ApiError('主动语音配额已用', 429, { error: '主动语音配额已用' }), '问候接口暂未接入'),
    '主动语音配额已用（429）',
  );
  assert.equal(
    formatApiErrorNotice(new Error('network down'), '问候接口暂未接入'),
    '问候接口暂未接入',
  );
});

test('child web uses API error notice formatter', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /formatApiErrorNotice/);
  assert.match(app, /showNotice\(formatApiErrorNotice\(error, fallback\)\)/);
});

test('greeting trigger result formatter shows played greeting text', async () => {
  const { formatGreetingTriggerResult } = await import('./greeting-result.js');

  assert.equal(
    formatGreetingTriggerResult({ status: 'played', text: '王阿姨，下午好~ 今天精神咋样？' }).notice,
    '问候已触发：王阿姨，下午好~ 今天精神咋样？',
  );
});

test('greeting trigger result formatter distinguishes queued greetings', async () => {
  const { formatGreetingTriggerResult } = await import('./greeting-result.js');

  assert.deepEqual(formatGreetingTriggerResult({
    status: 'played',
    text: '王阿姨，下午好~ 今天精神咋样？',
  }), {
    label: '在线',
    detail: '刚刚触发一次主动问候',
    notice: '问候已触发：王阿姨，下午好~ 今天精神咋样？',
  });

  assert.deepEqual(formatGreetingTriggerResult({
    status: 'pending',
    text: '王阿姨，下午好~ 今天精神咋样？',
  }), {
    label: '在线',
    detail: '主动问候已排队',
    notice: '问候已排队：王阿姨，下午好~ 今天精神咋样？',
  });
});

test('child web uses greeting trigger result formatter', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /formatGreetingTriggerResult/);
  assert.match(app, /const result = formatGreetingTriggerResult\(greeting\);/);
  assert.match(app, /renderStatus\(result\.label, result\.detail\)/);
  assert.match(app, /showNotice\(result\.notice\)/);
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

test('API client normalizes pasted reminder id path values', async () => {
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

  await client.deleteReminder(' 9 ');

  assert.equal(request.url, 'http://anban.local/api/reminders/9');
});

test('API client acknowledges reminders with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ reminderId: 9, status: 'completed', ackKind: 'voice' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.ackReminder(9, { ackKind: 'voice' });

  assert.equal(request.url, 'http://anban.local/api/reminders/9/ack');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.deepEqual(JSON.parse(request.options.body), { ackKind: 'voice' });
  assert.equal(result.status, 'completed');
});

test('child web exposes reminder cancel client capability', async () => {
  const client = await readFile(new URL('./api/client.js', import.meta.url), 'utf8');

  assert.match(client, /deleteReminder/);
  assert.match(client, /ackReminder/);
});

test('API client fetches reminders with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ reminders: [{ reminderId: 9, status: 'scheduled' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.listReminders({ deviceId: 'dev-001', status: 'scheduled' });

  assert.equal(request.url, 'http://anban.local/api/reminders?deviceId=dev-001&status=scheduled');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(result.reminders[0].reminderId, 9);
});

test('child web lists and cancels backend reminders', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(html, /reminderList/);
  assert.match(app, /client\(\)\.listReminders/);
  assert.match(app, /renderReminders/);
  assert.match(app, /client\(\)\.deleteReminder/);
  assert.match(app, /提醒已撤销/);
});

test('child web can mark played reminders completed for demo ack flow', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /reminder\.status === 'played'/);
  assert.match(app, /data-reminder-action="complete"/);
  assert.match(app, /client\(\)\.ackReminder\(button\.dataset\.reminderId,\s*\{\s*ackKind: 'voice'\s*\}\)/);
  assert.match(app, /提醒已完成：\$\{reminder\.content\}/);
});

test('child web labels reminder ack and timeout states', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /status === 'played'\) return '已播报'/);
  assert.match(app, /status === 'completed'\) return '已完成'/);
  assert.match(app, /status === 'unanswered'\) return '未应答'/);
  assert.match(app, /status === 'skipped'\) return '已跳过'/);
});

test('API client captures vision frame with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ deviceId: 'dev-001', tool: 'camera.capture', raw: { presence: 'someone' } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.captureVision({
    deviceId: 'dev-001',
    tool: 'camera.capture',
    args: { quality: 'low' },
  });

  assert.equal(request.url, 'http://anban.local/api/vision/capture');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.deepEqual(JSON.parse(request.options.body), {
    deviceId: 'dev-001',
    tool: 'camera.capture',
    args: { quality: 'low' },
  });
  assert.equal(result.raw.presence, 'someone');
});

test('child web exposes vision capture action', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(html, /visionButton/);
  assert.match(html, /visionResult/);
  assert.match(app, /client\(\)\.captureVision/);
  assert.match(app, /看一眼结果/);
});

test('child web uses real ESP32 camera MCP tool for vision actions', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /const VISION_CAPTURE_TOOL = 'self\.camera\.take_photo';/);
  assert.match(app, /tool: VISION_CAPTURE_TOOL/);
  assert.doesNotMatch(app, /tool: 'camera\.capture'/);
});

test('API client checks vision presence with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({
        capture: { raw: { presence: 'someone' } },
        observation: { presence: 'someone', triggeredGreeting: true },
      }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.checkVisionPresence({
    deviceId: 'dev-001',
    tool: 'camera.capture',
    args: { quality: 'low' },
  });

  assert.equal(request.url, 'http://anban.local/api/vision/check-presence');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.deepEqual(JSON.parse(request.options.body), {
    deviceId: 'dev-001',
    tool: 'camera.capture',
    args: { quality: 'low' },
  });
  assert.equal(result.observation.triggeredGreeting, true);
});

test('vision presence result formatter distinguishes queued greeting', async () => {
  const { formatVisionPresenceResult } = await import('./vision-presence-result.js');

  assert.deepEqual(formatVisionPresenceResult({
    observation: {
      presence: 'someone',
      triggeredGreeting: true,
      greeting: { status: 'played', text: '王阿姨，回来啦' },
    },
  }), {
    label: '在线',
    detail: '视觉触发了一次主动问候',
    notice: '视觉触发已完成',
    output: '视觉触发结果：有人 · 已触发问候',
  });

  assert.deepEqual(formatVisionPresenceResult({
    observation: {
      presence: 'someone',
      triggeredGreeting: true,
      greeting: { status: 'pending', text: '王阿姨，回来啦' },
    },
  }), {
    label: '在线',
    detail: '视觉触发的主动问候已排队',
    notice: '视觉触发已排队',
    output: '视觉触发结果：有人 · 问候已排队',
  });

  assert.deepEqual(formatVisionPresenceResult({
    observation: {
      presence: 'no_one',
      triggeredGreeting: false,
    },
  }), {
    label: '在线',
    detail: '刚刚完成一次视觉判定',
    notice: '视觉判定已返回',
    output: '视觉触发结果：无人 · 未触发问候',
  });

  assert.equal(formatVisionPresenceResult({ observation: {} }).output, '视觉触发结果：未知 · 未触发问候');
});

test('child web exposes vision presence trigger action', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const formatter = await readFile(new URL('./vision-presence-result.js', import.meta.url), 'utf8');

  assert.match(html, /visionPresenceButton/);
  assert.match(html, /visionPresenceResult/);
  assert.match(app, /client\(\)\.checkVisionPresence/);
  assert.match(app, /formatVisionPresenceResult/);
  assert.match(formatter, /视觉触发结果/);
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

test('API client fetches device conversation history with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({ deviceId: 'dev-001', messages: [{ role: 'user', text: '今天腰有点酸' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.getHistory({ deviceId: '  dev-001  ', limit: 20 });

  assert.equal(request.url, 'http://anban.local/api/device/history?deviceId=dev-001&limit=20');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(result.messages[0].text, '今天腰有点酸');
});

test('child web shows backend conversation history on connect', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const refreshBlock = app.slice(
    app.indexOf('async function refreshMessages'),
    app.indexOf('async function refreshBackendStatus'),
  );

  assert.match(html, /对话记录/);
  assert.match(html, /historyList/);
  assert.match(app, /history:/);
  assert.match(app, /client\(\)\.getHistory/);
  assert.match(refreshBlock, /await refreshHistory\(\)/);
  assert.match(app, /function renderHistory/);
});

test('status detail surfaces latest message playback state', async () => {
  const { formatStatusDetail, messageStatusLabel } = await import('./status-summary.js');

  const detail = formatStatusDetail({
    lastInteractionAt: '2026-06-01T08:30:00.000Z',
    messages: [{ messageId: 7, status: 'played' }],
  }, {
    formatDateTime: () => '06/01 08:30',
  });

  assert.equal(detail, '最近互动：06/01 08:30 · 最新留言：已播报');
  assert.equal(messageStatusLabel('failed'), '失败');
  assert.equal(messageStatusLabel('pending'), '排队中');
});

test('status detail keeps fallback when there is no message summary', async () => {
  const { formatStatusDetail } = await import('./status-summary.js');

  assert.equal(formatStatusDetail({}, { formatDateTime: () => 'unused' }), '暂无最近互动');
  assert.match(formatStatusDetail({
    lastSeenAt: '2026-06-01T08:30:00.000Z',
    messages: [{ messageId: 7, status: 'failed' }],
  }), /最新留言：失败/);
});

test('status display can be built from local messages before backend status exists', async () => {
  const { buildStatusSnapshotForDisplay, formatStatusDetail } = await import('./status-summary.js');

  const snapshot = buildStatusSnapshotForDisplay(null, [{ messageId: 7, status: 'played' }]);

  assert.equal(snapshot.online, true);
  assert.equal(
    formatStatusDetail(snapshot, { formatDateTime: () => 'unused' }),
    '暂无最近互动 · 最新留言：已播报',
  );
});

test('status display merges backend status with current local messages', async () => {
  const { buildStatusSnapshotForDisplay } = await import('./status-summary.js');

  assert.equal(buildStatusSnapshotForDisplay(null, []), null);

  const snapshot = buildStatusSnapshotForDisplay(
    {
      online: true,
      messages: [{ messageId: 3, status: 'pending' }],
    },
    [{ messageId: 7, status: 'failed' }],
  );

  assert.equal(snapshot.online, true);
  assert.deepEqual(snapshot.messages, [{ messageId: 7, status: 'failed' }]);
});

test('child web refreshes backend status before listing messages', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /client\(\)\.getStatus/);
  assert.match(app, /renderBackendStatus/);
});

test('child web status polling refreshes conversation history', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const statusRefresh = app.slice(
    app.indexOf('async function refreshBackendStatus'),
    app.indexOf('function restartStatusPolling'),
  );

  assert.match(statusRefresh, /const snapshot = await client\(\)\.getStatus\(\{ deviceId: state\.deviceId \}\);/);
  assert.match(statusRefresh, /updateStatusSnapshot\(snapshot\);[\s\S]*await refreshHistory\(\);/);
});

test('status polling schedules backend refresh every 30 seconds', async () => {
  const { STATUS_REFRESH_INTERVAL_MS, startStatusPolling, stopStatusPolling } = await import('./status-polling.js');
  let scheduledDelay;
  let refreshCalls = 0;
  let clearedTimer;

  const timer = startStatusPolling(() => {
    refreshCalls += 1;
  }, {
    setIntervalImpl: (fn, delay) => {
      scheduledDelay = delay;
      fn();
      return 42;
    },
  });
  stopStatusPolling(timer, {
    clearIntervalImpl: (id) => {
      clearedTimer = id;
    },
  });

  assert.equal(STATUS_REFRESH_INTERVAL_MS, 30_000);
  assert.equal(scheduledDelay, 30_000);
  assert.equal(refreshCalls, 1);
  assert.equal(clearedTimer, 42);
});

test('child web starts status polling after connect', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /startStatusPolling/);
  assert.match(app, /stopStatusPolling/);
  assert.match(app, /restartStatusPolling/);
  assert.match(app, /refreshBackendStatus/);
});

test('child web validates connection settings before backend calls', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const submitIndex = app.indexOf("els.connectForm.addEventListener('submit'");
  const validationIndex = app.indexOf('后端地址、访问码和设备 ID 必填', submitIndex);
  const refreshIndex = app.indexOf('await refreshMessages()', submitIndex);

  assert.notEqual(submitIndex, -1);
  assert.notEqual(validationIndex, -1);
  assert.notEqual(refreshIndex, -1);
  assert.ok(validationIndex < refreshIndex);
  assert.match(app.slice(submitIndex, refreshIndex), /!state\.apiBaseURL\s*\|\|\s*!state\.accessCode\s*\|\|\s*!state\.deviceId/);
});

test('child web backend address placeholder matches required static deployment', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(html, /id="apiBaseURL"[^>]*placeholder="http:\/\/localhost:8090"/);
  assert.doesNotMatch(html, /同源留空/);
});

test('child web defaults backend address to local anban server for Gate C', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const stateStart = app.indexOf('const state = {');
  const stateEnd = app.indexOf('\n};', stateStart);
  const stateBlock = app.slice(stateStart, stateEnd);

  assert.notEqual(stateStart, -1);
  assert.notEqual(stateEnd, -1);
  assert.match(stateBlock, /apiBaseURL:\s*localStorage\.getItem\('anban\.apiBaseURL'\)\s*\|\|\s*'http:\/\/localhost:8090'/);
});

test('child web stops existing polling when connection settings become invalid', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const submitIndex = app.indexOf("els.connectForm.addEventListener('submit'");
  const validationIndex = app.indexOf('后端地址、访问码和设备 ID 必填', submitIndex);
  const invalidBranch = app.slice(submitIndex, validationIndex);

  assert.notEqual(submitIndex, -1);
  assert.notEqual(validationIndex, -1);
  assert.match(invalidBranch, /stopConnectionPolling\(\)/);
  assert.match(app, /function stopConnectionPolling\(\)/);
});

test('child web clears stale device data when connection settings become invalid', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const submitIndex = app.indexOf("els.connectForm.addEventListener('submit'");
  const validationIndex = app.indexOf('后端地址、访问码和设备 ID 必填', submitIndex);
  const invalidBranch = app.slice(submitIndex, validationIndex);
  const clearStart = app.indexOf('function clearConnectionData()');
  const clearEnd = app.indexOf('\n}\n\nasync function refreshReminders', clearStart);
  const clearBlock = app.slice(clearStart, clearEnd);

  assert.notEqual(submitIndex, -1);
  assert.notEqual(validationIndex, -1);
  assert.match(invalidBranch, /clearConnectionData\(\)/);
  assert.notEqual(clearStart, -1);
  assert.notEqual(clearEnd, -1);
  assert.match(clearBlock, /state\.messages\s*=\s*\[\]/);
  assert.match(clearBlock, /state\.reminders\s*=\s*\[\]/);
  assert.match(clearBlock, /renderMessages\(\)/);
  assert.match(clearBlock, /renderReminders\(\)/);
  assert.match(clearBlock, /clearProfile\(\)/);
});

test('child web starts polling only after a successful backend refresh', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const submitIndex = app.indexOf("els.connectForm.addEventListener('submit'");
  const restartIndex = app.indexOf('restartStatusPolling()', submitIndex);
  const submitBlock = app.slice(submitIndex, restartIndex);
  const refreshIndex = app.indexOf('async function refreshMessages()');
  const refreshEnd = app.indexOf('\n}\n\nasync function refreshBackendStatus', refreshIndex);
  const refreshBlock = app.slice(refreshIndex, refreshEnd);

  assert.notEqual(submitIndex, -1);
  assert.notEqual(restartIndex, -1);
  assert.match(submitBlock, /if\s*\(\s*!\s*await refreshMessages\(\)\s*\)\s*{\s*return;\s*}/);
  assert.notEqual(refreshIndex, -1);
  assert.notEqual(refreshEnd, -1);
  assert.match(refreshBlock, /return true;/);
  assert.match(refreshBlock, /return false;/);
});

test('message status polling schedules backend refresh every 10 seconds', async () => {
  const {
    MESSAGE_STATUS_REFRESH_INTERVAL_MS,
    startMessageStatusPolling,
    stopMessageStatusPolling,
  } = await import('./message-status-polling.js');
  let scheduledDelay;
  let refreshCalls = 0;
  let clearedTimer;

  const timer = startMessageStatusPolling(() => {
    refreshCalls += 1;
  }, {
    setIntervalImpl: (fn, delay) => {
      scheduledDelay = delay;
      fn();
      return 24;
    },
  });
  stopMessageStatusPolling(timer, {
    clearIntervalImpl: (id) => {
      clearedTimer = id;
    },
  });

  assert.equal(MESSAGE_STATUS_REFRESH_INTERVAL_MS, 10_000);
  assert.equal(scheduledDelay, 10_000);
  assert.equal(refreshCalls, 1);
  assert.equal(clearedTimer, 24);
});

test('child web starts message status polling after connect', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /startMessageStatusPolling/);
  assert.match(app, /stopMessageStatusPolling/);
  assert.match(app, /restartMessageStatusPolling/);
  assert.match(app, /refreshBackendMessages/);
});

test('child web refreshes status card after message status polling updates', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /statusSnapshot: null/);
  assert.match(app, /function renderCurrentBackendStatus/);
  assert.match(app, /refreshBackendMessages[\s\S]*state\.messages = payload\.messages \|\| \[\];[\s\S]*renderMessages\(\);[\s\S]*renderCurrentBackendStatus\(\);/);
});

test('reminder status polling schedules backend refresh every 10 seconds', async () => {
  const {
    REMINDER_STATUS_REFRESH_INTERVAL_MS,
    startReminderStatusPolling,
    stopReminderStatusPolling,
  } = await import('./reminder-status-polling.js');
  let scheduledDelay;
  let refreshCalls = 0;
  let clearedTimer;

  const timer = startReminderStatusPolling(() => {
    refreshCalls += 1;
  }, {
    setIntervalImpl: (fn, delay) => {
      scheduledDelay = delay;
      fn();
      return 33;
    },
  });
  stopReminderStatusPolling(timer, {
    clearIntervalImpl: (id) => {
      clearedTimer = id;
    },
  });

  assert.equal(REMINDER_STATUS_REFRESH_INTERVAL_MS, 10_000);
  assert.equal(scheduledDelay, 10_000);
  assert.equal(refreshCalls, 1);
  assert.equal(clearedTimer, 33);
});

test('child web starts reminder status polling after connect', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /startReminderStatusPolling/);
  assert.match(app, /stopReminderStatusPolling/);
  assert.match(app, /restartReminderStatusPolling/);
  assert.match(app, /refreshBackendReminders/);
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

test('API client fetches family profile with access code', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    accessCode: 'demo-code',
    fetchImpl: async (url, options) => {
      request = { url, options };
      return new Response(JSON.stringify({
        deviceId: 'dev-001',
        fields: { nickname: '妈', hobbies: ['豫剧'] },
      }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const result = await client.getProfile({ deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/profile?deviceId=dev-001');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.headers['X-Access-Code'], 'demo-code');
  assert.equal(result.fields.nickname, '妈');
});

test('child web submits profile to backend instead of local draft only', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /client\(\)\.updateProfile/);
  assert.match(app, /画像已同步/);
});

test('child web writes backend-normalized profile back after save', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const submitIndex = app.indexOf("els.profileForm.addEventListener('submit'");
  const updateIndex = app.indexOf('client().updateProfile', submitIndex);
  const renderIndex = app.indexOf('renderProfile(profile)', updateIndex);
  const catchIndex = app.indexOf('} catch (error) {', renderIndex);
  const submitSuccessBlock = app.slice(updateIndex, catchIndex);
  const writeIndex = submitSuccessBlock.indexOf('writeProfileForm(profile)');

  assert.notEqual(submitIndex, -1);
  assert.notEqual(updateIndex, -1);
  assert.notEqual(renderIndex, -1);
  assert.notEqual(catchIndex, -1);
  assert.notEqual(writeIndex, -1);
  assert.ok(writeIndex > submitSuccessBlock.indexOf('renderProfile(profile)'));
});

test('child web keeps backend-persisted profile when xiaozhi profile sync fails', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const submitIndex = app.indexOf("els.profileForm.addEventListener('submit'");
  const catchIndex = app.indexOf('} catch (error) {', submitIndex);
  const submitEnd = app.indexOf('\n});', catchIndex);
  const catchBlock = app.slice(catchIndex, submitEnd);

  assert.notEqual(submitIndex, -1);
  assert.notEqual(catchIndex, -1);
  assert.notEqual(submitEnd, -1);
  assert.match(catchBlock, /error\.payload\?\.profile/);
  assert.match(catchBlock, /renderProfile\(error\.payload\.profile\)/);
  assert.match(catchBlock, /writeProfileForm\(error\.payload\.profile\)/);
  assert.ok(catchBlock.indexOf('writeProfileForm(error.payload.profile)') < catchBlock.indexOf("handleApiError(error, '画像同步失败')"));
});

test('profile form writer clears fields missing from backend profile', async () => {
  const { writeProfileFormFields } = await import('./profile-form.js');
  const inputs = {
    elderName: { value: '王秀英' },
    nickname: { value: '王阿姨' },
    children: { value: '小明, 小红' },
    grandchildren: { value: '小宝（7岁）' },
    hobby: { value: '豫剧, 下棋' },
    schedule: { value: '早睡早起' },
    health: { value: '高血压' },
    taboos: { value: '甜食' },
  };
  const form = {
    elements: {
      namedItem: (name) => inputs[name] || null,
    },
  };

  writeProfileFormFields(form, {
    nickname: '妈',
    hobbies: ['豫剧'],
  });

  assert.equal(inputs.elderName.value, '');
  assert.equal(inputs.nickname.value, '妈');
  assert.equal(inputs.children.value, '');
  assert.equal(inputs.grandchildren.value, '');
  assert.equal(inputs.hobby.value, '豫剧');
  assert.equal(inputs.schedule.value, '');
  assert.equal(inputs.health.value, '');
  assert.equal(inputs.taboos.value, '');
});

test('child web loads saved profile on connect', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /refreshProfile/);
  assert.match(app, /client\(\)\.getProfile/);
  assert.match(app, /writeProfileFormFields/);
});

test('child web clears sample profile when backend has no saved profile', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');
  const refreshIndex = app.indexOf('async function refreshProfile()');
  const catchIndex = app.indexOf('} catch (error) {', refreshIndex);
  const endIndex = app.indexOf('\n}\n\nfunction renderBackendStatus', catchIndex);
  const refreshBlock = app.slice(refreshIndex, endIndex);
  const notFoundIndex = refreshBlock.indexOf('error.status === 404');
  const clearIndex = refreshBlock.indexOf('clearProfile()', notFoundIndex);

  assert.notEqual(refreshIndex, -1);
  assert.notEqual(catchIndex, -1);
  assert.notEqual(endIndex, -1);
  assert.notEqual(notFoundIndex, -1);
  assert.notEqual(clearIndex, -1);
  assert.ok(clearIndex > notFoundIndex);
});
