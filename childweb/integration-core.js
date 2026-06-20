export function formatLoginError(error) {
  if (error?.status === 401) return '访问码错误，请重新输入';
  if (error?.status && error?.message) return `${error.message}（${error.status}）`;
  return '无法连接安伴服务，请检查后端地址';
}

export function formatRelativeTime(value, now = new Date()) {
  const at = new Date(value);
  if (Number.isNaN(at.getTime())) return '暂无互动记录';
  const diffMs = Math.max(0, now.getTime() - at.getTime());
  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 1) return '刚刚';
  if (minutes < 60) return `${minutes}分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}小时前`;
  return `${Math.floor(hours / 24)}天前`;
}

export function buildHomeStatus(status = {}, now = new Date()) {
  const online = status.online === true;
  const relative = status.lastInteractionAt
    ? formatRelativeTime(status.lastInteractionAt, now)
    : '暂无互动记录';

  return {
    online,
    label: online ? '在线' : '离线',
    title: online ? '设备在线' : '设备暂时离线',
    description: status.lastInteractionAt ? `最近互动 ${relative}` : relative,
    updatedAt: '刚刚更新',
  };
}

export function formatGreetingTriggerResult(greeting = {}) {
  const text = String(greeting.text || '').trim();
  if (greeting.status === 'pending') {
    return {
      label: '在线',
      detail: '主动问候已排队',
      notice: withOptionalText('问候已排队', text),
    };
  }

  return {
    label: '在线',
    detail: '刚刚触发一次主动问候',
    notice: withOptionalText('问候已触发', text),
  };
}

export function formatVisionPresenceResult(result = {}) {
  const observation = result.observation || {};
  const greeting = observation.greeting || {};
  const text = String(greeting.text || '').trim();

  if (observation.presence === 'someone' && observation.triggeredGreeting) {
    if (greeting.status === 'pending') {
      return {
        detail: '看见有人，问候已排队',
        notice: withOptionalText('视觉触发问候已排队', text),
      };
    }
    return {
      detail: '看见有人，已触发问候',
      notice: withOptionalText('视觉触发问候已触发', text),
    };
  }

  if (observation.presence === 'someone') {
    return {
      detail: '看见有人，暂未触发问候',
      notice: '看一眼完成：看见有人',
    };
  }

  if (observation.presence === 'no_one') {
    return {
      detail: '暂时没有看到老人',
      notice: '看一眼完成：暂时没有看到老人',
    };
  }

  return {
    detail: '看一眼结果暂不可用',
    notice: '看一眼完成：结果暂不可用',
  };
}

export function buildLatestMessageSummary(payload = {}) {
  const messages = Array.isArray(payload.messages) ? payload.messages : [];
  const latest = messages.slice().sort((left, right) => {
    const leftAt = new Date(left?.queuedAt || left?.playedAt || 0).getTime();
    const rightAt = new Date(right?.queuedAt || right?.playedAt || 0).getTime();
    return (Number.isNaN(rightAt) ? 0 : rightAt) - (Number.isNaN(leftAt) ? 0 : leftAt);
  })[0];
  if (!latest) return { label: '暂无留言', tone: 'muted' };
  if (latest.status === 'played') return { label: '最近留言已播报', tone: 'success' };
  if (latest.status === 'failed') return { label: '最近留言发送失败', tone: 'danger' };
  return { label: '最近留言待播报', tone: 'pending' };
}

export function groupVisionCapturesByDate(captures = [], now = new Date()) {
  const valid = (Array.isArray(captures) ? captures : [])
    .filter((capture) => capture && String(capture.imageUrl || '').trim())
    .slice()
    .sort((left, right) => captureTimestamp(right) - captureTimestamp(left));
  const todayKey = localDateKey(now);
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  const yesterdayKey = localDateKey(yesterday);
  const groups = [];
  const byKey = new Map();

  valid.forEach((capture) => {
    const capturedAt = new Date(capture.capturedAt || 0);
    const key = Number.isNaN(capturedAt.getTime()) ? 'unknown' : localDateKey(capturedAt);
    if (!byKey.has(key)) {
      const group = {
        key,
        label: key === todayKey ? '今天' : key === yesterdayKey ? '昨天' : key === 'unknown' ? '日期未知' : key.replaceAll('-', '/'),
        items: [],
      };
      byKey.set(key, group);
      groups.push(group);
    }
    byKey.get(key).items.push(capture);
  });
  return groups;
}

function captureTimestamp(capture) {
  const value = new Date(capture?.capturedAt || 0).getTime();
  return Number.isNaN(value) ? 0 : value;
}

function localDateKey(value) {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return 'unknown';
  return [date.getFullYear(), String(date.getMonth() + 1).padStart(2, '0'), String(date.getDate()).padStart(2, '0')].join('-');
}

