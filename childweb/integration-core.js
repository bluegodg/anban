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
  const name = String(profile.name || '').trim();

  return {
    name,
    nickname: name,
    hobbies: cleanList(profile.hobbies),
    schedule: [...habits, ...communicationDos].join('\n'),
    health: [...(portrait ? [`AI画像：${portrait}`] : []), ...healthItems].join('\n'),
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

  return {
    name: String(name || '').trim(),
    age: Number(local.age) || 0,
    livingSituation: String(local.livingSituation || '').trim(),
    occupation: String(local.occupation || '').trim(),
    aiPortrait: (healthLines.find((line) => line.startsWith('AI画像：')) || '').replace(/^AI画像：/, ''),
    hobbies: cleanList(hobbies),
    habits: scheduleLines
      .filter((line) => !line.startsWith('沟通建议：'))
      .map((text) => ({ icon: 'wb_twilight', text })),
    health: healthLines
      .filter((line) => !line.startsWith('AI画像：'))
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
