import { ApiError, createAnbanClient } from './api/client.js';
import { formatApiErrorNotice } from './api-error-notice.js';
import { startMessageStatusPolling, stopMessageStatusPolling } from './message-status-polling.js';
import { normalizeMessageDraft } from './message-input.js';
import { formatMessageSendResult } from './message-result.js';
import { normalizeReminderDraft } from './reminder-input.js';
import { upsertMessage, upsertMessageFromSendError } from './message-state.js';
import { writeProfileFormFields } from './profile-form.js';
import { startReminderStatusPolling, stopReminderStatusPolling } from './reminder-status-polling.js';
import { startStatusPolling, stopStatusPolling } from './status-polling.js';
import { buildStatusSnapshotForDisplay, formatStatusDetail, messageStatusLabel } from './status-summary.js';
import { formatGreetingTriggerResult } from './greeting-result.js';
import { formatVisionPresenceResult } from './vision-presence-result.js';

const VISION_CAPTURE_TOOL = 'self.camera.take_photo';

const state = {
  accessCode: localStorage.getItem('anban.accessCode') || 'demo',
  deviceId: localStorage.getItem('anban.deviceId') || 'dev-001',
  apiBaseURL: localStorage.getItem('anban.apiBaseURL') || 'http://localhost:8090',
  messages: [],
  reminders: [],
  history: [],
  statusSnapshot: null,
  statusPoller: null,
  messageStatusPoller: null,
  reminderStatusPoller: null,
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
  historyList: document.querySelector('#historyList'),
  greetingButton: document.querySelector('#greetingButton'),
  visionButton: document.querySelector('#visionButton'),
  visionResult: document.querySelector('#visionResult'),
  visionPresenceButton: document.querySelector('#visionPresenceButton'),
  visionPresenceResult: document.querySelector('#visionPresenceResult'),
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
  reminderCategory: document.querySelector('#reminderCategory'),
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
  renderHistory();
}

els.connectForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  state.accessCode = els.accessCode.value.trim();
  state.deviceId = els.deviceId.value.trim();
  state.apiBaseURL = els.apiBaseURL.value.trim();
  if (!state.apiBaseURL || !state.accessCode || !state.deviceId) {
    stopConnectionPolling();
    clearConnectionData();
    renderStatus('未连接', '请填写后端地址、访问码和设备 ID');
    showNotice('后端地址、访问码和设备 ID 必填');
    return;
  }
  localStorage.setItem('anban.accessCode', state.accessCode);
  localStorage.setItem('anban.deviceId', state.deviceId);
  localStorage.setItem('anban.apiBaseURL', state.apiBaseURL);
  if (!await refreshMessages()) {
    return;
  }
  restartStatusPolling();
  restartMessageStatusPolling();
  restartReminderStatusPolling();
});

els.messageForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const draft = normalizeMessageDraft(els.messageText.value);
  if (!draft.text) {
    showNotice('留言不能为空');
    return;
  }

  try {
    const message = await client().sendMessage({
      deviceId: state.deviceId,
      text: draft.text,
      fromName: els.fromName.value.trim(),
    });
    const result = formatMessageSendResult(message, { draftNotice: draft.notice });
    els.messageText.value = '';
    state.messages = upsertMessage(state.messages, message);
    renderMessages();
    renderCurrentBackendStatus();
    showNotice(result.notice);
  } catch (error) {
    if (error instanceof ApiError && error.payload?.message) {
      state.messages = upsertMessageFromSendError(state.messages, error);
      renderMessages();
      renderCurrentBackendStatus();
    }
    handleApiError(error, '留言发送失败');
  }
});

els.greetingButton.addEventListener('click', async () => {
  try {
    const greeting = await client().triggerGreeting({ deviceId: state.deviceId, tonePreset: 'warm' });
    const result = formatGreetingTriggerResult(greeting);
    renderStatus(result.label, result.detail);
    showNotice(result.notice);
  } catch (error) {
    handleApiError(error, '问候接口暂未接入');
  }
});

