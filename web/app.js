import { ApiError, createAnbanClient } from './api/client.js';

const state = {
  accessCode: localStorage.getItem('anban.accessCode') || 'demo',
  deviceId: localStorage.getItem('anban.deviceId') || 'dev-001',
  apiBaseURL: localStorage.getItem('anban.apiBaseURL') || '',
  messages: [],
  reminders: [],
};

const els = {
  accessCode: document.querySelector('#accessCode'),
  deviceId: document.querySelector('#deviceId'),
  apiBaseURL: document.querySelector('#apiBaseURL'),
  connectForm: document.querySelector('#connectForm'),
  statusPanel: document.querySelector('#statusPanel'),
  statusText: document.querySelector('#statusText'),
  lastInteraction: document.querySelector('#lastInteraction'),
  messageForm: document.querySelector('#messageForm'),
  messageText: document.querySelector('#messageText'),
  fromName: document.querySelector('#fromName'),
  messageList: document.querySelector('#messageList'),
  greetingButton: document.querySelector('#greetingButton'),
  visionButton: document.querySelector('#visionButton'),
  visionResult: document.querySelector('#visionResult'),
  greetingScheduleForm: document.querySelector('#greetingScheduleForm'),
  morningGreetingTime: document.querySelector('#morningGreetingTime'),
  morningGreetingEnabled: document.querySelector('#morningGreetingEnabled'),
  morningGreetingTone: document.querySelector('#morningGreetingTone'),
  noonGreetingTime: document.querySelector('#noonGreetingTime'),
  noonGreetingEnabled: document.querySelector('#noonGreetingEnabled'),
  noonGreetingTone: document.querySelector('#noonGreetingTone'),
  eveningGreetingTime: document.querySelector('#eveningGreetingTime'),
  eveningGreetingEnabled: document.querySelector('#eveningGreetingEnabled'),
  eveningGreetingTone: document.querySelector('#eveningGreetingTone'),
  reminderForm: document.querySelector('#reminderForm'),
  reminderContent: document.querySelector('#reminderContent'),
  reminderTime: document.querySelector('#reminderTime'),
  reminderList: document.querySelector('#reminderList'),
  profileForm: document.querySelector('#profileForm'),
  profileSummary: document.querySelector('#profileSummary'),
  notice: document.querySelector('#notice'),
};

function client() {
  return createAnbanClient({
    baseURL: state.apiBaseURL,
    accessCode: state.accessCode,
  });
}

function boot() {
  els.accessCode.value = state.accessCode;
  els.deviceId.value = state.deviceId;
  els.apiBaseURL.value = state.apiBaseURL;
  renderStatus('未连接', '等待访问码');
  renderMessages();
  renderReminders();
}

els.connectForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  state.accessCode = els.accessCode.value.trim();
  state.deviceId = els.deviceId.value.trim();
  state.apiBaseURL = els.apiBaseURL.value.trim();
  localStorage.setItem('anban.accessCode', state.accessCode);
  localStorage.setItem('anban.deviceId', state.deviceId);
  localStorage.setItem('anban.apiBaseURL', state.apiBaseURL);
  await refreshMessages();
});

els.messageForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const text = els.messageText.value.trim();
  if (!text) {
    showNotice('留言不能为空');
    return;
  }

  try {
    const message = await client().sendMessage({
      deviceId: state.deviceId,
      text,
      fromName: els.fromName.value.trim(),
    });
    els.messageText.value = '';
    state.messages = [message, ...state.messages.filter((item) => item.messageId !== message.messageId)];
    renderMessages();
    renderStatus('在线', '刚刚完成一次留言播报');
    showNotice('留言已发送');
  } catch (error) {
    handleApiError(error, '留言发送失败');
  }
});

els.greetingButton.addEventListener('click', async () => {
  try {
    const greeting = await client().triggerGreeting({ deviceId: state.deviceId, tonePreset: 'warm' });
    renderStatus('在线', '刚刚触发一次主动问候');
    showNotice(`问候已触发：${greeting.text}`);
  } catch (error) {
    handleApiError(error, '问候接口暂未接入');
  }
});

els.visionButton.addEventListener('click', async () => {
  els.visionButton.disabled = true;
  try {
    const capture = await client().captureVision({
      deviceId: state.deviceId,
      tool: 'camera.capture',
      args: { quality: 'low' },
    });
    renderVisionCapture(capture);
    renderStatus('在线', '刚刚完成一次看一眼');
    showNotice('看一眼结果已返回');
  } catch (error) {
    handleApiError(error, '看一眼失败');
  } finally {
    els.visionButton.disabled = false;
  }
});

