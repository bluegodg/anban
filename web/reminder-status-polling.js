export const REMINDER_STATUS_REFRESH_INTERVAL_MS = 10_000;

export function startReminderStatusPolling(refresh, { setIntervalImpl = setInterval } = {}) {
  if (typeof refresh !== 'function') {
    throw new TypeError('reminder status polling refresh must be a function');
  }
  return setIntervalImpl(refresh, REMINDER_STATUS_REFRESH_INTERVAL_MS);
}

export function stopReminderStatusPolling(timerId, { clearIntervalImpl = clearInterval } = {}) {
  if (timerId !== undefined && timerId !== null) {
    clearIntervalImpl(timerId);
  }
}
