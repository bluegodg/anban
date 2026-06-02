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
    captureVision(payload) {
      return request('/api/vision/capture', { method: 'POST', body: payload });
    },
    getGreetingSchedule({ deviceId } = {}) {
      const params = new URLSearchParams();
      if (deviceId) params.set('deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/greetings/schedule${suffix}`);
    },
    updateGreetingSchedule(payload) {
      return request('/api/greetings/schedule', { method: 'PUT', body: payload });
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
    deleteReminder(reminderId) {
      return request(`/api/reminders/${encodeURIComponent(reminderId)}`, { method: 'DELETE' });
    },
    ackReminder(reminderId, payload) {
      return request(`/api/reminders/${encodeURIComponent(reminderId)}/ack`, { method: 'POST', body: payload });
    },
    getStatus({ deviceId } = {}) {
      const params = new URLSearchParams();
      if (deviceId) params.set('deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/device/status${suffix}`);
    },
    getProfile({ deviceId } = {}) {
      const params = new URLSearchParams();
      if (deviceId) params.set('deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/profile${suffix}`);
    },
    updateProfile(payload) {
      return request('/api/profile', { method: 'PUT', body: payload });
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
