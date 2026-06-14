export const STATUS_REFRESH_INTERVAL_MS = 20_000;

export function startStatusPolling(refresh, { setIntervalImpl = setInterval } = {}) {
  if (typeof refresh !== 'function') {
    throw new TypeError('status polling refresh must be a function');
  }
  return setIntervalImpl(refresh, STATUS_REFRESH_INTERVAL_MS);
}

export function stopStatusPolling(timerId, { clearIntervalImpl = clearInterval } = {}) {
  if (timerId !== undefined && timerId !== null) {
    clearIntervalImpl(timerId);
  }
}
