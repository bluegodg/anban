export function formatHistoryEntry(message = {}, { formatDateTime = defaultFormatDateTime } = {}) {
  return {
    role: historyRoleLabel(message.role || message.Role),
    text: message.text || message.Text || '',
    time: historyTimeLabel(message.at || message.At, { formatDateTime }),
  };
}

export function historyRoleLabel(role) {
  if (role === 'user') return '老人';
  if (role === 'assistant') return '安伴';
  return '对话';
}

export function historyTimeLabel(value, { formatDateTime = defaultFormatDateTime } = {}) {
  if (!value) return '时间未知';
  const at = new Date(value);
  if (Number.isNaN(at.getTime())) return '时间未知';
  return formatDateTime(value);
}

function defaultFormatDateTime(value) {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}
