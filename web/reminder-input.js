export function normalizeReminderDraft({
  content = '',
  scheduledAtLocal = '',
  now = new Date(),
} = {}) {
  const normalizedContent = String(content ?? '').trim();
  const normalizedLocal = String(scheduledAtLocal ?? '').trim();

  if (!normalizedContent || !normalizedLocal) {
    return invalidReminderDraft(normalizedContent, '提醒内容和时间都要填写');
  }

  const scheduledAt = new Date(normalizedLocal);
  const nowDate = now instanceof Date ? now : new Date(now);
  if (Number.isNaN(scheduledAt.getTime())) {
    return invalidReminderDraft(normalizedContent, '提醒时间格式无效');
  }
  if (Number.isNaN(nowDate.getTime()) || scheduledAt <= nowDate) {
    return invalidReminderDraft(normalizedContent, '提醒时间要晚于现在');
  }

  return {
    content: normalizedContent,
    scheduledAt: scheduledAt.toISOString(),
    valid: true,
    notice: '',
  };
}

function invalidReminderDraft(content, notice) {
  return {
    content,
    scheduledAt: '',
    valid: false,
    notice,
  };
}