export function buildVisionCaptureView(capture = {}) {
  const analysis = capture.analysis || {};
  const status = String(capture.status || 'pending').trim();
  const statusMap = {
    succeeded: { label: '已完成', tone: 'success' },
    partial: { label: '部分成功', tone: 'warning' },
    failed: { label: '拍摄失败', tone: 'danger' },
    expired: { label: '已过期', tone: 'muted' },
    pending: { label: '进行中', tone: 'pending' },
  };
  const statusView = statusMap[status] || statusMap.pending;
  const isPartial = status === 'partial';
  const isFailed = status === 'failed';
  const isExpired = status === 'expired';
  return {
    statusLabel: statusView.label,
    statusTone: statusView.tone,
    summary: isExpired ? '图片已按保留策略清理' : String(analysis.summary || capture.failureMessage || '暂无观察结果').trim(),
    presenceLabel: visionPresenceLabel(analysis.presence),
    concerns: Array.isArray(analysis.concerns) ? analysis.concerns : [],
    capturedAtLabel: formatVisionCapturedAt(capture.capturedAt),
    showImage: (status === 'succeeded' || isPartial) && Boolean(capture.imageUrl),
    action: isPartial ? { kind: 'reanalyze', label: '重新分析' } : (isFailed ? { kind: 'retry', label: '重试' } : null),
  };
}

export function buildVisionLookProgress(stage = 'idle') {
  const stages = {
    connecting: { statusText: '正在连接设备', buttonText: '连接中', disabled: true },
    capturing: { statusText: '设备正在拍摄', buttonText: '拍摄中', disabled: true },
    analyzing: { statusText: '正在分析画面', buttonText: '分析中', disabled: true },
    idle: { statusText: '看看老人在不在', buttonText: '看一眼', disabled: false },
  };
  return stages[stage] || stages.idle;
}

function visionPresenceLabel(presence) {
  if (presence === 'someone') return '看到老人';
  if (presence === 'no_one') return '未看到老人';
  return '暂未确认';
}

function formatVisionCapturedAt(value) {
  const at = new Date(value || '');
  if (Number.isNaN(at.getTime())) return '拍摄时间未知';
  const parts = new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(at);
  const pick = (type) => parts.find((part) => part.type === type)?.value || '';
  return `${pick('year')}/${pick('month')}/${pick('day')} ${pick('hour')}:${pick('minute')}`;
}

const DEFAULT_GREETING_SLOTS = Object.freeze([
  Object.freeze({ label: 'morning', time: '08:00', enabled: true, tonePreset: 'warm' }),
  Object.freeze({ label: 'noon', time: '12:30', enabled: true, tonePreset: 'warm' }),
  Object.freeze({ label: 'evening', time: '18:00', enabled: true, tonePreset: 'warm' }),
]);

export function normalizeGreetingSlots(slots = []) {
  const byLabel = new Map((Array.isArray(slots) ? slots : [])
    .map((slot) => [String(slot?.label || '').trim(), slot]));

  return DEFAULT_GREETING_SLOTS.map((fallback) => {
    const slot = byLabel.get(fallback.label) || {};
    const time = normalizeGreetingTime(slot.time) || fallback.time;
    const tonePreset = slot.tonePreset === 'casual' ? 'casual' : fallback.tonePreset;
    return {
      label: fallback.label,
      time,
      enabled: typeof slot.enabled === 'boolean' ? slot.enabled : fallback.enabled,
      tonePreset,
    };
  });
}

function withOptionalText(prefix, text) {
  return text ? `${prefix}：${text}` : prefix;
}

function normalizeGreetingTime(value) {
  const text = String(value || '').trim();
  if (!/^\d{2}:\d{2}$/.test(text)) return '';
  const [hour, minute] = text.split(':').map(Number);
  if (hour > 23 || minute > 59) return '';
  return text;
}

export function normalizeHistoryMessages(payload = {}, limit = 10) {
  const messages = Array.isArray(payload.messages) ? payload.messages : [];
  return messages
    .filter((item) => item && (item.role === 'user' || item.role === 'assistant') && String(item.text || '').trim())
    .map((item) => ({
      role: item.role,
      text: String(item.text).trim(),
      at: item.at || '',
    }))
    .slice(-Math.max(0, limit))
    .reverse();
}

function messageStatusLabel(status) {
  if (status === 'played') return '已播报';
  if (status === 'failed') return '发送失败';
  if (status === 'cancelled') return '已取消';
  return '待播报';
}

export function buildConversationBubbles({ history = {}, messages = {} } = {}) {
  const historyBubbles = (Array.isArray(history.messages) ? history.messages : [])
    .filter((item) => item && String(item.text || '').trim())
    .map((item, index) => ({
      id: `history-${index}`,
      side: 'left',
      role: item.role || 'user',
      text: String(item.text).trim(),
      at: item.at || '',
      status: '',
    }));
  const childBubbles = (Array.isArray(messages.messages) ? messages.messages : [])
    .filter((item) => item && String(item.text || '').trim())
    .map((item, index) => ({
      id: item.messageId || `message-${index}`,
      side: 'right',
      role: 'child',
      text: String(item.text).trim(),
      at: item.playedAt || item.queuedAt || '',
      status: messageStatusLabel(item.status),
    }));

  return [...historyBubbles, ...childBubbles].sort((left, right) => {
    const leftAt = new Date(left.at).getTime();
    const rightAt = new Date(right.at).getTime();
    return (Number.isNaN(leftAt) ? 0 : leftAt) - (Number.isNaN(rightAt) ? 0 : rightAt);
  });
}

