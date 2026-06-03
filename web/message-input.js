export const MESSAGE_TEXT_LIMIT = 100;

export function normalizeMessageDraft(value) {
  const text = String(value || '').trim();
  const chars = Array.from(text);

  if (chars.length <= MESSAGE_TEXT_LIMIT) {
    return {
      text,
      wasTruncated: false,
      notice: '',
    };
  }

  return {
    text: chars.slice(0, MESSAGE_TEXT_LIMIT).join(''),
    wasTruncated: true,
    notice: `留言已按 ${MESSAGE_TEXT_LIMIT} 字发送`,
  };
}
