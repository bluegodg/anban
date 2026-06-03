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

test('child web applies message length notice before sending', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /normalizeMessageDraft/);
  assert.match(app, /draft\.notice/);
});

test('child web renders failed message returned by send error payload', async () => {
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(app, /upsertMessageFromSendError/);
  assert.match(app, /error\.payload\?\.message/);
  assert.match(app, /renderMessages\(\)/);
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

test('child web exposes vision presence trigger action', async () => {
  const html = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('./app.js', import.meta.url), 'utf8');

  assert.match(html, /visionPresenceButton/);
  assert.match(html, /visionPresenceResult/);
  assert.match(app, /client\(\)\.checkVisionPresence/);
  assert.match(app, /视觉触发结果/);
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
