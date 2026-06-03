export const MESSAGE_STATUS_REFRESH_INTERVAL_MS = 10_000;

export function startMessageStatusPolling(refresh, { setIntervalImpl = setInterval } = {}) {
  if (typeof refresh !== 'function') {
    throw new TypeError('message status polling refresh must be a function');
  }
  return setIntervalImpl(refresh, MESSAGE_STATUS_REFRESH_INTERVAL_MS);
}

export function stopMessageStatusPolling(timerId, { clearIntervalImpl = clearInterval } = {}) {
  if (timerId !== undefined && timerId !== null) {
    clearIntervalImpl(timerId);
  }
}
