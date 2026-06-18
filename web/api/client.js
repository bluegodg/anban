export function createAnbanClient({ baseURL = '', accessCode = '', token = '', isBound = true, fetchImpl = fetch } = {}) {
  const root = baseURL.trim().replace(/\/+$/, '');
  const childAccessCode = String(accessCode || '').trim();
  const bearerToken = String(token || '').trim();

  async function request(path, { method = 'GET', body, device = false } = {}) {
    if (device && bearerToken && !isBound) {
      throw new ApiError('请先绑定安伴设备', 409, { error: 'device_not_bound' });
    }
    const headers = {};
    if (bearerToken) headers.Authorization = `Bearer ${bearerToken}`;
    else if (childAccessCode) headers['X-Access-Code'] = childAccessCode;
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

  async function requestBlob(path, { method = 'GET', device = false } = {}) {
    if (device && bearerToken && !isBound) {
      throw new ApiError('请先绑定安伴设备', 409, { error: 'device_not_bound' });
    }
    const headers = {};
    if (bearerToken) headers.Authorization = `Bearer ${bearerToken}`;
    else if (childAccessCode) headers['X-Access-Code'] = childAccessCode;

    const response = await fetchImpl(`${root}${path}`, { method, headers });
    if (!response.ok) {
      const contentType = response.headers.get('Content-Type') || '';
      const payload = contentType.includes('application/json') ? await response.json() : await response.text();
      const message = typeof payload === 'string' ? payload : payload.error || '请求失败';
      throw new ApiError(message, response.status, payload);
    }
    return response.blob();
  }

  return {
    register(payload) {
      return request('/api/auth/register', { method: 'POST', body: payload });
    },
    login(payload) {
      return request('/api/auth/login', { method: 'POST', body: payload });
    },
    requestVerificationCode(payload) {
      return request('/api/auth/verification-code', { method: 'POST', body: payload });
    },
    codeLogin(payload) {
      return request('/api/auth/code-login', { method: 'POST', body: payload });
    },
    logout() {
      return request('/api/auth/logout', { method: 'POST' });
    },
    getMe() {
      return request('/api/me');
    },
    updateMe(payload) {
      return request('/api/me', { method: 'PUT', body: payload });
    },
    bindDevice(payload) {
      return request('/api/device-binding', { method: 'POST', body: payload });
    },
    unbindDevice() {
      return request('/api/device-binding', { method: 'DELETE' });
    },
    resetBindingCode() {
      return request('/api/device-binding/reset-code', { method: 'POST' });
    },
    listMembers() {
      return request('/api/device-binding/members');
    },
    removeMember(accountId) {
      return request(`/api/device-binding/members/${encodePathSegment(accountId)}`, { method: 'DELETE' });
    },
    getTimeline({ deviceId, limit, before, elderDisplayName } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      setQueryParam(params, 'limit', limit);
      setQueryParam(params, 'before', before);
      setQueryParam(params, 'elderDisplayName', elderDisplayName);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/timeline${suffix}`, { device: true });
    },
    sendMessage(payload) {
      return request('/api/messages', { method: 'POST', body: payload, device: true });
    },
    listMessages({ deviceId, status } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      setQueryParam(params, 'status', status);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/messages${suffix}`, { device: true });
    },
    triggerGreeting(payload) {
      return request('/api/greetings/trigger', { method: 'POST', body: payload, device: true });
    },
    captureVision(payload) {
      return request('/api/vision/capture', { method: 'POST', body: payload, device: true });
    },
    lookVision(payload = {}) {
      return request('/api/vision/look', { method: 'POST', body: payload, device: true });
    },
    getVisionCaptureImage(captureId, { deviceId } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return requestBlob(`/api/vision/captures/${encodePathSegment(captureId)}/image${suffix}`, { device: true });
    },
    listVisionCaptures({ deviceId, limit } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      setQueryParam(params, 'limit', limit);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/vision/captures${suffix}`, { device: true });
    },
    reanalyzeVisionCapture(captureId, { deviceId } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/vision/captures/${encodePathSegment(captureId)}/reanalyze${suffix}`, { method: 'POST', device: true });
    },
    checkVisionPresence(payload) {
      return request('/api/vision/check-presence', { method: 'POST', body: payload, device: true });
    },
    getGreetingSchedule({ deviceId } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/greetings/schedule${suffix}`, { device: true });
    },
    updateGreetingSchedule(payload) {
      return request('/api/greetings/schedule', { method: 'PUT', body: payload, device: true });
    },
    createReminder(payload) {
      return request('/api/reminders', { method: 'POST', body: payload, device: true });
    },
    listReminders({ deviceId, status } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      setQueryParam(params, 'status', status);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/reminders${suffix}`, { device: true });
    },
    deleteReminder(reminderId) {
      return request(`/api/reminders/${encodePathSegment(reminderId)}`, { method: 'DELETE', device: true });
    },
    ackReminder(reminderId, payload) {
      return request(`/api/reminders/${encodePathSegment(reminderId)}/ack`, { method: 'POST', body: payload, device: true });
    },
    getStatus({ deviceId } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/device/status${suffix}`, { device: true });
    },
    getHistory({ deviceId, limit } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      setQueryParam(params, 'limit', limit);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/device/history${suffix}`, { device: true });
    },
    getProfile({ deviceId } = {}) {
      const params = new URLSearchParams();
      setQueryParam(params, 'deviceId', deviceId);
      const suffix = params.toString() ? `?${params}` : '';
      return request(`/api/profile${suffix}`, { device: true });
    },
    updateProfile(payload) {
      return request('/api/profile', { method: 'PUT', body: payload, device: true });
    },
  };
}

function setQueryParam(params, name, value) {
  const normalized = String(value ?? '').trim();
  if (normalized) params.set(name, normalized);
}

function encodePathSegment(value) {
  return encodeURIComponent(String(value ?? '').trim());
}

export class ApiError extends Error {
  constructor(message, status, payload) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.payload = payload;
  }
}