export function nextOccurrenceUTC(hour, minute, now = new Date()) {
  const scheduled = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
    Number(hour),
    Number(minute),
    0,
    0,
  );
  if (scheduled <= now) scheduled.setDate(scheduled.getDate() + 1);
  return scheduled.toISOString();
}

export function buildReminderScheduleOptions(frequency = '仅一次', customDates = []) {
  const label = String(frequency || '').trim();
  const dates = normalizeReminderCustomDates(customDates);

  if (label === '每天') return { recurrence: 'daily', customDates: [] };
  if (label === '工作日') return { recurrence: 'weekdays', customDates: [] };
  if (label === '周末') return { recurrence: 'weekends', customDates: [] };
  if (dates.length > 0) return { recurrence: 'custom-dates', customDates: dates };
  return { recurrence: 'none', customDates: [] };
}

function normalizeReminderCustomDates(values = []) {
  return [...new Set((Array.isArray(values) ? values : [])
    .map((value) => String(value || '').trim())
    .filter((value) => /^\d{4}-\d{2}-\d{2}$/.test(value)))]
    .sort();
}

function cleanList(values) {
  return (Array.isArray(values) ? values : [])
    .map((value) => String(value || '').trim())
    .filter(Boolean);
}

export function mapStitchProfileToFields(profile = {}) {
  const habits = (Array.isArray(profile.habits) ? profile.habits : [])
    .map((item) => String(item?.text || '').trim())
    .filter(Boolean);
  const communicationDos = cleanList(profile.communicationDos)
    .map((item) => `沟通建议：${item}`);
  const healthItems = (Array.isArray(profile.health) ? profile.health : [])
    .map((item) => {
      const name = String(item?.name || '').trim();
      const detail = String(item?.detail || '').trim();
      return name || detail ? `${name || '健康事项'}：${detail}` : '';
    })
    .filter(Boolean);
  const portrait = String(profile.aiPortrait || '').trim();
  const portraitMode = profile.aiPortraitMode === 'manual' ? 'manual' : 'auto';
  const name = String(profile.name || '').trim();

  return {
    name,
    nickname: name,
    hobbies: cleanList(profile.hobbies),
    schedule: [...habits, ...communicationDos].join('\n'),
    aiPortrait: portrait,
    aiPortraitMode: portraitMode,
    health: healthItems.join('\n'),
    taboos: cleanList(profile.communicationDonts),
  };
}

export function mapFieldsToStitchProfile(fields = {}, local = {}) {
  const name = fields.name ?? fields.Name ?? fields.nickname ?? fields.Nickname ?? '';
  const hobbies = fields.hobbies ?? fields.Hobbies ?? [];
  const schedule = String(fields.schedule ?? fields.Schedule ?? '');
  const healthText = String(fields.health ?? fields.Health ?? '');
  const taboos = fields.taboos ?? fields.Taboos ?? [];
  const scheduleLines = schedule.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const healthLines = healthText.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const explicitPortrait = String(fields.aiPortrait ?? fields.AIPortrait ?? '').trim();
  const legacyPortrait = (healthLines.find((line) => line.startsWith('AI画像：') || line.startsWith('AI认知画像：')) || '')
    .replace(/^AI(?:认知)?画像：/, '');
  const portraitMode = String(fields.aiPortraitMode ?? fields.AIPortraitMode ?? '').trim() === 'manual' ? 'manual' : 'auto';

  return {
    name: String(name || '').trim(),
    age: Number(local.age) || 0,
    livingSituation: String(local.livingSituation || '').trim(),
    occupation: String(local.occupation || '').trim(),
    aiPortrait: explicitPortrait || legacyPortrait,
    aiPortraitMode: portraitMode,
    hobbies: cleanList(hobbies),
    habits: scheduleLines
      .filter((line) => !line.startsWith('沟通建议：'))
      .map((text) => ({ icon: 'wb_twilight', text })),
    health: healthLines
      .filter((line) => !line.startsWith('AI画像：') && !line.startsWith('AI认知画像：'))
      .map((line) => {
        const separator = line.indexOf('：');
        return separator >= 0
          ? { name: line.slice(0, separator), detail: line.slice(separator + 1) }
          : { name: '健康事项', detail: line };
      }),
    communicationDos: scheduleLines
      .filter((line) => line.startsWith('沟通建议：'))
      .map((line) => line.replace(/^沟通建议：/, '')),
    communicationDonts: cleanList(taboos),
  };
}
