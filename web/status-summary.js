const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

export function formatStatusDetail(snapshot = {}, { formatInteractionTime, formatDateTime } = {}) {
  const at = snapshot.lastInteractionAt || snapshot.lastSeenAt;
  const formatter = formatInteractionTime || formatDateTime || formatRelativeInteractionTime;
  const parts = [at ? `最近互动：${formatter(at)}` : '暂无最近互动'];
  const latestMessage = Array.isArray(snapshot.messages) ? snapshot.messages[0] : null;

  if (latestMessage?.status) {
    parts.push(`最新留言：${messageStatusLabel(latestMessage.status)}`);
  }

  return parts.join(' · ');
}

export function formatRelativeInteractionTime(
  value,
  { now = new Date(), formatDateTime = defaultFormatDateTime } = {},
) {
  const elapsedMs = Math.max(0, new Date(now).getTime() - new Date(value).getTime());

  if (elapsedMs < MINUTE_MS) return '刚刚';
  if (elapsedMs < HOUR_MS) return `${Math.floor(elapsedMs / MINUTE_MS)} 分钟前`;
  if (elapsedMs < DAY_MS) return `${Math.floor(elapsedMs / HOUR_MS)} 小时前`;
  return formatDateTime(value);
}

export function buildStatusSnapshotForDisplay(statusSnapshot, messages = []) {
  const localMessages = Array.isArray(messages) ? messages : [];
  if (statusSnapshot) {
    return {
      ...statusSnapshot,
      messages: localMessages.length ? localMessages : statusSnapshot.messages,
    };
  }
  if (!localMessages.length) {
    return null;
  }

  return {
    online: localMessages[0].status !== 'failed',
    messages: localMessages,
  };
}

export function messageStatusLabel(status) {
  if (status === 'played') return '已播报';
  if (status === 'failed') return '失败';
  return '排队中';
}

function defaultFormatDateTime(value) {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}
