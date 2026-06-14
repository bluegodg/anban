export function formatMessageSendResult(message = {}, { draftNotice = '' } = {}) {
  const text = String(message.text || '').trim();
  const suffix = String(draftNotice || '').trim();

  if (message.status === 'pending') {
    return {
      label: '在线',
      detail: '留言已排队等待设备空闲',
      notice: appendDraftNotice(withOptionalText('留言已排队', text), suffix),
    };
  }

  if (message.status === 'failed') {
    return {
      label: '在线',
      detail: '留言发送失败',
      notice: appendDraftNotice(withOptionalText('留言发送失败', text), suffix),
    };
  }

  return {
    label: '在线',
    detail: '留言已播报',
    notice: appendDraftNotice(withOptionalText('留言已播报', text), suffix),
  };
}

function withOptionalText(prefix, text) {
  return text ? `${prefix}：${text}` : prefix;
}

function appendDraftNotice(base, draftNotice) {
  return draftNotice ? `${base}（${draftNotice}）` : base;
}