els.greetingScheduleForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  try {
    const schedule = await client().updateGreetingSchedule({
      deviceId: state.deviceId,
      slots: readGreetingSchedule(),
    });
    renderGreetingSchedule(schedule);
    showNotice('问候时段已保存');
  } catch (error) {
    handleApiError(error, '问候时段保存失败');
  }
});

els.reminderForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const content = els.reminderContent.value.trim();
  const at = els.reminderTime.value;
  if (!content || !at) {
    showNotice('提醒内容和时间都要填写');
    return;
  }
  try {
    const reminder = await client().createReminder({
      deviceId: state.deviceId,
      scheduledAt: new Date(at).toISOString(),
      content,
      category: 'med',
    });
    state.reminders = [reminder, ...state.reminders.filter((item) => item.reminderId !== reminder.reminderId)];
    renderReminders();
    renderStatus('在线', `已排入提醒：${formatTime(reminder.scheduledAt)}`);
    showNotice(`提醒已创建：${reminder.content}`);
  } catch (error) {
    handleApiError(error, '提醒创建失败');
  }
});

els.reminderList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-reminder-id]');
  if (!button) return;

  button.disabled = true;
  try {
    const reminder = await client().deleteReminder(button.dataset.reminderId);
    state.reminders = state.reminders.map((item) => (
      String(item.reminderId) === String(reminder.reminderId) ? reminder : item
    ));
    renderReminders();
    showNotice(`提醒已撤销：${reminder.content}`);
  } catch (error) {
    button.disabled = false;
    handleApiError(error, '提醒撤销失败');
  }
});

els.profileForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const form = new FormData(els.profileForm);
  const fields = {
    name: readText(form, 'elderName'),
    nickname: readText(form, 'nickname'),
    children: readList(form, 'children'),
    grandchildren: readList(form, 'grandchildren'),
    hobbies: readList(form, 'hobby'),
    schedule: readText(form, 'schedule'),
    health: readText(form, 'health'),
    taboos: readList(form, 'taboos'),
  };

  try {
    const profile = await client().updateProfile({
      deviceId: state.deviceId,
      fields,
    });
    renderProfile(profile);
    showNotice('画像已同步');
  } catch (error) {
    handleApiError(error, '画像同步失败');
  }
});

async function refreshMessages() {
  try {
    const snapshot = await client().getStatus({ deviceId: state.deviceId });
    renderBackendStatus(snapshot);

    const payload = await client().listMessages({ deviceId: state.deviceId });
    state.messages = payload.messages || [];
    renderMessages();
    await refreshReminders();
    await refreshGreetingSchedule();
    await refreshProfile();
    showNotice('已连接后端');
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      renderStatus('骨架模式', '后端业务域仍是占位');
      showNotice('后端仍在占位模式');
      return;
    }
    handleApiError(error, '暂时无法连接后端');
  }
}

async function refreshReminders() {
  try {
    const payload = await client().listReminders({ deviceId: state.deviceId });
    state.reminders = payload.reminders || [];
    renderReminders();
  } catch (error) {
    if (!(error instanceof ApiError && error.status === 501)) {
      throw error;
    }
    state.reminders = [];
    renderReminders();
  }
}

async function refreshGreetingSchedule() {
  try {
    const schedule = await client().getGreetingSchedule({ deviceId: state.deviceId });
    renderGreetingSchedule(schedule);
  } catch (error) {
    if (!(error instanceof ApiError && error.status === 501)) {
      throw error;
    }
  }
}

async function refreshProfile() {
  try {
    const profile = await client().getProfile({ deviceId: state.deviceId });
    renderProfile(profile);
    writeProfileForm(profile);
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 501)) {
      return;
    }
    throw error;
  }
}

function renderBackendStatus(snapshot) {
  const label = snapshot.online ? '在线' : '离线';
  const at = snapshot.lastInteractionAt || snapshot.lastSeenAt;
  const detail = at ? `最近互动：${formatDateTime(at)}` : '暂无最近互动';
  renderStatus(label, detail);
}

function renderStatus(label, detail) {
  els.statusPanel.dataset.state = label;
  els.statusText.textContent = label;
  els.lastInteraction.textContent = detail;
}

