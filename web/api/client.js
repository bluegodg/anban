export function createAnbanClient({ baseURL = '', accessCode = '', fetchImpl = fetch } = {}) {
  const root = baseURL.replace(/\/$/, '');

  async function request(path, { method = 'GET', body } = {}) {
    const headers = {
      'X-Access-Code': accessCode,
    };
    const init = { method, headers };
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(body);
    }

    const response = await fetchImpl(`${root}${path}`, init);
    const contentType = response.headers.get('Content-Type') || '';
    const payload = contentType.includes('application/json') ? await response.json() : await response.text();
    if (!response.ok) {
      const message = typeof payload === 'string' ? payload : payload.error || '请求失败';
      throw new ApiError(message, response.status, payload);
    }
    return payload;
  }

  return {
    sendMessage(payload) {
      return request('/api/messages', { method: 'POST', body: payload });
    },
    listMessages({ deviceId, status } = {}) {
      const params = new URLSearchParams();
      if (deviceId) params.set('deviceId', deviceId);
      if (status) params.set('status', status);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/messages${suffix}`);
    },
    triggerGreeting(payload) {
      return request('/api/greetings/trigger', { method: 'POST', body: payload });
    },
    createReminder(payload) {
      return request('/api/reminders', { method: 'POST', body: payload });
    },
    listReminders({ deviceId, status } = {}) {
      const params = new URLSearchParams();
      if (deviceId) params.set('deviceId', deviceId);
      if (status) params.set('status', status);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/reminders${suffix}`);
    },
    getStatus({ deviceId } = {}) {
      const params = new URLSearchParams();
      if (deviceId) params.set('deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/device/status${suffix}`);
    },
  };
}

export class ApiError extends Error {
  constructor(message, status, payload) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.payload = payload;
  }
}
