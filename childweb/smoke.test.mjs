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

  assert.equal(childClient.replace(/\r\n/g, '\n'), verifiedClient.replace(/\r\n/g, '\n'));
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

test('P2 supports account login, session restore, and legacy demo login', async () => {
  const { DEFAULT_CONFIG } = await import('./config.js');

  assert.equal(DEFAULT_CONFIG.baseURL, 'http://127.0.0.1:8090');
  assert.match(appJS, /candidateClient\.login\(\{ phone: phone, password: password \}\)/);
  assert.match(appJS, /candidateClient\.getStatus\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /localStorage\.setItem\('anban_account_token', anbanSession\.token\)/);
  assert.match(appJS, /await anbanClient\.getMe\(\)/);
  assert.match(appJS, /await candidateClient\.getStatus\(\{ deviceId: anbanConfig\.deviceId \}\)/);
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

test('P3 home loads compact status and latest message state through the shared client', () => {
  assert.match(appJS, /anbanClient\.getStatus\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.listMessages\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.doesNotMatch(appJS, /anbanClient\.getHistory\(\{ deviceId: anbanConfig\.deviceId, limit: 10 \}\)/);
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

test('P4 loads a unified timeline and does not trust account-mode fromName', () => {
  assert.match(appJS, /anbanClient\.getTimeline\(\{/);
  assert.match(appJS, /if \(isAccountMode\(\)\) return anbanClient\.sendMessage\(\{ text: text \}\)/);
  assert.match(appJS, /sourceLabel/);
  assert.match(appJS, /statusLabels = \{ played: '已播报', pending: '待播报', failed: '发送失败' \}/);
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

test('W1.2 maps reminder frequency labels to recurrence payload fields', async () => {
  const { buildReminderScheduleOptions } = await import('./integration-core.js');

  assert.deepEqual(buildReminderScheduleOptions('仅一次'), {
    recurrence: 'none',
    customDates: [],
  });
  assert.deepEqual(buildReminderScheduleOptions('每天'), {
    recurrence: 'daily',
    customDates: [],
  });
  assert.deepEqual(buildReminderScheduleOptions('工作日'), {
    recurrence: 'weekdays',
    customDates: [],
  });
  assert.deepEqual(buildReminderScheduleOptions('周末'), {
    recurrence: 'weekends',
    customDates: [],
  });
  assert.deepEqual(buildReminderScheduleOptions('6月20日 等2天', ['2026-06-20', '2026-06-22']), {
    recurrence: 'custom-dates',
    customDates: ['2026-06-20', '2026-06-22'],
  });
});

test('P5 connects one-time reminder list, create, and delete APIs', () => {
  assert.match(appJS, /anbanClient\.listReminders\(\{ deviceId: anbanConfig\.deviceId, status: 'scheduled' \}\)/);
  assert.match(appJS, /anbanClient\.createReminder\(\{[\s\S]*scheduledAt:[\s\S]*content:[\s\S]*category:/);
  assert.match(appJS, /anbanClient\.deleteReminder\(reminderId\)/);
});

test('P5 defaults to one-time reminders and keeps pause unsupported', () => {
  assert.match(indexHTML, /id="freqDisplay">仅一次</);
  assert.match(appJS, /notImplemented\('暂停提醒'\)/);
});

test('W1.2 childweb sends recurrence and important reminder fields', () => {
  assert.doesNotMatch(indexHTML, /id="importantToggle" onclick="notImplemented\('重要提醒'\)"/);
  assert.doesNotMatch(appJS, /notImplemented\('重复提醒'\)|notImplemented\('重要提醒'\)/);
  assert.match(appJS, /buildReminderScheduleOptions\(freq, customDates\)/);
  assert.match(appJS, /recurrence: scheduleOptions\.recurrence/);
  assert.match(appJS, /customDates: scheduleOptions\.customDates/);
  assert.match(appJS, /important: isImportant/);
});

test('reminder information architecture uses a create sheet and dedicated list page', () => {
  for (const id of [
    'nextReminderSummary',
    'openReminderCreateButton',
    'openReminderListButton',
    'openReminderHistoryButton',
    'reminderCreateOverlay',
    'reminderCreateSheet',
    'reminderCreateClose',
    's-reminder-list',
    'reminderList',
  ]) {
    assert.match(indexHTML, new RegExp(`id="${id}"`));
  }
  for (const sample of ['饭后半小时服用', '保持身体水分', '营养均衡，按时吃饭', '早点休息，养成好习惯']) {
    assert.doesNotMatch(indexHTML, new RegExp(sample));
  }
  assert.match(appJS, /'reminder-list':'s-reminder-list'/);
  assert.match(appJS, /function initReminderList\(\)/);
  assert.match(appJS, /if \(!SPA\.initialized\.warn\)[\s\S]*initWarn\(\)/);
  assert.match(appJS, /function openReminderCreateModal\(\)/);
  assert.match(appJS, /function closeReminderCreateModal\(\)/);
  assert.match(appJS, /renderNextReminder\(reminders\)/);
  assert.match(appJS, /closeReminderCreateModal\(\);[\s\S]*await loadSavedReminders\(\)/);
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
    aiPortraitMode: 'manual',
  }), {
    name: '妈妈',
    nickname: '妈妈',
    hobbies: ['园艺'],
    schedule: '每天散步\n沟通建议：多聊家常',
    aiPortrait: '性格温和',
    aiPortraitMode: 'manual',
    health: '血压：每日测量',
    taboos: ['不要催促'],
  });
});

test('P6 maps backend fields back to the Stitch form and keeps local demographics', async () => {
  const { mapFieldsToStitchProfile } = await import('./integration-core.js');
  const result = mapFieldsToStitchProfile({
    name: '爸爸',
    hobbies: ['听戏'],
    schedule: '早上六点起床\n沟通建议：说慢一点',
    aiPortrait: '开朗',
    aiPortraitMode: 'auto',
    health: '膝盖：避免久站',
    taboos: ['不要提旧伤'],
  }, { age: 75, livingSituation: '独居', occupation: '退休工人' });

  assert.equal(result.name, '爸爸');
  assert.equal(result.age, 75);
  assert.equal(result.habits[0].text, '早上六点起床');
  assert.deepEqual(result.communicationDos, ['说慢一点']);
  assert.equal(result.health[0].name, '膝盖');
  assert.equal(result.aiPortrait, '开朗');
  assert.equal(result.aiPortraitMode, 'auto');
});

test('P6 still reads a legacy portrait embedded in health', async () => {
  const { mapFieldsToStitchProfile } = await import('./integration-core.js');
  const result = mapFieldsToStitchProfile({ health: 'AI画像：旧画像\n血压：每日测量' });
  assert.equal(result.aiPortrait, '旧画像');
  assert.equal(result.aiPortraitMode, 'auto');
  assert.deepEqual(result.health, [{ name: '血压', detail: '每日测量' }]);
});

test('family portrait editor exposes automatic and manual modes', async () => {
  const integrationCoreJS = await readFile(new URL('./integration-core.js', import.meta.url), 'utf8');
  assert.match(indexHTML, /id="editAiPortraitAuto"/);
  assert.doesNotMatch(indexHTML, /退休前曾是一名优秀教师/);
  assert.match(indexHTML, /画像会在资料和专属记忆积累后自动形成/);
  assert.match(appJS, /editAiPortraitAuto/);
  assert.match(appJS, /aiPortraitMode/);
  assert.doesNotMatch(integrationCoreJS, /`AI画像：\$\{portrait\}`/);
});

test('P6 loads and saves profiles through the shared client', () => {
  assert.match(appJS, /anbanClient\.getProfile\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.updateProfile\(\{[\s\S]*deviceId: anbanConfig\.deviceId,[\s\S]*fields:/);
});

test('family page exposes editable memory library through the shared client', () => {
  assert.match(indexHTML, /专属记忆/);
  assert.match(indexHTML, /id="memoryFacts"/);
  assert.match(indexHTML, /id="memoryInput"/);
  assert.match(appJS, /anbanClient\.listMemoryFacts\(\{ deviceId: anbanConfig\.deviceId, limit: 20 \}\)/);
  assert.match(appJS, /anbanClient\.addMemoryFact\(\{ deviceId: anbanConfig\.deviceId, text: text \}\)/);
  assert.match(appJS, /anbanClient\.updateMemoryFact\(fact\.factId/);
  assert.match(appJS, /anbanClient\.deleteMemoryFact\(fact\.factId/);
});

test('family and settings use compact landing pages with dedicated detail routes', () => {
  for (const id of [
    's-family-profile',
    's-family-memory',
    's-settings-account',
    's-settings-device',
    's-settings-connection',
    's-settings-greeting',
  ]) {
    assert.match(indexHTML, new RegExp(`id="${id}"`));
  }

  for (const route of [
    'family-profile',
    'family-memory',
    'settings-account',
    'settings-device',
    'settings-connection',
    'settings-greeting',
  ]) {
    assert.match(appJS, new RegExp(`'${route}'`));
  }

  assert.doesNotMatch(indexHTML, /id="clearCacheBtn"|id="cacheSize"|id="aboutBtn"|v 2\.4\.0/);
  assert.doesNotMatch(indexHTML, /晨间 6:30 起床|饭后必喝一杯龙井|退休教师/);
  assert.doesNotMatch(appJS, /晨间 6:30 起床|饭后必喝一杯龙井|退休教师/);
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
  assert.match(sw, /anban-childweb-v7/);
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
  for (const feature of ['使用帮助', '联系客服']) {
    assert.match(indexHTML, new RegExp(`notImplemented\\('${feature}'\\)`));
  }

  assert.doesNotMatch(indexHTML, /环境状态|envTemp|envHumidity/);
  assert.match(appJS, /window\.editDetailTime = function\(\) \{\s*return notImplemented\('编辑提醒'\);/);
});

test('account role hides every profile edit entry from members', () => {
  assert.match(appJS, /document\.querySelectorAll\('a\[href="#family-edit"\]'\)\.forEach/);
  assert.match(appJS, /anbanSession\.binding\.role === 'admin'/);
  assert.match(appJS, /只有家庭管理员可以编辑家人画像/);
});

test('account API client sends bearer auth and blocks unbound device fetches', async () => {
  const { createAnbanClient, ApiError } = await import('./api/client.js');
  const calls = [];
  const fetchImpl = async (url, init) => {
    calls.push({ url, init });
    return new Response(JSON.stringify({ account: { accountId: 1 }, binding: null }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  };
  const client = createAnbanClient({
    baseURL: 'http://127.0.0.1:8090',
    token: 'session-token',
    isBound: false,
    fetchImpl,
  });

  await client.getMe();
  assert.equal(calls[0].init.headers.Authorization, 'Bearer session-token');
  await assert.rejects(client.getStatus(), (error) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.payload.error, 'device_not_bound');
    return true;
  });
  assert.equal(calls.length, 1);
});

test('P8 documents startup, deployment, and supported scope', async () => {
  const readme = await readFile(new URL('./README.md', import.meta.url), 'utf8');
  assert.match(readme, /npm start/);
  assert.match(readme, /http:\/\/127\.0\.0\.1:3001/);
  assert.match(readme, /HTTPS/);
  assert.match(readme, /已实现/);
  assert.match(readme, /未实现/);
});

test('W1.1 formats greeting trigger result for played and queued greetings', async () => {
  const { formatGreetingTriggerResult } = await import('./integration-core.js');

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

test('W1.1 normalizes morning noon and evening greeting slots', async () => {
  const { normalizeGreetingSlots } = await import('./integration-core.js');

  assert.deepEqual(normalizeGreetingSlots([
    { label: 'morning', time: ' 07:30 ', enabled: true, tonePreset: 'warm' },
    { label: 'noon', time: 'bad', enabled: true, tonePreset: 'unknown' },
  ]), [
    { label: 'morning', time: '07:30', enabled: true, tonePreset: 'warm' },
    { label: 'noon', time: '12:30', enabled: true, tonePreset: 'warm' },
    { label: 'evening', time: '18:00', enabled: true, tonePreset: 'warm' },
  ]);
});

test('W1.1 childweb can trigger greetings and configure greeting schedule', () => {
  assert.match(indexHTML, /id="greetingTriggerBtn"/);
  assert.match(indexHTML, /id="greetingStatusText"/);
  assert.match(indexHTML, /id="greetingScheduleForm"/);
  for (const id of ['morningGreetingTime', 'noonGreetingTime', 'eveningGreetingTime']) {
    assert.match(indexHTML, new RegExp(`id="${id}"`));
  }

  assert.match(appJS, /anbanClient\.triggerGreeting\(\{ deviceId: anbanConfig\.deviceId, tonePreset: 'warm' \}\)/);
  assert.match(appJS, /formatGreetingTriggerResult\(greeting\)/);
  assert.match(appJS, /anbanClient\.getGreetingSchedule\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.updateGreetingSchedule\(\{[\s\S]*deviceId: anbanConfig\.deviceId,[\s\S]*slots:/);
  assert.match(appJS, /问候时段已保存/);
});

test('W1.4 formats vision presence results for childweb', async () => {
  const { formatVisionPresenceResult } = await import('./integration-core.js');

  assert.deepEqual(formatVisionPresenceResult({
    observation: {
      presence: 'someone',
      triggeredGreeting: true,
      greeting: { status: 'pending', text: '王阿姨，回来啦' },
    },
  }), {
    detail: '看见有人，问候已排队',
    notice: '视觉触发问候已排队：王阿姨，回来啦',
  });

  assert.deepEqual(formatVisionPresenceResult({
    observation: { presence: 'no_one', triggeredGreeting: false },
  }), {
    detail: '暂时没有看到老人',
    notice: '看一眼完成：暂时没有看到老人',
  });
});

test('W1.4 childweb exposes the manual look action through the original-image flow', () => {
  assert.match(indexHTML, /id="visionLookButton"/);
  assert.match(indexHTML, /看一眼/);
  assert.match(indexHTML, /id="visionStatusText"/);
  for (const id of [
    'visionHistoryButton',
    'visionHistoryOverlay',
    'visionHistorySheet',
    'visionHistoryClose',
    'visionHistoryCount',
    'visionHistoryContent',
    'visionDeleteConfirm',
  ]) {
    assert.match(indexHTML, new RegExp(`id="${id}"`));
  }
  for (const id of [
    'visionResultOverlay',
    'visionResultImage',
    'visionResultClose',
    'visionResultStatus',
    'visionResultSummary',
    'visionResultMeta',
    'visionResultPresence',
    'visionResultConcerns',
    'visionResultAction',
  ]) {
    assert.match(indexHTML, new RegExp(`id="${id}"`));
  }
  assert.match(appJS, /anbanClient\.lookVision\(\{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.getVisionCaptureImage\(capture\.captureId, \{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanClient\.listVisionCaptures\(\{ deviceId: anbanConfig\.deviceId, limit: 100 \}\)/);
  assert.match(appJS, /groupVisionCapturesByDate\(captures\)/);
  assert.match(appJS, /new IntersectionObserver/);
  assert.match(appJS, /anbanClient\.deleteVisionCapture\(captureId, \{ deviceId: anbanConfig\.deviceId \}\)/);
  assert.match(appJS, /anbanSession\.binding\.role === 'admin'/);
  assert.match(appJS, /URL\.createObjectURL\(blob\)/);
  assert.match(appJS, /URL\.revokeObjectURL\(visionImageObjectURL\)/);
  assert.match(appJS, /URL\.revokeObjectURL\(url\)/);
  assert.match(appJS, /generation !== visionHistoryLoadGeneration/);
  assert.doesNotMatch(appJS, /anbanClient\.checkVisionPresence\(\{ deviceId: anbanConfig\.deviceId, tool: VISION_CAPTURE_TOOL \}\)/);
});

test('first information-architecture slice removes the fake status bar and inline home lists', () => {
  assert.doesNotMatch(indexHTML, /globalStatusBar|status-bar-time|status-bar-icons/);
  assert.doesNotMatch(indexHTML, /\.spa-section\s*\{padding-top:54px\}/);
  assert.doesNotMatch(indexHTML, /\.spa-section\s*>\s*header\s*\{top:54px/);
  assert.doesNotMatch(indexHTML, /id="visionRecentList"|id="recentMsgList"/);
  assert.doesNotMatch(appJS, /updateStatusBarTime|renderVisionRecent|renderRecentHistory/);
  assert.match(indexHTML, /id="latestMessageStatus"/);
});

test('vision API starts a manual look through the authenticated device endpoint', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    token: 'session-token',
    isBound: true,
    fetchImpl: async (url, init) => {
      request = { url, init };
      return new Response(JSON.stringify({
        captureId: 'cap_123',
        status: 'succeeded',
        imageUrl: '/api/vision/captures/cap_123/image',
        analysis: { summary: '老人正在沙发上休息', presence: 'someone', concerns: [] },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    },
});

  const result = await client.lookVision({ deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/vision/look');
  assert.equal(request.init.method, 'POST');
  assert.equal(request.init.headers.Authorization, 'Bearer session-token');
  assert.deepEqual(JSON.parse(request.init.body), { deviceId: 'dev-001' });
  assert.equal(result.captureId, 'cap_123');
});

test('vision API reads the original capture as an authenticated blob', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    token: 'session-token',
    isBound: true,
    fetchImpl: async (url, init) => {
      request = { url, init };
      return new Response(new Uint8Array([0xff, 0xd8, 0xff, 0xd9]), {
        status: 200,
        headers: { 'Content-Type': 'image/jpeg' },
      });
    },
  });

  const image = await client.getVisionCaptureImage('cap/123', { deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/vision/captures/cap%2F123/image?deviceId=dev-001');
  assert.equal(request.init.headers.Authorization, 'Bearer session-token');
  assert.equal(image.type, 'image/jpeg');
  assert.equal(image.size, 4);
});

test('vision API restores recent captures for the bound device', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    token: 'session-token',
    isBound: true,
    fetchImpl: async (url, init) => {
      request = { url, init };
      return new Response(JSON.stringify([{ captureId: 'cap_recent', status: 'succeeded' }]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });

  const captures = await client.listVisionCaptures({ deviceId: 'dev-001', limit: 5 });

  assert.equal(request.url, 'http://anban.local/api/vision/captures?deviceId=dev-001&limit=5');
  assert.equal(request.init.headers.Authorization, 'Bearer session-token');
  assert.equal(captures[0].captureId, 'cap_recent');
});

test('vision API reanalyzes a saved partial capture', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    token: 'session-token',
    isBound: true,
    fetchImpl: async (url, init) => {
      request = { url, init };
      return new Response(JSON.stringify({
        captureId: 'cap_partial',
        status: 'succeeded',
        analysis: { summary: '重新分析完成', presence: 'someone', concerns: [] },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    },
  });

  const capture = await client.reanalyzeVisionCapture('cap_partial', { deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/vision/captures/cap_partial/reanalyze?deviceId=dev-001');
  assert.equal(request.init.method, 'POST');
  assert.equal(request.init.headers.Authorization, 'Bearer session-token');
  assert.equal(capture.status, 'succeeded');
});

test('vision API deletes a capture through the authenticated device endpoint', async () => {
  const { createAnbanClient } = await import('./api/client.js');
  let request;
  const client = createAnbanClient({
    baseURL: 'http://anban.local',
    token: 'session-token',
    isBound: true,
    fetchImpl: async (url, init) => {
      request = { url, init };
      return new Response(null, { status: 204 });
    },
  });

  await client.deleteVisionCapture('cap/delete', { deviceId: 'dev-001' });

  assert.equal(request.url, 'http://anban.local/api/vision/captures/cap%2Fdelete?deviceId=dev-001');
  assert.equal(request.init.method, 'DELETE');
  assert.equal(request.init.headers.Authorization, 'Bearer session-token');
});

test('vision history keeps retained originals and groups newest captures by day', async () => {
  const { groupVisionCapturesByDate } = await import('./integration-core.js');
  const groups = groupVisionCapturesByDate([
    { captureId: 'expired', status: 'expired', capturedAt: '2026-06-17T09:00:00Z' },
    { captureId: 'yesterday', imageUrl: '/yesterday', capturedAt: '2026-06-19T10:00:00Z' },
    { captureId: 'today-old', imageUrl: '/today-old', capturedAt: '2026-06-20T01:00:00Z' },
    { captureId: 'today-new', imageUrl: '/today-new', capturedAt: '2026-06-20T08:00:00Z' },
    { captureId: 'older', imageUrl: '/older', capturedAt: '2026-06-10T08:00:00Z' },
  ], new Date('2026-06-20T12:00:00Z'));

  assert.deepEqual(groups.map((group) => [group.label, group.items.map((item) => item.captureId)]), [
    ['今天', ['today-new', 'today-old']],
    ['昨天', ['yesterday']],
    ['2026/06/10', ['older']],
  ]);
});

test('home message summary exposes the latest delivery state without an inline message list', async () => {
  const { buildLatestMessageSummary } = await import('./integration-core.js');
  assert.deepEqual(buildLatestMessageSummary({ messages: [
    { messageId: 'old', status: 'played', queuedAt: '2026-06-20T08:00:00Z' },
    { messageId: 'new', status: 'pending', queuedAt: '2026-06-20T09:00:00Z' },
  ] }), { label: '最近留言待播报', tone: 'pending' });
  assert.deepEqual(buildLatestMessageSummary({ messages: [] }), { label: '暂无留言', tone: 'muted' });
});

test('vision capture view presents a successful observation with image and presence', async () => {
  const { buildVisionCaptureView } = await import('./integration-core.js');

  assert.deepEqual(buildVisionCaptureView({
    status: 'succeeded',
    imageUrl: '/api/vision/captures/cap_1/image',
    capturedAt: '2026-06-18T15:30:00+08:00',
    analysis: {
      summary: '老人正在沙发上休息，神态平静。',
      presence: 'someone',
      concerns: ['地面有水杯'],
    },
  }), {
    statusLabel: '已完成',
    statusTone: 'success',
    summary: '老人正在沙发上休息，神态平静。',
    presenceLabel: '看到老人',
    concerns: ['地面有水杯'],
    capturedAtLabel: '2026/06/18 15:30',
    showImage: true,
    action: null,
  });
});

test('vision look progress describes stable loading stages', async () => {
  const { buildVisionLookProgress } = await import('./integration-core.js');

  assert.deepEqual(buildVisionLookProgress('connecting'), {
    statusText: '正在连接设备',
    buttonText: '连接中',
    disabled: true,
  });
  assert.deepEqual(buildVisionLookProgress('capturing'), {
    statusText: '设备正在拍摄',
    buttonText: '拍摄中',
    disabled: true,
  });
  assert.deepEqual(buildVisionLookProgress('analyzing'), {
    statusText: '正在分析画面',
    buttonText: '分析中',
    disabled: true,
  });
  assert.deepEqual(buildVisionLookProgress('idle'), {
    statusText: '看看老人在不在',
    buttonText: '看一眼',
    disabled: false,
  });
});

test('vision capture view keeps a partial image visible and offers reanalysis', async () => {
  const { buildVisionCaptureView } = await import('./integration-core.js');

  const view = buildVisionCaptureView({
    status: 'partial',
    imageUrl: '/api/vision/captures/cap_partial/image',
    failureMessage: '图片已保存，但画面分析暂时失败',
    analysis: { presence: 'unknown', concerns: [] },
  });

  assert.equal(view.statusLabel, '部分成功');
  assert.equal(view.statusTone, 'warning');
  assert.equal(view.summary, '图片已保存，但画面分析暂时失败');
  assert.equal(view.showImage, true);
  assert.deepEqual(view.action, { kind: 'reanalyze', label: '重新分析' });
});

test('vision capture view turns a failed capture into an actionable retry', async () => {
  const { buildVisionCaptureView } = await import('./integration-core.js');

  const view = buildVisionCaptureView({
    status: 'failed',
    failureMessage: '设备当前离线，请确认设备已联网',
    analysis: { presence: 'unknown', concerns: [] },
  });

  assert.equal(view.statusLabel, '拍摄失败');
  assert.equal(view.statusTone, 'danger');
  assert.equal(view.summary, '设备当前离线，请确认设备已联网');
  assert.equal(view.showImage, false);
  assert.deepEqual(view.action, { kind: 'retry', label: '重试' });
});

test('vision capture view explains that an expired image is no longer available', async () => {
  const { buildVisionCaptureView } = await import('./integration-core.js');

  const view = buildVisionCaptureView({
    status: 'expired',
    analysis: { summary: '老人此前在客厅休息', presence: 'someone', concerns: [] },
  });

  assert.equal(view.statusLabel, '已过期');
  assert.equal(view.statusTone, 'muted');
  assert.equal(view.summary, '图片已按保留策略清理');
  assert.equal(view.showImage, false);
  assert.equal(view.action, null);
});