els.visionButton.addEventListener('click', async () => {
  els.visionButton.disabled = true;
  try {
    const capture = await client().captureVision({
      deviceId: state.deviceId,
      tool: VISION_CAPTURE_TOOL,
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

els.visionPresenceButton.addEventListener('click', async () => {
  els.visionPresenceButton.disabled = true;
  try {
    const result = await client().checkVisionPresence({
      deviceId: state.deviceId,
      tool: VISION_CAPTURE_TOOL,
      args: { quality: 'low' },
    });
    const rendered = formatVisionPresenceResult(result);
    renderVisionPresence(rendered);
    renderStatus(rendered.label, rendered.detail);
    showNotice(rendered.notice);
  } catch (error) {
    handleApiError(error, '视觉触发失败');
  } finally {
    els.visionPresenceButton.disabled = false;
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
  const draft = normalizeReminderDraft({
    content: els.reminderContent.value,
    scheduledAtLocal: els.reminderTime.value,
  });
  if (!draft.valid) {
    showNotice(draft.notice);
    return;
  }
  try {
    const reminder = await client().createReminder({
      deviceId: state.deviceId,
      scheduledAt: draft.scheduledAt,
      content: draft.content,
      category: els.reminderCategory.value,
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

  const action = button.dataset.reminderAction || 'cancel';
  const completeReminder = action === 'complete';
  button.disabled = true;
  try {
    const reminder = completeReminder
      ? await client().ackReminder(button.dataset.reminderId, { ackKind: 'voice' })
      : await client().deleteReminder(button.dataset.reminderId);
    state.reminders = state.reminders.map((item) => (
      String(item.reminderId) === String(reminder.reminderId) ? reminder : item
    ));
    renderReminders();
    showNotice(completeReminder ? `提醒已完成：${reminder.content}` : `提醒已撤销：${reminder.content}`);
  } catch (error) {
    button.disabled = false;
    handleApiError(error, completeReminder ? '提醒确认失败' : '提醒撤销失败');
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
    writeProfileForm(profile);
    showNotice('画像已同步');
  } catch (error) {
    if (error instanceof ApiError && error.payload?.profile) {
      renderProfile(error.payload.profile);
      writeProfileForm(error.payload.profile);
    }
    handleApiError(error, '画像同步失败');
  }
});

async function refreshMessages() {
  try {
    const snapshot = await client().getStatus({ deviceId: state.deviceId });
    updateStatusSnapshot(snapshot);
    await refreshHistory();

    const payload = await client().listMessages({ deviceId: state.deviceId });
    state.messages = payload.messages || [];
    renderMessages();
    renderCurrentBackendStatus();
    await refreshReminders();
    await refreshGreetingSchedule();
    await refreshProfile();
    showNotice('已连接后端');
    return true;
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      renderStatus('骨架模式', '后端业务域仍是占位');
      showNotice('后端仍在占位模式');
      return false;
    }
    handleApiError(error, '暂时无法连接后端');
    return false;
  }
}

async function refreshBackendStatus() {
  try {
    const snapshot = await client().getStatus({ deviceId: state.deviceId });
    updateStatusSnapshot(snapshot);
    await refreshHistory();
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      return;
    }
    renderStatus('离线', '状态刷新失败');
  }
}

function restartStatusPolling() {
  stopStatusPolling(state.statusPoller);
  state.statusPoller = startStatusPolling(refreshBackendStatus);
}

async function refreshBackendMessages() {
  try {
    const payload = await client().listMessages({ deviceId: state.deviceId });
    state.messages = payload.messages || [];
    renderMessages();
    renderCurrentBackendStatus();
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      return;
    }
    showNotice('留言状态刷新失败');
  }
}

function restartMessageStatusPolling() {
  stopMessageStatusPolling(state.messageStatusPoller);
  state.messageStatusPoller = startMessageStatusPolling(refreshBackendMessages);
}

async function refreshBackendReminders() {
  try {
    await refreshReminders();
  } catch (error) {
    showNotice('提醒状态刷新失败');
  }
}

function restartReminderStatusPolling() {
  stopReminderStatusPolling(state.reminderStatusPoller);
  state.reminderStatusPoller = startReminderStatusPolling(refreshBackendReminders);
}

function stopConnectionPolling() {
  stopStatusPolling(state.statusPoller);
  stopMessageStatusPolling(state.messageStatusPoller);
  stopReminderStatusPolling(state.reminderStatusPoller);
  state.statusPoller = null;
  state.messageStatusPoller = null;
  state.reminderStatusPoller = null;
}

function clearConnectionData() {
  state.messages = [];
  state.reminders = [];
  state.history = [];
  state.statusSnapshot = null;
  renderMessages();
  renderReminders();
  renderHistory();
  clearProfile();
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

async function refreshHistory() {
  try {
    const payload = await client().getHistory({ deviceId: state.deviceId, limit: 50 });
    state.history = payload.messages || [];
    renderHistory();
  } catch (error) {
    if (error instanceof ApiError && (error.status === 501 || error.status === 502)) {
      state.history = [];
      renderHistory();
      return;
    }
    throw error;
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
      clearProfile();
      return;
    }
    throw error;
  }
}

function clearProfile() {
  els.profileSummary.textContent = '暂无画像';
  writeProfileForm({ fields: {} });
}

function renderBackendStatus(snapshot) {
  const label = snapshot.online ? '在线' : '离线';
  const detail = formatStatusDetail(snapshot, { formatDateTime });
  renderStatus(label, detail);
}

function updateStatusSnapshot(snapshot) {
  state.statusSnapshot = snapshot;
  renderCurrentBackendStatus();
}

function renderCurrentBackendStatus() {
  const snapshot = buildStatusSnapshotForDisplay(state.statusSnapshot, state.messages);
  if (!snapshot) {
    return;
  }
  renderBackendStatus(snapshot);
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
  writeProfileFormFields(els.profileForm, profile.fields || {});
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

function renderVisionPresence(result) {
  els.visionPresenceResult.textContent = result.output;
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
      let action = '';
      if (reminder.status === 'scheduled') {
        action = `<button type="button" data-reminder-id="${escapeHtml(reminder.reminderId)}" data-reminder-action="cancel">撤销</button>`;
      }
      if (reminder.status === 'played') {
        action = `<button type="button" data-reminder-id="${escapeHtml(reminder.reminderId)}" data-reminder-action="complete">完成</button>`;
      }
      return `<li><span>${escapeHtml(reminder.content)}</span><strong>${status}</strong><time>${at}</time>${action}</li>`;
    })
    .join('');
}

function renderHistory() {
  if (!state.history.length) {
    els.historyList.innerHTML = '<li class="empty">暂无对话记录</li>';
    return;
  }

  els.historyList.innerHTML = state.history
    .map((message) => {
      const role = historyRoleLabel(message.role || message.Role);
      const text = message.text || message.Text || '';
      const at = formatDateTime(message.at || message.At);
      return `<li><strong>${role}</strong><span>${escapeHtml(text)}</span><time>${at}</time></li>`;
    })
    .join('');
}

function historyRoleLabel(role) {
  if (role === 'user') return '老人';
  if (role === 'assistant') return '安伴';
  return '对话';
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

function statusLabel(status) {
  return messageStatusLabel(status);
}

function reminderStatusLabel(status) {
  if (status === 'played') return '已播报';
  if (status === 'completed') return '已完成';
  if (status === 'unanswered') return '未应答';
  if (status === 'failed') return '失败';
  if (status === 'canceled') return '已撤销';
  if (status === 'skipped') return '已跳过';
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
  showNotice(formatApiErrorNotice(error, fallback));
}

boot();
