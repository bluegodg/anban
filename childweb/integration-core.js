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
