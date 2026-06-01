import { ApiError, createAnbanClient } from './api/client.js';

const state = {
  accessCode: localStorage.getItem('anban.accessCode') || 'demo',
  deviceId: localStorage.getItem('anban.deviceId') || 'dev-001',
  apiBaseURL: localStorage.getItem('anban.apiBaseURL') || '',
  messages: [],
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
  reminderForm: document.querySelector('#reminderForm'),
  reminderContent: document.querySelector('#reminderContent'),
  reminderTime: document.querySelector('#reminderTime'),
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
    renderStatus('在线', `已排入提醒：${formatTime(reminder.scheduledAt)}`);
    showNotice(`提醒已创建：${reminder.content}`);
  } catch (error) {
    handleApiError(error, '提醒创建失败');
  }
});

els.profileForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const form = new FormData(els.profileForm);
  const name = form.get('elderName') || '王阿姨';
  const hobby = form.get('hobby') || '豫剧';
  els.profileSummary.textContent = `${name} · 喜欢${hobby}`;
  showNotice('画像草稿已更新');
});

async function refreshMessages() {
  try {
    const snapshot = await client().getStatus({ deviceId: state.deviceId });
    renderBackendStatus(snapshot);

    const payload = await client().listMessages({ deviceId: state.deviceId });
    state.messages = payload.messages || [];
    renderMessages();
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

function statusLabel(status) {
  if (status === 'played') return '已播报';
  if (status === 'failed') return '失败';
  return '排队中';
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

function escapeHtml(value) {
  return value.replace(/[&<>"']/g, (char) => ({
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
