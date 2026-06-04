import { ApiError } from './api/client.js';

export function formatApiErrorNotice(error, fallback) {
  if (!(error instanceof ApiError)) {
    return fallback;
  }

  const message = String(error.message || fallback || '请求失败').trim() || fallback || '请求失败';
  return error.status ? `${message}（${error.status}）` : message;
}
