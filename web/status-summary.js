export function formatStatusDetail(snapshot = {}, { formatDateTime = defaultFormatDateTime } = {}) {
  const at = snapshot.lastInteractionAt || snapshot.lastSeenAt;
  const parts = [at ? `最近互动：${formatDateTime(at)}` : '暂无最近互动'];
  const latestMessage = Array.isArray(snapshot.messages) ? snapshot.messages[0] : null;

  if (latestMessage?.status) {
    parts.push(`最新留言：${messageStatusLabel(latestMessage.status)}`);
  }

  return parts.join(' · ');
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