function renderProfile(profile) {
  const fields = profile.fields || {};
  const name = fields.nickname || fields.name || '未命名';
  const hobbies = Array.isArray(fields.hobbies) ? fields.hobbies.join('、') : fields.hobbies || '暂无喜好';
  els.profileSummary.textContent = `${name} · 喜欢${hobbies}`;
}

function writeProfileForm(profile) {
  const fields = profile.fields || {};
  writeFormValue('elderName', fields.name);
  writeFormValue('nickname', fields.nickname);
  writeFormValue('children', fields.children);
  writeFormValue('grandchildren', fields.grandchildren);
  writeFormValue('hobby', fields.hobbies);
  writeFormValue('schedule', fields.schedule);
  writeFormValue('health', fields.health);
  writeFormValue('taboos', fields.taboos);
}

function renderGreetingSchedule(schedule) {
  const byLabel = new Map((schedule.slots || []).map((slot) => [slot.label, slot]));
  writeGreetingSlot('morning', byLabel.get('morning'));
  writeGreetingSlot('noon', byLabel.get('noon'));
  writeGreetingSlot('evening', byLabel.get('evening'));
}

function renderVisionCapture(capture) {
  const raw = capture && capture.raw !== undefined ? capture.raw : capture;
  const text = typeof raw === 'string' ? raw : JSON.stringify(raw);
  els.visionResult.textContent = `看一眼结果：${text || '暂无内容'}`;
}

function renderMessages() {
  if (!state.messages.length) {
    els.messageList.innerHTML = '<li class="empty">暂无留言</li>';
    return;
  }

  els.messageList.innerHTML = state.messages
    .map((message) => {
      const status = statusLabel(message.status);
      const at = formatTime(message.playedAt || message.queuedAt);
      return `<li><span>${escapeHtml(message.text)}</span><strong>${status}</strong><time>${at}</time></li>`;
    })
    .join('');
}

function renderReminders() {
  if (!state.reminders.length) {
    els.reminderList.innerHTML = '<li class="empty">暂无提醒</li>';
    return;
  }

  els.reminderList.innerHTML = state.reminders
    .map((reminder) => {
      const status = reminderStatusLabel(reminder.status);
      const at = formatDateTime(reminder.scheduledAt);
      const action = reminder.status === 'scheduled'
        ? `<button type="button" data-reminder-id="${escapeHtml(reminder.reminderId)}">撤销</button>`
        : '';
      return `<li><span>${escapeHtml(reminder.content)}</span><strong>${status}</strong><time>${at}</time>${action}</li>`;
    })
    .join('');
}

function readGreetingSchedule() {
  return [
    readGreetingSlot('morning'),
    readGreetingSlot('noon'),
    readGreetingSlot('evening'),
  ];
}

function readGreetingSlot(label) {
  return {
    label,
    time: els[`${label}GreetingTime`].value,
    enabled: els[`${label}GreetingEnabled`].checked,
    tonePreset: els[`${label}GreetingTone`].value,
  };
}

function writeGreetingSlot(label, slot) {
  if (!slot) return;
  els[`${label}GreetingTime`].value = slot.time || els[`${label}GreetingTime`].value;
  els[`${label}GreetingEnabled`].checked = Boolean(slot.enabled);
  els[`${label}GreetingTone`].value = slot.tonePreset || 'warm';
}

function writeFormValue(name, value) {
  const input = els.profileForm.elements.namedItem(name);
  if (!input || value === undefined || value === null) return;
  input.value = Array.isArray(value) ? value.join(', ') : value;
}

function statusLabel(status) {
  if (status === 'played') return '已播报';
  if (status === 'failed') return '失败';
  return '排队中';
}

function reminderStatusLabel(status) {
  if (status === 'played') return '已播报';
  if (status === 'completed') return '已完成';
  if (status === 'unanswered') return '未应答';
  if (status === 'failed') return '失败';
  if (status === 'canceled') return '已撤销';
  return '待提醒';
}

function formatTime(value) {
  if (!value) return '--';
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function formatDateTime(value) {
  if (!value) return '--';
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function readText(form, name) {
  return String(form.get(name) || '').trim();
}

function readList(form, name) {
  return readText(form, name)
    .split(/[,，、]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function escapeHtml(value) {
  return String(value || '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;',
  })[char]);
}

function showNotice(text) {
  els.notice.textContent = text;
}

function handleApiError(error, fallback) {
  const detail = error instanceof ApiError && error.status ? `${fallback}（${error.status}）` : fallback;
  showNotice(detail);
}

boot();
