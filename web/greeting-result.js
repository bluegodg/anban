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

function withOptionalText(prefix, text) {
  return text ? `${prefix}：${text}` : prefix;
}
