import { ApiError, createAnbanClient } from './api/client.js';
import { loadConfig, saveConfig } from './config.js';
import {
  buildConversationBubbles,
  buildHomeStatus,
  buildReminderScheduleOptions,
  buildVisionCaptureView,
  buildVisionLookProgress,
  formatGreetingTriggerResult,
  formatLoginError,
  formatRelativeTime,
  mapFieldsToStitchProfile,
  mapStitchProfileToFields,
  nextOccurrenceUTC,
  normalizeGreetingSlots,
  normalizeHistoryMessages,
} from './integration-core.js';
import { notImplemented as notifyNotImplemented } from './not-implemented.js';

var anbanConfig = loadConfig();
var anbanSession = {
  token: localStorage.getItem('anban_account_token') || '',
  account: null,
  binding: null,
  authMode: localStorage.getItem('anban_auth_mode') || '',
};
var anbanClient = createRuntimeClient();
var visionImageObjectURL = '';
var visionCurrentCapture = null;

window.anbanRuntime = {
  ApiError,
  config: anbanConfig,
  client: anbanClient,
};

if ('serviceWorker' in navigator) {
  window.addEventListener('load', function() {
    navigator.serviceWorker.register('./sw.js').catch(function() {});
  });
}

window.addEventListener('pagehide', releaseVisionImageObjectURL);

function releaseVisionImageObjectURL() {
  if (!visionImageObjectURL) return;
  URL.revokeObjectURL(visionImageObjectURL);
  visionImageObjectURL = '';
}

function closeVisionResult() {
  var overlay = document.getElementById('visionResultOverlay');
  if (overlay) overlay.classList.remove('open');
  var image = document.getElementById('visionResultImage');
  var empty = document.getElementById('visionResultImageEmpty');
  var wrap = document.getElementById('visionResultImageWrap');
  if (image) {
    image.removeAttribute('src');
    image.style.display = 'none';
  }
  if (empty) empty.style.display = '';
  if (wrap) wrap.classList.add('is-empty');
  releaseVisionImageObjectURL();
  visionCurrentCapture = null;
}

function updateAnbanConfig(patch) {
  anbanConfig = saveConfig({ ...anbanConfig, ...patch });
  anbanClient = createRuntimeClient();
  window.anbanRuntime.config = anbanConfig;
  window.anbanRuntime.client = anbanClient;
  return anbanConfig;
}

function createRuntimeClient() {
  return createAnbanClient({
    ...anbanConfig,
    token: anbanSession.token,
    isBound: !anbanSession.token || Boolean(anbanSession.binding),
  });
}

function setAccountSession(payload) {
  anbanSession.token = payload && payload.token ? payload.token : '';
  anbanSession.account = payload && payload.account ? payload.account : null;
  anbanSession.binding = payload && payload.binding ? payload.binding : null;
  anbanSession.authMode = anbanSession.token ? 'account' : '';
  if (anbanSession.token) {
    localStorage.setItem('anban_account_token', anbanSession.token);
    localStorage.setItem('anban_auth_mode', 'account');
    localStorage.setItem('anban_session', '1');
  } else {
    localStorage.removeItem('anban_account_token');
  }
  anbanClient = createRuntimeClient();
  window.anbanRuntime.client = anbanClient;
  applyBindingState();
}

async function restoreAccountSession() {
  if (!anbanSession.token) return false;
  anbanClient = createRuntimeClient();
  try {
    var me = await anbanClient.getMe();
    anbanSession.account = me.account || null;
    anbanSession.binding = me.binding || null;
    anbanSession.authMode = 'account';
    anbanClient = createRuntimeClient();
    window.anbanRuntime.client = anbanClient;
    return true;
  } catch (error) {
    setAccountSession(null);
    localStorage.removeItem('anban_session');
    return false;
  }
}

function isAccountMode() {
  return anbanSession.authMode === 'account' && Boolean(anbanSession.token);
}

function isDeviceBound() {
  return !isAccountMode() || Boolean(anbanSession.binding);
}

function ensureDeviceBound() {
  if (isDeviceBound()) return true;
  openBindDevice();
  showToast('请先绑定安伴设备');
  return false;
}

function notImplemented(featureName) {
  return notifyNotImplemented(featureName, showToast);
}

function sendChildMessage(text) {
  if (!ensureDeviceBound()) return Promise.reject(new ApiError('请先绑定安伴设备', 409, { error: 'device_not_bound' }));
  if (isAccountMode()) return anbanClient.sendMessage({ text: text });
  return anbanClient.sendMessage({ deviceId: anbanConfig.deviceId, fromName: '家人', text: text });
}

// ============================
// SPA Router
// ============================
var SPA = {
  currentSection: null,
  initialized: {},
  sectionsWithNav: { home:1, warn:1, message:1, family:1, mine:1 },
  sectionIds: { login:'s-login', home:'s-home', warn:'s-warn', message:'s-message', family:'s-family', mine:'s-mine', 'family-edit':'s-family-edit', history:'s-history', detail:'s-detail' }
};

function navigateTo(name) {
  // Save warn scroll position before navigating to sub-pages (detail, history)
  // Only sub-pages should restore scroll when returning; main pages should reset to top
  var subPages = ['detail', 'history', 'family-edit'];
  if (SPA.currentSection === 'warn' && subPages.indexOf(name) >= 0) {
    var inner = document.getElementById('screenInner');
    if (inner) SPA._warnScrollTop = inner.scrollTop;
    SPA._fromWarnSubpage = true;
  }
  location.hash = name;
}

function getRouteFromHash() {
  var h = location.hash.replace('#','');
  if (!h || !SPA.sectionIds[h]) return 'login';
  return h;
}

function resetLoginUI() {
  var btn = document.getElementById('loginBtn');
  var btnText = document.getElementById('loginBtnText');
  var btnIcon = document.getElementById('loginBtnIcon');
  var input = document.getElementById('loginPhone');

  if (btn) {
    btn.style.pointerEvents = '';
    btn.style.opacity = '';
  }
  if (btnText) btnText.textContent = '登录';
  if (btnIcon) {
    btnIcon.textContent = 'arrow_forward';
    btnIcon.style.animation = '';
  }
  if (input) input.classList.remove('border-danger');
}

function showSection(name) {
  // Reset login UI when entering login page
  if (name === 'login') resetLoginUI();

  // Hide current
  if (SPA.currentSection) {
    var oldEl = document.getElementById(SPA.sectionIds[SPA.currentSection]);
    if (oldEl) oldEl.classList.remove('active');
  }

  // Show target
  var el = document.getElementById(SPA.sectionIds[name]);
  if (el) {
    el.classList.add('active');
    // Reset scroll
    var inner = document.getElementById('screenInner');
    if (inner) inner.scrollTop = 0;
  }

  SPA.currentSection = name;
  // Restore warn scroll position when returning from a sub-page (detail, history, etc.)
  if (name === 'warn' && SPA._fromWarnSubpage && typeof SPA._warnScrollTop === 'number' && SPA._warnScrollTop > 0) {
    var inner = document.getElementById('screenInner');
    var saved = SPA._warnScrollTop;
    SPA._warnScrollTop = undefined;
    SPA._fromWarnSubpage = false;
    if (inner) setTimeout(function() { inner.scrollTop = saved; }, 50);
  } else if (name === 'warn') {
    // Coming from other main pages (not sub-pages) - go to top
    SPA._warnScrollTop = undefined;
    SPA._fromWarnSubpage = false;
  }

  // Hide bottom navs for secondary pages (history, family-edit)
  var noNavPages = ['history', 'family-edit', 'detail'];
  var allNavs = document.querySelectorAll('#s-home nav, #s-warn nav, #s-message nav, #s-family nav, #s-mine nav');
  if (noNavPages.indexOf(name) >= 0) {
    for (var n = 0; n < allNavs.length; n++) allNavs[n].style.display = 'none';
  } else {
    for (var n = 0; n < allNavs.length; n++) allNavs[n].style.display = '';
    // Also ensure the current section's nav is visible
    var curSection = document.getElementById(SPA.sectionIds[name]);
    if (curSection) {
      var curNav = curSection.querySelector('nav');
      if (curNav) curNav.style.display = '';
    }
  }

  // Update status bar color
  var sb = document.getElementById('globalStatusBar');
  if (sb) {
    if (name === 'login') { sb.className = 'status-bar light'; }
    else { sb.className = 'status-bar dark'; }
  }

  // Lazy init
  var initFn = 'init' + name.charAt(0).toUpperCase() + name.slice(1).replace(/-./g, function(x){return x[1].toUpperCase()});
  var alwaysRefresh = ['history', 'detail'];
  if ((!SPA.initialized[name] || alwaysRefresh.indexOf(name) >= 0) && typeof window[initFn] === 'function') {
    SPA.initialized[name] = true;
    window[initFn]();
  }

  if (name === 'home' && typeof window.refreshHome === 'function') window.refreshHome();
  if (name === 'message' && typeof window.refreshMessages === 'function') window.refreshMessages();
  if (name === 'warn' && typeof window.refreshReminders === 'function') window.refreshReminders();

  // Update nav active states
  updateNavActive(name);
}

function updateNavActive(name) {
  var allLinks = document.querySelectorAll('.nav-link');
  for (var i = 0; i < allLinks.length; i++) {
    var link = allLinks[i];
    var nav = link.getAttribute('data-nav');
    var parent = link.parentElement;
    var isActive = (nav === name) || (name === 'mine' && nav === 'mine');

    if (isActive) {
      link.style.color = '#9a4429';
      var icon = link.querySelector('.material-symbols-outlined');
      if (icon) icon.style.fontVariationSettings = "'FILL' 1,'wght' 400,'GRAD' 0,'opsz' 24";
      var label = link.querySelector('.font-label-sm');
      if (label) label.classList.add('font-medium');
      // Home/warn/message/family use the circle highlight pattern
      if (parent.classList.contains('relative') && parent.querySelector('.absolute')) {
        parent.querySelector('.absolute').classList.add('bg-primary-container/10');
        parent.querySelector('.absolute').classList.remove('hidden');
      }
    } else {
      link.style.color = '';
      var icon = link.querySelector('.material-symbols-outlined');
      if (icon) icon.style.fontVariationSettings = '';
      var label = link.querySelector('.font-label-sm');
      if (label) label.classList.remove('font-medium');
    }
  }
}

Object.assign(window, {
  navigateTo,
  notImplemented,
  showToast,
  closeVisionResult,
  openBindDevice,
  closeBindDevice,
  submitDeviceBinding,
  saveAccountProfile,
  resetDeviceBindingCode,
  unbindCurrentDevice,
  initLogin,
  initHome,
  initMessage,
  initWarn,
  initFamily,
  initMine,
  initFamilyEdit,
});

// Hash change handler
window.addEventListener('hashchange', function() {
  showSection(getRouteFromHash());
});

// Initial load
await restoreAccountSession();
var initialRoute = getRouteFromHash();
if (initialRoute === 'login') {
  // If already logged in, skip to home
  if (localStorage.getItem('anban_session')) {
    initialRoute = 'home';
    location.replace('#home');
  }
}
showSection(initialRoute);
applyBindingState();

// Update status bar time to real time
function updateStatusBarTime() {
  var now = new Date();
  var h = now.getHours();
  var m = now.getMinutes();
  var timeStr = (h < 10 ? '0' : '') + h + ':' + (m < 10 ? '0' : '') + m;
  var sb = document.getElementById('globalStatusBar');
  if (sb) {
    var timeEl = sb.querySelector('.status-bar-time');
    if (timeEl) timeEl.textContent = timeStr;
  }
}
updateStatusBarTime();
setInterval(updateStatusBarTime, 10000);

// ============================
// Shared Toast
// ============================
function showToast(msg) {
  var t = document.getElementById('spaToast');
  t.textContent = msg;
  t.classList.add('show');
  setTimeout(function() { t.classList.remove('show'); }, 2000);
}

// Shared touch feedback
document.addEventListener('touchstart', function(e) {
  var el = e.target.closest('button, a');
  if (el) el.style.opacity = '0.7';
}, {passive:true});
document.addEventListener('touchend', function(e) {
  var el = e.target.closest('button, a');
  if (el) el.style.opacity = '1';
}, {passive:true});

// ============================
// initLogin
// ============================
function initLogin() {
  var loginMode = 'password';
  function setLoginMode(mode) {
    loginMode = mode;
    ['password', 'code', 'demo'].forEach(function(name) {
      var panel = document.getElementById(name + 'LoginPanel');
      var button = document.getElementById(name + 'LoginMode');
      if (panel) panel.style.display = name === mode ? '' : 'none';
      if (button) button.classList.toggle('active', name === mode);
    });
  }

  async function finishAccountLogin(response) {
    setAccountSession(response);
    var me = await anbanClient.getMe();
    anbanSession.account = me.account || response.account || null;
    anbanSession.binding = me.binding || null;
    anbanClient = createRuntimeClient();
    window.anbanRuntime.client = anbanClient;
    SPA.initialized = {};
    navigateTo('home');
    setTimeout(applyBindingState, 0);
  }

  async function handleLogin() {
    var phoneInput = document.getElementById('loginPhone');
    var passwordInput = document.getElementById('loginPassword');
    var phone = phoneInput.value.trim();
    var password = passwordInput.value;
    if (!phone || !password) {
      showToast('请输入手机号和密码');
      return;
    }
    var btn = document.getElementById('loginBtn');
    var btnText = document.getElementById('loginBtnText');
    var btnIcon = document.getElementById('loginBtnIcon');
    btnText.textContent = '验证中...';
    btnIcon.textContent = 'hourglass_top';
    btnIcon.style.animation = 'spin 1s linear infinite';
    btn.style.pointerEvents = 'none';
    btn.style.opacity = '0.85';

    try {
      var candidateClient = createAnbanClient({ baseURL: anbanConfig.baseURL });
      await finishAccountLogin(await candidateClient.login({ phone: phone, password: password }));
    } catch (error) {
      showToast(error.message || '登录失败');
    } finally {
      btnText.textContent = '登录';
      btnIcon.textContent = 'arrow_forward';
      btnIcon.style.animation = '';
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
    }
  }

  async function handleRegister() {
    var phone = document.getElementById('loginPhone').value.trim();
    var password = document.getElementById('loginPassword').value;
    var nickname = document.getElementById('registerNickname').value.trim();
    if (!phone || !password) {
      showToast('请输入手机号和至少 6 位密码');
      return;
    }
    try {
      var client = createAnbanClient({ baseURL: anbanConfig.baseURL });
      await finishAccountLogin(await client.register({ phone: phone, password: password, nickname: nickname }));
    } catch (error) {
      showToast(error.message || '注册失败');
    }
  }

  async function requestCode() {
    var phone = document.getElementById('codeLoginPhone').value.trim();
    if (!phone) {
      showToast('请输入手机号');
      return;
    }
    try {
      var client = createAnbanClient({ baseURL: anbanConfig.baseURL });
      var result = await client.requestVerificationCode({ phone: phone, purpose: 'login' });
      showToast(result.debugCode ? '开发验证码：' + result.debugCode : '验证码已发送');
    } catch (error) {
      showToast(error.message || '验证码发送失败');
    }
  }

  async function handleCodeLogin() {
    var phone = document.getElementById('codeLoginPhone').value.trim();
    var code = document.getElementById('loginCode').value.trim();
    if (!phone || !code) {
      showToast('请输入手机号和验证码');
      return;
    }
    try {
      var client = createAnbanClient({ baseURL: anbanConfig.baseURL });
      await finishAccountLogin(await client.codeLogin({ phone: phone, code: code }));
    } catch (error) {
      showToast(error.message || '验证码登录失败');
    }
  }

  async function handleDemoLogin() {
    var input = document.getElementById('accessCode');
    var accessCode = input.value.trim();
    if (!accessCode) {
      showToast('请输入演示访问码');
      return;
    }
    try {
      var candidateClient = createAnbanClient({ baseURL: anbanConfig.baseURL, accessCode: accessCode });
      await candidateClient.getStatus({ deviceId: anbanConfig.deviceId });
      setAccountSession(null);
      anbanSession.authMode = 'demo';
      localStorage.setItem('anban_auth_mode', 'demo');
      updateAnbanConfig({ accessCode: accessCode });
      localStorage.setItem('anban_session', '1');
      SPA.initialized = {};
      navigateTo('home');
    } catch (error) {
      showToast(formatLoginError(error));
    }
  }

  document.getElementById('loginBtn').addEventListener('click', handleLogin);
  document.getElementById('registerBtn').addEventListener('click', handleRegister);
  document.getElementById('requestCodeBtn').addEventListener('click', requestCode);
  document.getElementById('codeLoginBtn').addEventListener('click', handleCodeLogin);
  document.getElementById('demoLoginBtn').addEventListener('click', handleDemoLogin);
  document.getElementById('passwordLoginMode').addEventListener('click', function() { setLoginMode('password'); });
  document.getElementById('codeLoginMode').addEventListener('click', function() { setLoginMode('code'); });
  document.getElementById('demoLoginMode').addEventListener('click', function() { setLoginMode('demo'); });
  document.getElementById('loginPassword').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') handleLogin();
  });
  setLoginMode(loginMode);
}

function applyBindingState() {
  var locked = isAccountMode() && !anbanSession.binding;
  ['s-home', 's-message', 's-warn', 's-family'].forEach(function(sectionId) {
    var section = document.getElementById(sectionId);
    if (!section) return;
    section.classList.toggle('anban-unbound-section', locked);
    var main = section.querySelector('main');
    if (main) main.classList.toggle('anban-device-locked', locked);
  });
  var notice = document.getElementById('unboundNotice');
  if (notice) notice.style.display = locked ? 'flex' : 'none';
  var admin = !isAccountMode() || (anbanSession.binding && anbanSession.binding.role === 'admin');
  document.querySelectorAll('a[href="#family-edit"]').forEach(function(profileEdit) {
    profileEdit.style.display = admin ? '' : 'none';
  });
  document.querySelectorAll('.memory-admin-control').forEach(function(memoryControl) {
    memoryControl.style.display = admin ? '' : 'none';
  });
  renderAccountSettings();
}

function openBindDevice() {
  if (!isAccountMode()) {
    showToast('请先使用子女账号登录');
    return;
  }
  var overlay = document.getElementById('bindDeviceOverlay');
  var card = document.getElementById('bindDeviceCard');
  if (overlay) overlay.classList.add('open');
  if (card) card.classList.add('open');
}

function closeBindDevice() {
  var overlay = document.getElementById('bindDeviceOverlay');
  var card = document.getElementById('bindDeviceCard');
  if (overlay) overlay.classList.remove('open');
  if (card) card.classList.remove('open');
}

async function submitDeviceBinding() {
  var input = document.getElementById('bindingCodeInput');
  var selected = document.querySelector('input[name="bindingRole"]:checked');
  var bindingCode = input ? input.value.trim() : '';
  var role = selected ? selected.value : 'member';
  if (!bindingCode) {
    showToast('请输入设备码');
    return;
  }
  try {
    await anbanClient.bindDevice({ role: role, bindingCode: bindingCode });
    var me = await anbanClient.getMe();
    anbanSession.account = me.account;
    anbanSession.binding = me.binding;
    anbanClient = createRuntimeClient();
    window.anbanRuntime.client = anbanClient;
    closeBindDevice();
    applyBindingState();
    SPA.initialized = {};
    showToast('安伴设备绑定成功');
    if (typeof window.refreshHome === 'function') window.refreshHome();
  } catch (error) {
    var messages = {
      device_code_not_found: '设备码不存在',
      account_already_bound: '当前账号已绑定设备',
      admin_already_bound: '该设备已有家庭管理员',
    };
    showToast(messages[error.payload && error.payload.error] || error.message || '设备绑定失败');
  }
}

function renderAccountSettings() {
  var account = anbanSession.account || {};
  var binding = anbanSession.binding;
  var fields = {
    accountNickname: account.nickname || '',
    accountRealName: account.realName || '',
    accountRelationship: account.relationshipToElder || '',
    accountAvatarColor: account.avatarColor || '#E89A6A',
  };
  Object.keys(fields).forEach(function(id) {
    var el = document.getElementById(id);
    if (el && document.activeElement !== el) el.value = fields[id];
  });
  var title = document.getElementById('accountDisplayName');
  if (title) title.textContent = account.displayName || account.nickname || '子女账号';
  var phone = document.getElementById('accountMaskedPhone');
  if (phone) phone.textContent = account.phone || '';
  var deviceSummary = document.getElementById('bindingSummary');
  if (deviceSummary) {
    deviceSummary.textContent = binding
      ? (binding.deviceDisplayName || '安伴设备') + ' · ' + (binding.role === 'admin' ? '家庭管理员' : '家庭成员')
      : '尚未绑定安伴设备';
  }
  var adminPanel = document.getElementById('adminDevicePanel');
  if (adminPanel) adminPanel.style.display = binding && binding.role === 'admin' ? '' : 'none';
  var bindButton = document.getElementById('settingsBindDeviceBtn');
  if (bindButton) bindButton.style.display = isAccountMode() && !binding ? '' : 'none';
}

async function saveAccountProfile() {
  if (!isAccountMode()) {
    showToast('演示模式不保存账号资料');
    return;
  }
  try {
    var account = await anbanClient.updateMe({
      nickname: document.getElementById('accountNickname').value.trim(),
      realName: document.getElementById('accountRealName').value.trim(),
      relationshipToElder: document.getElementById('accountRelationship').value.trim(),
      avatarColor: document.getElementById('accountAvatarColor').value,
    });
    anbanSession.account = account;
    renderAccountSettings();
    showToast('个人资料已保存');
  } catch (error) {
    showToast(error.message || '个人资料保存失败');
  }
}

async function resetDeviceBindingCode() {
  try {
    var result = await anbanClient.resetBindingCode();
    showToast('新设备码：' + result.bindingCode);
  } catch (error) {
    showToast(error.message || '设备码重置失败');
  }
}

async function unbindCurrentDevice() {
  if (!confirm('确认解绑当前安伴设备？已有家庭成员仍会保留绑定。')) return;
  try {
    await anbanClient.unbindDevice();
    anbanSession.binding = null;
    anbanClient = createRuntimeClient();
    window.anbanRuntime.client = anbanClient;
    applyBindingState();
    showToast('设备已解绑');
  } catch (error) {
    showToast(error.message || '设备解绑失败');
  }
}

// ============================
// initHome
// ============================
function initHome() {
  function renderHomeStatus(payload) {
    var view = buildHomeStatus(payload);
    var badge = document.getElementById('deviceStatusBadge');
    var dot = document.getElementById('deviceStatusDot');
    var label = document.getElementById('deviceStatusLabel');
    var title = document.getElementById('statusTitle');
    var desc = document.getElementById('statusDesc');
    var updated = document.getElementById('statusTime');

    if (badge) badge.className = 'ab-tag ' + (view.online ? 'ab-tag-ok' : 'ab-tag-off');
    if (dot) dot.className = 'ab-dot' + (view.online ? ' ab-dot-pulse' : '');
    if (label) label.textContent = view.label;
    if (title) title.textContent = view.title;
    if (desc) desc.textContent = view.description;
    if (updated) updated.textContent = view.updatedAt;
  }

  function renderRecentHistory(payload) {
    var list = document.getElementById('recentMsgList');
    if (!list) return;
    list.innerHTML = '';

    var messages = normalizeHistoryMessages(payload, 2);
    if (!messages.length) {
      var empty = document.createElement('div');
      empty.className = 'ab-card';
      empty.style.cssText = 'padding:18px;text-align:center;color:var(--ab-ink3);font-size:13px';
      empty.textContent = '暂无最近对话';
      list.appendChild(empty);
    }

    messages.forEach(function(item) {
      var card = document.createElement('div');
      card.className = 'ab-card';
      card.style.cssText = 'padding:13px;display:flex;gap:12px;align-items:center';
      card.innerHTML = '<div class="ab-iconbox material-symbols-outlined" style="width:36px;height:36px;background:#f7ece4;color:var(--ab-primary);font-size:18px"></div><div style="flex:1;min-width:0"><div class="flex justify-between items-center" style="gap:8px"><span class="font-label-md" style="font-size:13.5px;font-weight:600;color:var(--ab-ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap"></span><span class="font-label-sm ab-tag ab-tag-cat" style="flex-shrink:0"></span></div><p style="font-size:11.5px;color:var(--ab-ink3);margin-top:3px"></p></div>';
      card.querySelector('.material-symbols-outlined').textContent = item.role === 'user' ? 'person' : 'spatial_audio';
      card.querySelector('.font-label-md').textContent = item.text;
      card.querySelector('.font-label-sm').textContent = item.role === 'user' ? '家人' : '安伴';
      card.querySelector('p').textContent = formatRelativeTime(item.at);
      list.appendChild(card);
    });
  }

  function setVisionProgress(stage) {
    var view = buildVisionLookProgress(stage);
    var statusText = document.getElementById('visionStatusText');
    var label = document.getElementById('visionLookButtonLabel');
    var button = document.getElementById('visionLookButton');
    if (statusText) statusText.textContent = view.statusText;
    if (label) label.textContent = view.buttonText;
    if (button) {
      button.disabled = view.disabled;
      button.classList.toggle('opacity-70', view.disabled);
    }
  }

  function visionToneClass(tone) {
    if (tone === 'success') return 'ab-tag ab-tag-ok';
    if (tone === 'danger') return 'ab-tag ab-tag-off';
    if (tone === 'warning') return 'ab-tag ab-tag-cat';
    return 'ab-tag';
  }

  function renderVisionRecent(captures) {
    var list = document.getElementById('visionRecentList');
    if (!list) return;
    var items = Array.isArray(captures) ? captures.slice(0, 3) : [];
    list.innerHTML = '';
    if (!items.length) {
      var empty = document.createElement('div');
      empty.className = 'ab-card';
      empty.style.cssText = 'padding:13px;color:var(--ab-ink3);font-size:13px';
      empty.textContent = '暂无拍摄记录';
      list.appendChild(empty);
      return;
    }
    items.forEach(function(capture) {
      var view = buildVisionCaptureView(capture);
      var row = document.createElement('button');
      row.type = 'button';
      row.className = 'ab-card active:scale-95 transition-all';
      row.style.cssText = 'padding:12px;display:flex;align-items:center;gap:10px;text-align:left';
      row.innerHTML = '<span class="material-symbols-outlined" style="font-size:19px;color:var(--ab-primary)">photo_camera</span><span style="flex:1;min-width:0"><span class="font-label-md" style="display:block;font-size:13px;font-weight:700;color:var(--ab-ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap"></span><span class="font-label-sm" style="display:block;font-size:11px;color:var(--ab-ink3);margin-top:2px"></span></span><span class="ab-tag" style="flex-shrink:0"></span>';
      row.querySelector('.font-label-md').textContent = view.summary;
      row.querySelector('.font-label-sm').textContent = view.capturedAtLabel;
      var tag = row.querySelector('.ab-tag');
      tag.className = visionToneClass(view.statusTone);
      tag.textContent = view.statusLabel;
      row.addEventListener('click', async function() {
        try {
          await showVisionCapture(capture);
        } catch (error) {
          showToast(error.message || '图片加载失败');
        }
      });
      list.appendChild(row);
    });
  }

  function renderVisionResult(capture, imageURL) {
    var view = buildVisionCaptureView(capture);
    visionCurrentCapture = capture;
    var overlay = document.getElementById('visionResultOverlay');
    var image = document.getElementById('visionResultImage');
    var empty = document.getElementById('visionResultImageEmpty');
    var wrap = document.getElementById('visionResultImageWrap');
    var status = document.getElementById('visionResultStatus');
    var meta = document.getElementById('visionResultMeta');
    var summary = document.getElementById('visionResultSummary');
    var presence = document.getElementById('visionResultPresence');
    var concerns = document.getElementById('visionResultConcerns');
    var action = document.getElementById('visionResultAction');

    if (status) {
      status.className = visionToneClass(view.statusTone);
      status.textContent = view.statusLabel;
    }
    if (meta) meta.textContent = view.capturedAtLabel;
    if (summary) summary.textContent = view.summary;
    if (presence) presence.textContent = view.presenceLabel;
    if (concerns) {
      concerns.innerHTML = '';
      view.concerns.forEach(function(item) {
        var tag = document.createElement('span');
        tag.className = 'ab-tag ab-tag-cat';
        tag.textContent = item;
        concerns.appendChild(tag);
      });
    }
    if (image && empty && wrap) {
      if (imageURL && view.showImage) {
        image.src = imageURL;
        image.style.display = '';
        empty.style.display = 'none';
        wrap.classList.remove('is-empty');
      } else {
        releaseVisionImageObjectURL();
        image.removeAttribute('src');
        image.style.display = 'none';
        empty.style.display = '';
        wrap.classList.add('is-empty');
      }
    }
    if (action) {
      if (view.action) {
        action.style.display = '';
        action.textContent = view.action.label;
        action.onclick = view.action.kind === 'reanalyze' ? reanalyzeCurrentVisionCapture : startVisionLook;
      } else {
        action.style.display = 'none';
        action.onclick = null;
      }
    }
    if (overlay) overlay.classList.add('open');
  }

  async function loadVisionImage(capture) {
    var view = buildVisionCaptureView(capture);
    if (!view.showImage) return '';
    setVisionProgress('analyzing');
    var blob = await anbanClient.getVisionCaptureImage(capture.captureId, { deviceId: anbanConfig.deviceId });
    releaseVisionImageObjectURL();
    visionImageObjectURL = URL.createObjectURL(blob);
    return visionImageObjectURL;
  }

  async function showVisionCapture(capture) {
    var imageURL = '';
    if (buildVisionCaptureView(capture).showImage) {
      imageURL = await loadVisionImage(capture);
    }
    renderVisionResult(capture, imageURL);
  }

  function sleep(ms) {
    return new Promise(function(resolve) { setTimeout(resolve, ms); });
  }

  async function waitForVisionCapture(capture) {
    if (!capture || capture.status !== 'pending') return capture;
    for (var attempt = 0; attempt < 12; attempt++) {
      await sleep(1500);
      var captures = await anbanClient.listVisionCaptures({ deviceId: anbanConfig.deviceId, limit: 3 });
      var found = (Array.isArray(captures) ? captures : []).find(function(item) {
        return item.captureId === capture.captureId;
      });
      if (found && found.status !== 'pending') return found;
    }
    return capture;
  }

  async function refreshVisionCaptures() {
    if (!isDeviceBound()) {
      renderVisionRecent([]);
      return;
    }
    try {
      var captures = await anbanClient.listVisionCaptures({ deviceId: anbanConfig.deviceId, limit: 3 });
      renderVisionRecent(captures);
    } catch (error) {
      renderVisionRecent([]);
    }
  }

  async function reanalyzeCurrentVisionCapture() {
    if (!visionCurrentCapture) return;
    try {
      setVisionProgress('analyzing');
      var capture = await anbanClient.reanalyzeVisionCapture(visionCurrentCapture.captureId, { deviceId: anbanConfig.deviceId });
      await showVisionCapture(capture);
      await refreshVisionCaptures();
      showToast('重新分析完成');
    } catch (error) {
      showToast(error.message || '重新分析失败');
    } finally {
      setVisionProgress('idle');
    }
  }

  async function startVisionLook() {
    if (!ensureDeviceBound()) return;
    var captureTimer;
    var finalStatusText = '';
    try {
      setVisionProgress('connecting');
      captureTimer = setTimeout(function() { setVisionProgress('capturing'); }, 900);
      var capture = await anbanClient.lookVision({ deviceId: anbanConfig.deviceId });
      clearTimeout(captureTimer);
      capture = await waitForVisionCapture(capture);
      await showVisionCapture(capture);
      await refreshVisionCaptures();
      var view = buildVisionCaptureView(capture);
      finalStatusText = view.statusLabel + ' · ' + view.presenceLabel;
      showToast('看一眼完成');
    } catch (error) {
      var failedCapture = {
        status: 'failed',
        failureMessage: error.message || '看一眼失败',
        analysis: { presence: 'unknown', concerns: [] },
      };
      renderVisionResult(failedCapture, '');
      finalStatusText = '看一眼失败';
      showToast(error.message || '看一眼失败');
    } finally {
      clearTimeout(captureTimer);
      setVisionProgress('idle');
      if (finalStatusText) document.getElementById('visionStatusText').textContent = finalStatusText;
    }
  }

  window.refreshHome = async function() {
    if (!isDeviceBound()) {
      renderHomeStatus({ online: false });
      document.getElementById('statusTitle').textContent = '请先绑定安伴设备';
      document.getElementById('statusDesc').textContent = '绑定后即可查看老人状态与最近对话';
      renderRecentHistory({ messages: [] });
      renderVisionRecent([]);
      return;
    }
    var results = await Promise.allSettled([
      anbanClient.getStatus({ deviceId: anbanConfig.deviceId }),
      anbanClient.getHistory({ deviceId: anbanConfig.deviceId, limit: 10 }),
      anbanClient.listVisionCaptures({ deviceId: anbanConfig.deviceId, limit: 3 }),
    ]);

    if (results[0].status === 'fulfilled') {
      renderHomeStatus(results[0].value);
    } else {
      renderHomeStatus({ online: false });
      document.getElementById('statusTitle').textContent = '状态加载失败';
      document.getElementById('statusDesc').textContent = '请检查安伴后端连接';
    }

    if (results[1].status === 'fulfilled') {
      renderRecentHistory(results[1].value);
    } else {
      renderRecentHistory({ messages: [] });
    }

    if (results[2].status === 'fulfilled') {
      renderVisionRecent(results[2].value);
    } else {
      renderVisionRecent([]);
    }
  };

  var greetingButton = document.getElementById('greetingTriggerBtn');
  if (greetingButton) {
    greetingButton.addEventListener('click', async function() {
      if (!ensureDeviceBound()) return;
      var statusText = document.getElementById('greetingStatusText');
      greetingButton.disabled = true;
      greetingButton.classList.add('opacity-70');
      if (statusText) statusText.textContent = '正在触发问候...';
      try {
        var greeting = await anbanClient.triggerGreeting({ deviceId: anbanConfig.deviceId, tonePreset: 'warm' });
        var result = formatGreetingTriggerResult(greeting);
        if (statusText) statusText.textContent = result.detail;
        showToast(result.notice);
      } catch (error) {
        if (statusText) statusText.textContent = '问候触发失败';
        showToast(error.message || '问候触发失败');
      } finally {
        greetingButton.disabled = false;
        greetingButton.classList.remove('opacity-70');
      }
    });
  }

  var visionButton = document.getElementById('visionLookButton');
  if (visionButton) {
    visionButton.addEventListener('click', startVisionLook);
  }

  // Quick Msg Modal
  window.openQuickMsg = function() {
    document.getElementById('quickMsgOverlay').classList.add('open');
    document.getElementById('quickMsgCard').classList.add('open');
    document.getElementById('quickMsgInput').focus();
  };
  window.closeQuickMsg = function() {
    document.getElementById('quickMsgOverlay').classList.remove('open');
    document.getElementById('quickMsgCard').classList.remove('open');
  };
  window.sendQuickMsg = async function() {
    var input = document.getElementById('quickMsgInput');
    var text = input.value.trim();
    if (!text) return;
    try {
      await sendChildMessage(text);
      input.value = '';
      closeQuickMsg();
      showToast('留言已提交');
      window.refreshHome();
    } catch (error) {
      showToast(error.message || '留言发送失败');
    }
  };
  document.getElementById('quickMsgSend').addEventListener('click', window.sendQuickMsg);
  document.getElementById('quickMsgInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') window.sendQuickMsg();
  });

  // Bottom Sheet
  window.openSheet = function() {
    document.getElementById('bottomSheet').classList.add('open');
    document.getElementById('sheetOverlay').classList.add('open');
    document.getElementById('sheetInput').focus();
  };
  window.closeSheet = function() {
    document.getElementById('bottomSheet').classList.remove('open');
    document.getElementById('sheetOverlay').classList.remove('open');
  };
  window.fillTemplate = function(text) {
    document.getElementById('sheetInput').value = text;
    document.getElementById('sheetInput').focus();
  };
  window.sendFromSheet = async function() {
    var input = document.getElementById('sheetInput');
    var text = input.value.trim();
    if (!text) return;
    try {
      await sendChildMessage(text);
      input.value = '';
      closeSheet();
      showToast('留言已提交');
      window.refreshHome();
    } catch (error) {
      showToast(error.message || '留言发送失败');
    }
  };
  // Reminder time picker (in bottom sheet)
  var reminderHour = 8;
  var reminderMinute = 0;
  function buildReminderTimePicker() {
    var hList = document.getElementById('reminderHourList');
    var mList = document.getElementById('reminderMinuteList');
    if (!hList || hList.children.length) return;
    for (var h = 0; h < 24; h++) {
      var div = document.createElement('div');
      div.className = 'time-col-item' + (h === reminderHour ? ' selected' : '');
      div.textContent = h.toString().padStart(2, '0');
      div.setAttribute('data-h', h);
      div.onclick = function() { selectReminderHour(parseInt(this.getAttribute('data-h'))); };
      hList.appendChild(div);
    }
    for (var m = 0; m < 60; m++) {
      var div = document.createElement('div');
      div.className = 'time-col-item' + (m === reminderMinute ? ' selected' : '');
      div.textContent = m.toString().padStart(2, '0');
      div.setAttribute('data-m', m);
      div.onclick = function() { selectReminderMinute(parseInt(this.getAttribute('data-m'))); };
      mList.appendChild(div);
    }
  }
  function selectReminderHour(h) {
    reminderHour = h;
    var items = document.querySelectorAll('#reminderHourList .time-col-item');
    for (var i = 0; i < items.length; i++) items[i].classList.remove('selected');
    items[h].classList.add('selected');
    items[h].scrollIntoView({ block: 'center' });
    updateReminderTimeDisplay();
  }
  function selectReminderMinute(m) {
    reminderMinute = m;
    var items = document.querySelectorAll('#reminderMinuteList .time-col-item');
    for (var i = 0; i < items.length; i++) items[i].classList.remove('selected');
    items[m].classList.add('selected');
    items[m].scrollIntoView({ block: 'center' });
    updateReminderTimeDisplay();
  }
  function updateReminderTimeDisplay() {
    document.getElementById('reminderTimeDisplay').textContent = reminderHour.toString().padStart(2,'0') + ':' + reminderMinute.toString().padStart(2,'0');
  }
  window.toggleReminderTimePicker = function() {
    document.getElementById('reminderTimePickerPanel').classList.toggle('show');
  };
  window.saveQuickReminder = async function() {
    var input = document.getElementById('reminderSheetInput');
    var text = input.value.trim();
    if (!text) return;
    var freq = document.getElementById('sheetFreqDisplay').textContent;
    var scheduleOptions = buildReminderScheduleOptions(freq, []);
    try {
      await anbanClient.createReminder({
        deviceId: anbanConfig.deviceId,
        scheduledAt: nextOccurrenceUTC(reminderHour, reminderMinute),
        content: text,
        category: text.includes('药') ? 'med' : 'custom',
        recurrence: scheduleOptions.recurrence,
        customDates: scheduleOptions.customDates,
        important: false,
      });
      input.value = '';
      closeSheet();
      showToast('提醒已保存');
      if (typeof window.refreshReminders === 'function') window.refreshReminders();
    } catch (error) {
      showToast(error.message || '提醒保存失败');
    }
  };
  window.selectSheetFreq = function(el) {
    var items = document.querySelectorAll('#sheetFreqPickerPanel .freq-dropdown-item');
    for (var i = 0; i < items.length; i++) items[i].classList.remove('selected');
    el.classList.add('selected');
    document.getElementById('sheetFreqDisplay').textContent = el.textContent;
    document.getElementById('sheetFreqPickerPanel').classList.remove('show');
  };
  window.toggleSheetFreqPicker = function() {
    document.getElementById('sheetFreqPickerPanel').classList.toggle('show');
  };
  buildReminderTimePicker();

}

window.initHistory = async function() {
  var list = document.getElementById('historyList');
  var empty = document.getElementById('historyEmpty');
  if (!list || !empty) return;
  list.innerHTML = '';

  try {
    var payload = await anbanClient.listReminders({ deviceId: anbanConfig.deviceId });
    var records = (payload.reminders || []).filter(function(reminder) { return reminder.status !== 'scheduled'; });
    empty.style.display = records.length ? 'none' : '';
    records.forEach(function(reminder) {
      var row = document.createElement('div');
      row.className = 'flex items-center gap-4 py-4 border-b border-divider-warm/30';
      row.innerHTML = '<span class="material-symbols-outlined text-success text-[20px]">check_circle</span><div class="flex-1 min-w-0"><p class="font-body-md text-body-md text-on-surface truncate"></p><p class="font-label-sm text-label-sm text-text-secondary"></p></div><span class="font-label-sm text-label-sm text-on-surface-variant"></span>';
      row.querySelector('p').textContent = reminder.content || reminder.text || '提醒';
      row.querySelectorAll('p')[1].textContent = formatRelativeTime(reminder.playedAt || reminder.scheduledAt);
      row.lastElementChild.textContent = reminder.status === 'played' ? '已提醒' : reminder.status === 'failed' ? '提醒失败' : reminder.status === 'cancelled' ? '已取消' : reminder.status;
      list.appendChild(row);
    });
  } catch (error) {
    empty.style.display = '';
    showToast(error.message || '历史提醒加载失败');
  }
};

// ============================
// Reminder Detail / Edit / Delete
// ============================
var _detailCard = null; // reference to the card being edited
var _detailEditTarget;

window.openDetail = function(card) {
  _detailCard = card;
  navigateTo('detail');
};

window.initDetail = function() {
  var card = _detailCard;
  if (!card) { navigateTo('warn'); return; }

  // Read data from card attributes
  var name = card.getAttribute('data-name') || '';
  var time = card.getAttribute('data-time') || '08:30';
  var freq = card.getAttribute('data-freq') || '每天';
  var note = card.getAttribute('data-note') || '';
  var icon = card.getAttribute('data-icon') || 'notifications';
  var iconColor = card.getAttribute('data-iconcolor') || 'primary';
  var isImportant = card.getAttribute('data-important') === '1';
  var isPaused = card.classList.contains('reminder-paused');

  // Fill detail page
  var nameEl = document.getElementById('detailName');
  var timeEl = document.getElementById('detailTime');
  var freqEl = document.getElementById('detailFreq');
  var noteEl = document.getElementById('detailNote');
  var iconEl = document.getElementById('detailIcon');
  var iconWrap = document.getElementById('detailIconWrap');
  var tagsEl = document.getElementById('detailTags');
  var impToggle = document.getElementById('detailImportantToggle');

  if (nameEl) nameEl.textContent = name;
  if (timeEl) timeEl.textContent = time;
  if (freqEl) freqEl.textContent = freq;
  if (noteEl) noteEl.value = note.replace(/^备注：/, '');

  // Icon
  if (iconEl) {
    iconEl.textContent = icon;
    iconEl.className = 'material-symbols-outlined text-' + iconColor + ' text-[24px]';
    if (iconColor === 'warning') iconEl.style.fontVariationSettings = "'FILL' 1";
    else iconEl.style.fontVariationSettings = '';
  }
  if (iconWrap) {
    iconWrap.className = 'w-12 h-12 rounded-full flex items-center justify-center flex-shrink-0';
    if (iconColor === 'warning') iconWrap.classList.add('bg-warning/10');
    else if (iconColor === 'tertiary') iconWrap.classList.add('bg-tertiary-container/10');
    else if (iconColor === 'on-secondary-container') iconWrap.classList.add('bg-secondary-container/30');
    else if (iconColor === 'on-surface-variant') iconWrap.classList.add('bg-on-surface-variant/10');
    else iconWrap.classList.add('bg-primary-container/10');
  }

  // Tags
  var tagsHtml = '<span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold">' + freq + '</span>';
  if (isImportant) tagsHtml += ' <span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-0.5"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span>';
  if (isPaused) tagsHtml += ' <span class="bg-on-surface-variant/10 text-on-surface-variant text-[10px] px-2 py-0.5 rounded-full font-bold">已暂停</span>';
  if (tagsEl) tagsEl.innerHTML = tagsHtml;

  // Important toggle
  if (impToggle) {
    if (isImportant) { impToggle.classList.add('on'); impToggle.classList.remove('off'); }
    else { impToggle.classList.remove('on'); impToggle.classList.add('off'); }
  }
};

window.editDetailTime = function() {
  return notImplemented('编辑提醒');
  var timeEl = document.getElementById('detailTime');
  if (!timeEl) return;
  var timeText = timeEl.textContent.trim();
  var parts = timeText.split(':');
  var h = parseInt(parts[0]) || 8;
  var m = parseInt(parts[1]) || 30;
  _detailEditTarget = 'detail';
  if (typeof window.setTimePickerValues === 'function') window.setTimePickerValues(h, m);
  openTimePickerModal();
};

window.editDetailFreq = function() {
  var freqEl = document.getElementById('detailFreq');
  if (!freqEl) return;
  _detailEditTarget = 'detail';
  var currentFreq = freqEl.textContent.trim();
  var options = document.querySelectorAll('.freq-option');
  for (var i = 0; i < options.length; i++) {
    if (options[i].getAttribute('data-value') === currentFreq) {
      options[i].classList.add('selected');
    } else {
      options[i].classList.remove('selected');
    }
  }
  openFreqPickerModal();
};

window.toggleDetailImportant = function() {
  var toggle = document.getElementById('detailImportantToggle');
  if (!toggle) return;
  toggle.classList.toggle('on');
  toggle.classList.toggle('off');
  // Update tags
  var tagsEl = document.getElementById('detailTags');
  if (tagsEl) {
    var freqEl = document.getElementById('detailFreq');
    var isImportant = toggle.classList.contains('on');
    var html = '<span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold">' + (freqEl ? freqEl.textContent : '每天') + '</span>';
    if (isImportant) html += ' <span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-0.5"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span>';
    tagsEl.innerHTML = html;
  }
};

window.saveDetailReminder = function() {
  return notImplemented('编辑提醒');
  var card = _detailCard;
  if (!card) return;

  var timeEl = document.getElementById('detailTime');
  var freqEl = document.getElementById('detailFreq');
  var noteEl = document.getElementById('detailNote');
  var impToggle = document.getElementById('detailImportantToggle');

  var time = timeEl ? timeEl.textContent.trim() : '08:30';
  var freq = freqEl ? freqEl.textContent.trim() : '每天';
  var note = noteEl ? noteEl.value.trim() : '';
  var isImportant = impToggle && impToggle.classList.contains('on');
  var name = card.getAttribute('data-name') || '';
  var icon = card.getAttribute('data-icon') || 'notifications';
  var iconColor = isImportant ? 'warning' : (card.getAttribute('data-iconcolor') || 'primary');

  // Update card data attributes
  card.setAttribute('data-time', time);
  card.setAttribute('data-freq', freq);
  card.setAttribute('data-note', note);
  card.setAttribute('data-important', isImportant ? '1' : '0');
  card.setAttribute('data-iconcolor', iconColor);

  // Rebuild card innerHTML
  var importantTag = isImportant ? '<span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-0.5"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span>' : '';
  var isPaused = card.classList.contains('reminder-paused');
  var toggleState = isPaused ? 'off' : 'on';

  var iconStyle = isImportant ? ' style="font-variation-settings:\'FILL\' 1"' : '';
  card.innerHTML = '<div class="flex gap-4 cursor-pointer active:opacity-80 transition-opacity" onclick="openDetail(this.parentElement)"><div class="w-12 h-12 rounded-full flex items-center justify-center flex-shrink-0 ' + (isImportant ? 'bg-warning/10' : 'bg-primary-container/10') + '"><span class="material-symbols-outlined text-' + iconColor + '"' + iconStyle + '>' + icon + '</span></div><div><div class="flex items-center gap-2 mb-1"><h4 class="font-bold text-body-lg text-body-lg text-on-surface">' + name + '</h4><span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold">' + freq + '</span>' + importantTag + '</div><p class="font-bold text-title-lg text-title-lg text-on-surface mb-1">' + time + '</p><p class="font-label-sm text-label-sm text-text-secondary">备注：' + (note || '无') + '</p></div></div><div class="toggle-track ' + toggleState + ' flex-shrink-0" onclick="event.stopPropagation();handleToggle(this)"><div class="toggle-thumb"></div></div>';

  // Show toast
  var toast = document.getElementById('spaToast');
  if (toast) { toast.textContent = '已保存修改'; toast.classList.add('show'); setTimeout(function(){ toast.classList.remove('show'); }, 2000); }
  navigateTo('warn');
};

window.confirmDeleteReminder = function() {
  var overlay = document.getElementById('deleteConfirmOverlay');
  var card2 = document.getElementById('deleteConfirmCard');
  if (overlay) overlay.classList.add('open');
  if (card2) card2.classList.add('open');
};

window.closeDeleteConfirm = function() {
  var overlay = document.getElementById('deleteConfirmOverlay');
  var card2 = document.getElementById('deleteConfirmCard');
  if (overlay) overlay.classList.remove('open');
  if (card2) card2.classList.remove('open');
};

window.doDeleteReminder = async function() {
  var card = _detailCard;
  var reminderId = card && card.getAttribute('data-reminder-id');
  if (!reminderId) return;
  try {
    await anbanClient.deleteReminder(reminderId);
    _detailCard = null;
    closeDeleteConfirm();
    navigateTo('warn');
    showToast('提醒已删除');
  } catch (error) {
    showToast(error.message || '提醒删除失败');
  }
};

window.toggleImportant = function(el) {
  if (!el) return;
  el.classList.toggle('on');
  el.classList.toggle('off');
};


// ============================
// initMessage
// ============================
function initMessage() {
  function renderTimeline(payload) {
    var chatArea = document.getElementById('chatArea');
    if (!chatArea) return;
    chatArea.innerHTML = '';
    var items = payload && Array.isArray(payload.items) ? payload.items : [];

    if (!items.length) {
      var empty = document.createElement('div');
      empty.style.cssText = 'text-align:center;padding:48px 0;color:var(--ab-ink3);font-size:13px';
      empty.textContent = '暂无对话记录';
      chatArea.appendChild(empty);
    }

    items.forEach(function(item) {
      var row = document.createElement('div');
      var isRight = item.type === 'child_message';
      row.className = 'flex flex-col';
      row.style.cssText = 'max-width:85%;' + (isRight ? 'align-self:flex-end;align-items:flex-end' : 'align-self:flex-start;align-items:flex-start');
      row.innerHTML = '<div style="font-size:11px;color:var(--ab-ink3);margin-bottom:4px;padding:0 2px"></div><div style="padding:11px 13px;font-size:13px;line-height:1.5"><p style="margin:0"></p></div><div style="display:flex;align-items:center;gap:6px;margin-top:4px;padding:0 2px"><span style="font-size:10px;color:var(--ab-ink3)"></span><span style="font-size:10px;font-weight:600"></span></div>';
      row.firstElementChild.textContent = item.sourceLabel || (isRight ? '家人' : '安伴');
      if (isRight && item.avatarColor) row.firstElementChild.style.color = item.avatarColor;
      var bubbleEl = row.children[1];
      if (isRight) {
        bubbleEl.style.background = 'var(--ab-primary)';
        bubbleEl.style.color = '#fff';
        bubbleEl.style.borderRadius = '12px 12px 3px 12px';
      } else {
        bubbleEl.style.background = '#fff';
        bubbleEl.style.border = '1px solid var(--ab-line)';
        bubbleEl.style.color = 'var(--ab-ink)';
        bubbleEl.style.borderRadius = '12px 12px 12px 3px';
      }
      bubbleEl.querySelector('p').textContent = item.text;
      var meta = row.lastElementChild;
      meta.firstElementChild.textContent = formatRelativeTime(item.at);
      var status = meta.lastElementChild;
      var statusLabels = { played: '已播报', pending: '待播报', failed: '发送失败' };
      status.textContent = statusLabels[item.status] || '';
      status.style.color = item.status === 'played' ? 'var(--ab-ok)' : item.status === 'failed' ? '#d9534f' : 'var(--ab-ink3)';
      if (!item.status) status.remove();
      chatArea.appendChild(row);
    });

    var container = document.getElementById('messagesContainer');
    requestAnimationFrame(function() {
      if (container) container.scrollTop = container.scrollHeight;
    });
  }

  async function loadMessages() {
    if (!isDeviceBound()) {
      renderTimeline({ items: [] });
      return;
    }
    try {
      var payload = await anbanClient.getTimeline({
        deviceId: anbanConfig.deviceId,
        elderDisplayName: anbanSession.binding && anbanSession.binding.elderDisplayName,
        limit: 100,
      });
      renderTimeline(payload);
    } catch (error) {
      renderTimeline({ items: [] });
      showToast(error.message || '消息加载失败');
    }
  }

  window.refreshMessages = loadMessages;

  window.handleSend = async function() {
    var input = document.getElementById('messageInput');
    var text = input.value.trim();
    if (!text) return;
    try {
      var result = await sendChildMessage(text);
      input.value = '';
      input.style.height = '48px';
      showToast(result.status === 'played' ? '留言已播报' : '留言已提交');
      await loadMessages();
      if (typeof window.refreshHome === 'function') window.refreshHome();
    } catch (error) {
      showToast(error.message || '留言发送失败');
    }
  };

  var tx = document.getElementById('messageInput');
  if (tx) {
    tx.addEventListener('input', function() {
      this.style.height = 'auto';
      this.style.height = (this.scrollHeight) + 'px';
    });
  }

}

// ============================
// initWarn
// ============================
function initWarn() {
  var selectedHour = 8, selectedMinute = 0;

  function buildTimePicker() {
    var hL = document.getElementById('hourList');
    var mL = document.getElementById('minuteList');
    if (!hL || hL.children.length > 1) return;
    for (var h = 0; h < 24; h++) {
      var d = document.createElement('div');
      d.className = 'tp-col-item' + (h === selectedHour ? ' selected' : '');
      d.textContent = h.toString().padStart(2, '0');
      d.setAttribute('data-h', h);
      d.onclick = function() { selectHour(parseInt(this.getAttribute('data-h'))); };
      hL.appendChild(d);
    }
    for (var m = 0; m < 60; m++) {
      var d = document.createElement('div');
      d.className = 'tp-col-item' + (m === selectedMinute ? ' selected' : '');
      d.textContent = m.toString().padStart(2, '0');
      d.setAttribute('data-m', m);
      d.onclick = function() { selectMinute(parseInt(this.getAttribute('data-m'))); };
      mL.appendChild(d);
    }
  }

  function scrollToSelected(colId, index, itemH) {
    var colEl = document.getElementById(colId);
    if (colEl) {
      var scrollTop = index * itemH;
      colEl.scrollTop = scrollTop;
    }
  }

  function selectHour(h) {
    selectedHour = h;
    var items = document.querySelectorAll('#hourList .tp-col-item');
    for (var i = 0; i < items.length; i++) items[i].classList.remove('selected');
    items[h].classList.add('selected');
    scrollToSelected('tpHourCol', h, 44);
    updateTpDisplay();
  }
  function selectMinute(m) {
    selectedMinute = m;
    var items = document.querySelectorAll('#minuteList .tp-col-item');
    for (var i = 0; i < items.length; i++) items[i].classList.remove('selected');
    items[m].classList.add('selected');
    scrollToSelected('tpMinCol', m, 44);
    updateTpDisplay();
  }
  function updateTpDisplay() {
    var hEl = document.getElementById('tpHourDisplay');
    var mEl = document.getElementById('tpMinDisplay');
    if (hEl) hEl.textContent = selectedHour.toString().padStart(2,'0');
    if (mEl) mEl.textContent = selectedMinute.toString().padStart(2,'0');
  }
  window.openTimePickerModal = function() {
    var fpo = document.getElementById('freqPickerOverlay');
    if (fpo) fpo.classList.remove('open');
    document.getElementById('timePickerOverlay').classList.add('open');
    document.getElementById('timePickerModal').classList.add('open');
    updateTpDisplay();
    setTimeout(function() {
      scrollToSelected('tpHourCol', selectedHour, 44);
      scrollToSelected('tpMinCol', selectedMinute, 44);
    }, 100);
  };
  window.closeTimePickerModal = function() {
    document.getElementById('timePickerOverlay').classList.remove('open');
    document.getElementById('timePickerModal').classList.remove('open');
  };
  window.confirmTimePicker = function() {
    var timeStr = selectedHour.toString().padStart(2,'0') + ':' + selectedMinute.toString().padStart(2,'0');
    if (typeof _detailEditTarget !== 'undefined' && _detailEditTarget === 'detail') {
      var detailTimeEl = document.getElementById('detailTime');
      if (detailTimeEl) detailTimeEl.textContent = timeStr;
      _detailEditTarget = undefined;
    } else {
      document.getElementById('timeDisplay').textContent = timeStr;
    }
    closeTimePickerModal();
  };
  var selectedFreq = '仅一次';
  var customDates = [];
  var calYear, calMonth;

  function initCalDate() {
    var now = new Date();
    calYear = now.getFullYear();
    calMonth = now.getMonth();
  }

  function freqCalRender() {
    var grid = document.getElementById('freqCalGrid');
    var monthLabel = document.getElementById('freqCalMonth');
    if (!grid || !monthLabel) return;
    monthLabel.textContent = calYear + '年' + (calMonth + 1) + '月';
    var firstDay = new Date(calYear, calMonth, 1).getDay();
    var daysInMonth = new Date(calYear, calMonth + 1, 0).getDate();
    var today = new Date(); today.setHours(0,0,0,0);
    var html = '';
    // Monday=0 offset
    var startOffset = (firstDay + 6) % 7;
    for (var i = 0; i < startOffset; i++) {
      html += '<div class="freq-cal-day empty"></div>';
    }
    for (var d = 1; d <= daysInMonth; d++) {
      var dateObj = new Date(calYear, calMonth, d); dateObj.setHours(0,0,0,0);
      var dateStr = calYear + '-' + String(calMonth+1).padStart(2,'0') + '-' + String(d).padStart(2,'0');
      var cls = 'freq-cal-day';
      if (dateObj < today) cls += ' past';
      else {
        if (dateObj.getTime() === today.getTime()) cls += ' today';
        if (customDates.indexOf(dateStr) >= 0) cls += ' selected';
      }
      html += '<div class="' + cls + '" data-date="' + dateStr + '" onclick="toggleCalDay(this)">' + d + '</div>';
    }
    grid.innerHTML = html;
    updateCalHint();
  }

  function updateCalHint() {
    var hint = document.getElementById('freqCalHint');
    if (!hint) return;
    if (customDates.length === 0) {
      hint.textContent = '';
    } else if (customDates.length <= 2) {
      var parts = customDates.map(function(ds) {
        var p = ds.split('-');
        return parseInt(p[1]) + '月' + parseInt(p[2]) + '日';
      });
      hint.textContent = '已选：' + parts.join('、');
    } else {
      var first = customDates[0].split('-');
      hint.textContent = '已选：' + parseInt(first[1]) + '月' + parseInt(first[2]) + '日 等' + customDates.length + '天';
    }
  }

  window.toggleCalDay = function(el) {
    var dateStr = el.getAttribute('data-date');
    var idx = customDates.indexOf(dateStr);
    if (idx >= 0) {
      customDates.splice(idx, 1);
      el.classList.remove('selected');
    } else {
      customDates.push(dateStr);
      el.classList.add('selected');
    }
    customDates.sort();
    updateCalHint();
  };

  window.freqCalPrev = function() {
    calMonth--;
    if (calMonth < 0) { calMonth = 11; calYear--; }
    freqCalRender();
  };

  window.freqCalNext = function() {
    calMonth++;
    if (calMonth > 11) { calMonth = 0; calYear++; }
    freqCalRender();
  };

  function formatCustomLabel() {
    if (customDates.length === 0) return '自定义';
    if (customDates.length <= 2) {
      return customDates.map(function(ds) {
        var p = ds.split('-');
        return parseInt(p[1]) + '月' + parseInt(p[2]) + '日';
      }).join('、');
    }
    var first = customDates[0].split('-');
    return parseInt(first[1]) + '月' + parseInt(first[2]) + '日 等' + customDates.length + '天';
  }

  window.openFreqPickerModal = function() {
    closeTimePickerModal();
    document.getElementById('freqPickerOverlay').classList.add('open');
    document.getElementById('freqPickerModal').classList.add('open');
    var currentVal = document.getElementById('freqDisplay').textContent;
    var options = document.querySelectorAll('#freqPickerOptions .freq-option');
    var matchedPreset = false;
    for (var i = 0; i < options.length; i++) {
      var val = options[i].getAttribute('data-value');
      if (val === currentVal) {
        options[i].classList.add('selected');
        matchedPreset = true;
      } else {
        options[i].classList.remove('selected');
      }
    }
    if (matchedPreset) {
      selectedFreq = currentVal;
      document.getElementById('freqCalendarWrap').classList.remove('show');
    } else {
      selectedFreq = '自定义';
      for (var j = 0; j < options.length; j++) {
        if (options[j].getAttribute('data-value') === '自定义') options[j].classList.add('selected');
        else options[j].classList.remove('selected');
      }
      document.getElementById('freqCalendarWrap').classList.add('show');
      initCalDate();
      freqCalRender();
    }
  };
  
  window.closeFreqPickerModal = function() {
    document.getElementById('freqPickerOverlay').classList.remove('open');
    document.getElementById('freqPickerModal').classList.remove('open');
    document.getElementById('freqCalendarWrap').classList.remove('show');
  };
  
  window.selectFreqOption = function(el) {
    var options = document.querySelectorAll('#freqPickerOptions .freq-option');
    for (var i = 0; i < options.length; i++) options[i].classList.remove('selected');
    el.classList.add('selected');
    selectedFreq = el.getAttribute('data-value');
    if (selectedFreq === '自定义') {
      customDates = [];
      document.getElementById('freqCalendarWrap').classList.add('show');
      initCalDate();
      freqCalRender();
    } else {
      document.getElementById('freqCalendarWrap').classList.remove('show');
    }
  };
  
  window.confirmFreqPicker = function() {
    var freqText;
    if (selectedFreq === '自定义') {
      if (customDates.length === 0) {
        var hint = document.getElementById('freqCalHint');
        if (hint) { hint.textContent = '请至少选择一个日期'; hint.style.color = '#e74c3c'; }
        return;
      }
      freqText = formatCustomLabel();
    } else {
      freqText = selectedFreq;
    }
    if (typeof _detailEditTarget !== 'undefined' && _detailEditTarget === 'detail') {
      var detailFreqEl = document.getElementById('detailFreq');
      if (detailFreqEl) detailFreqEl.textContent = freqText;
      // Sync detailTags
      var tagsEl = document.getElementById('detailTags');
      var impToggle = document.getElementById('detailImportantToggle');
      if (tagsEl) {
        var isImportant = impToggle && impToggle.classList.contains('on');
        var html = '<span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold">' + freqText + '</span>';
        if (isImportant) html += ' <span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-0.5"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span>';
        tagsEl.innerHTML = html;
      }
      _detailEditTarget = undefined;
    } else {
      document.getElementById('freqDisplay').textContent = freqText;
    }
    closeFreqPickerModal();
  };

  var content = document.getElementById('reminderContent');
  var counter = document.getElementById('charCount');
  if (content && counter) {
    content.addEventListener('input', function() { counter.textContent = this.value.length + '/20'; });
  }

  window.updateCount = function() {
    // count display removed, kept for compat
  };

  window.handleToggle = function(el) {
    return notImplemented('暂停提醒');
  };




  window.saveReminder = async function() {
    var input = document.getElementById('reminderContent');
    var text = input.value.trim();
    if (!text) {
      input.style.borderColor = '#F78C6B';
      input.style.animation = 'shake 0.4s ease';
      input.setAttribute('placeholder', '请输入提醒内容');
      setTimeout(function() { input.style.borderColor = ''; input.style.animation = ''; }, 800);
      return;
    }
    var freq = document.getElementById('freqDisplay').textContent;
    var importantToggle = document.getElementById('importantToggle');
    var isImportant = importantToggle && importantToggle.classList.contains('on');
    var scheduleOptions = buildReminderScheduleOptions(freq, customDates);
    var timeParts = document.getElementById('timeDisplay').textContent.split(':');
    try {
      await anbanClient.createReminder({
        deviceId: anbanConfig.deviceId,
        scheduledAt: nextOccurrenceUTC(Number(timeParts[0]), Number(timeParts[1])),
        content: text,
        category: text.includes('药') ? 'med' : 'custom',
        recurrence: scheduleOptions.recurrence,
        customDates: scheduleOptions.customDates,
        important: isImportant,
      });
      input.value = '';
      input.setAttribute('placeholder', '例如：吃早饭、吃降压药、喝水等');
      if (counter) counter.textContent = '0/20';
      showToast('提醒已保存');
      await loadSavedReminders();
    } catch (error) {
      showToast(error.message || '提醒保存失败');
    }
  };

  function createReminderCard(reminder) {
    var scheduled = new Date(reminder.scheduledAt);
    var time = Number.isNaN(scheduled.getTime()) ? '--:--' : scheduled.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
    var text = reminder.content || reminder.text || '提醒';
    var freqLabel = reminderFrequencyLabel(reminder);
    var isImportant = reminder.important === true;
    var div = document.createElement('div');
    div.className = 'bg-surface-white rounded-2xl p-5 soft-shadow flex items-center justify-between';
    div.setAttribute('data-reminder-id', reminder.reminderId || '');
    div.setAttribute('data-important', isImportant ? '1' : '0');
    div.setAttribute('data-name', text);
    div.setAttribute('data-time', time);
    div.setAttribute('data-freq', freqLabel);
    div.setAttribute('data-note', reminder.status || 'scheduled');
    div.setAttribute('data-icon', reminder.category === 'med' ? 'medication' : 'notifications');
    div.setAttribute('data-iconcolor', 'primary');
    div.innerHTML = '<div class="flex gap-4 cursor-pointer active:opacity-80 transition-opacity" onclick="openDetail(this.parentElement)"><div class="w-12 h-12 rounded-full bg-primary-container/10 flex items-center justify-center flex-shrink-0"><span class="material-symbols-outlined text-primary"></span></div><div><div class="flex items-center gap-2 mb-1"><h4 class="font-bold text-body-lg text-body-lg text-on-surface"></h4><span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold reminder-freq"></span><span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold items-center gap-0.5 reminder-important" style="display:none"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span></div><p class="font-bold text-title-lg text-title-lg text-on-surface mb-1"></p><p class="font-label-sm text-label-sm text-text-secondary"></p></div></div><div class="toggle-track on flex-shrink-0" onclick="event.stopPropagation();handleToggle(this)"><div class="toggle-thumb"></div></div>';
    div.querySelector('.material-symbols-outlined').textContent = reminder.category === 'med' ? 'medication' : 'notifications';
    div.querySelector('h4').textContent = text;
    div.querySelector('.reminder-freq').textContent = freqLabel;
    if (isImportant) {
      var importantTag = div.querySelector('.reminder-important');
      importantTag.style.display = 'inline-flex';
    }
    div.querySelector('p.font-bold').textContent = time;
    div.querySelector('p.font-label-sm').textContent = formatRelativeTime(reminder.scheduledAt);
    return div;
  }

  function reminderFrequencyLabel(reminder) {
    if (reminder.recurrence === 'daily') return '每天';
    if (reminder.recurrence === 'weekdays') return '工作日';
    if (reminder.recurrence === 'weekends') return '周末';
    if (reminder.recurrence === 'custom-dates') {
      var dates = Array.isArray(reminder.customDates) ? reminder.customDates : [];
      return dates.length ? dates[0].slice(5).replace('-', '月') + '日 等' + dates.length + '天' : '自定义';
    }
    return '仅一次';
  }

  async function loadSavedReminders() {
    var list = document.getElementById('reminderList');
    try {
      var payload = await anbanClient.listReminders({ deviceId: anbanConfig.deviceId, status: 'scheduled' });
      var reminders = payload.reminders || [];
      list.innerHTML = '';
      if (!reminders.length) {
        var empty = document.createElement('div');
        empty.className = 'bg-surface-white rounded-2xl p-6 text-center text-text-secondary';
        empty.textContent = '暂无待执行提醒';
        list.appendChild(empty);
      }
      for (var i = 0; i < reminders.length; i++) {
        list.appendChild(createReminderCard(reminders[i]));
      }
    } catch (error) {
      list.innerHTML = '<div class="bg-surface-white rounded-2xl p-6 text-center text-text-secondary">提醒加载失败</div>';
    }
  }

  window.refreshReminders = loadSavedReminders;

  window.setTimePickerValues = function(h, m) {
    selectedHour = h;
    selectedMinute = m;
    var hEl = document.getElementById('tpHourDisplay');
    var mEl = document.getElementById('tpMinDisplay');
    if (hEl) hEl.textContent = h.toString().padStart(2,'0');
    if (mEl) mEl.textContent = m.toString().padStart(2,'0');
    var hItems = document.querySelectorAll('#hourList .tp-col-item');
    for (var i = 0; i < hItems.length; i++) { hItems[i].classList.toggle('selected', parseInt(hItems[i].textContent) === h); }
    var mItems = document.querySelectorAll('#minuteList .tp-col-item');
    for (var j = 0; j < mItems.length; j++) { mItems[j].classList.toggle('selected', parseInt(mItems[j].textContent) === m); }
  };

  buildTimePicker();
}

// ============================
// initFamily
// ============================
async function initFamily() {
  var defaultProfile = {
    name: '妈妈', age: 72, livingSituation: '与爸爸同住', occupation: '退休教师',
    aiPortrait: '', aiPortraitMode: 'auto',
    hobbies: ['园艺', '广场舞', '京剧', '养花', '做手工'],
    habits: [
      { icon: 'wb_twilight', text: '晨间 6:30 起床，作息规律' },
      { icon: 'local_cafe', text: '饭后必喝一杯龙井，已养成多年习惯' },
      { icon: 'music_note', text: '下午固定收听京剧频道' },
      { icon: 'directions_walk', text: '晚餐后在小区散步 30 分钟' }
    ],
    health: [
      { name: '降压药管理', detail: '每日 08:30 服用，饭后半小时温水送服。早晚各测一次血压并记录。' },
      { name: '注意事项', detail: '避免高盐饮食，保持情绪稳定。如血压超过 150/95 需及时联系医生。' }
    ],
    communicationDos: ['多聊聊她年轻时的教学故事，她会很开心', '每周至少视频通话两次，她会很期待'],
    communicationDonts: ['尽量避免在深夜提及已故的外公，以免引起情绪低落']
  };

  var localStored = localStorage.getItem('anban_family_profile_local');
  var localProfile = localStored ? JSON.parse(localStored) : {};
  var profile;
  try {
    var backendProfile = await anbanClient.getProfile({ deviceId: anbanConfig.deviceId });
    profile = mapFieldsToStitchProfile(backendProfile.fields || {}, localProfile);
  } catch (error) {
    var legacyStored = localStorage.getItem('anban_family_profile');
    profile = legacyStored ? JSON.parse(legacyStored) : { ...defaultProfile, ...localProfile };
  }

  document.getElementById('profileName').textContent = profile.name || '妈妈';
  document.getElementById('profileSubtitle').innerHTML = (profile.age || 72) + '岁 · ' + (profile.livingSituation || '与爸爸同住') + ' · ' + (profile.occupation || '退休教师');
  document.getElementById('aiPortraitText').textContent = profile.aiPortrait || '画像会在资料和专属记忆积累后自动形成。';
  document.getElementById('aiPortraitModeText').textContent = profile.aiPortraitMode === 'manual' ? '管理员维护' : 'AI 自动更新';
  var isAdmin = !isAccountMode() || (anbanSession.binding && anbanSession.binding.role === 'admin');

  var hobbyContainer = document.getElementById('hobbyTags');
  if (hobbyContainer) {
    hobbyContainer.innerHTML = '';
    (profile.hobbies || defaultProfile.hobbies).forEach(function(h) {
      var tag = document.createElement('span');
      tag.className = 'bg-secondary-container/40 text-on-secondary-container px-3 py-1.5 rounded-full font-body-md text-body-md';
      tag.textContent = h;
      hobbyContainer.appendChild(tag);
    });
  }

  var habitContainer = document.getElementById('habitItems');
  if (habitContainer) {
    habitContainer.innerHTML = '';
    (profile.habits || defaultProfile.habits).forEach(function(h) {
      var item = document.createElement('div');
      item.className = 'flex items-center gap-2.5 text-body-md text-body-md text-on-surface-variant';
      item.innerHTML = '<span class="material-symbols-outlined text-primary text-[18px]">' + h.icon + '</span> ' + h.text;
      habitContainer.appendChild(item);
    });
  }

  var healthContainer = document.getElementById('healthItems');
  if (healthContainer) {
    healthContainer.innerHTML = '';
    (profile.health || defaultProfile.health).forEach(function(h, i) {
      var div = document.createElement('div');
      div.className = 'flex items-start gap-3' + (i > 0 ? ' mt-3' : '');
      var icon = (i === 0) ? 'medication' : 'warning';
      div.innerHTML = '<span class="material-symbols-outlined text-on-secondary-container text-[20px] flex-shrink-0 mt-0.5">' + icon + '</span>' +
        '<div><p class="font-body-md text-body-md text-on-secondary-container font-medium">' + h.name + '</p>' +
        '<p class="font-body-md text-body-md text-on-secondary-container/80">' + h.detail + '</p></div>';
      healthContainer.appendChild(div);
    });
  }

  var dosContainer = document.getElementById('commDos');
  if (dosContainer) {
    dosContainer.innerHTML = '';
    (profile.communicationDos || defaultProfile.communicationDos).forEach(function(d) {
      var item = document.createElement('div');
      item.className = 'flex items-start gap-2.5';
      item.innerHTML = '<span class="material-symbols-outlined text-success text-[18px] flex-shrink-0 mt-0.5">check_circle</span>' +
        '<p class="font-body-md text-body-md text-on-surface-variant">' + d + '</p>';
      dosContainer.appendChild(item);
    });
    (profile.communicationDonts || defaultProfile.communicationDonts).forEach(function(d) {
      var item = document.createElement('div');
      item.className = 'flex items-start gap-2.5';
      item.innerHTML = '<span class="material-symbols-outlined text-danger text-[18px] flex-shrink-0 mt-0.5">cancel</span>' +
        '<p class="font-body-md text-body-md text-on-surface-variant">' + d + '</p>';
      dosContainer.appendChild(item);
    });
  }

  var now = new Date();
  var timeEl = document.getElementById('updateTime');
  if (timeEl) {
    timeEl.textContent = 'AI 认知画像更新于 ' + (now.getMonth() + 1) + '月' + now.getDate() + '日 ' + now.getHours() + ':' + now.getMinutes().toString().padStart(2, '0');
  }

  async function loadMemoryFacts() {
    var list = document.getElementById('memoryFacts');
    if (list) list.innerHTML = '<div class="font-body-md text-body-md text-text-secondary">正在读取记忆…</div>';
    try {
      var payload = await anbanClient.listMemoryFacts({ deviceId: anbanConfig.deviceId, limit: 20 });
      renderMemoryFacts(Array.isArray(payload.facts) ? payload.facts : []);
    } catch (error) {
      if (list) list.innerHTML = '<div class="font-body-md text-body-md text-text-secondary">记忆读取失败</div>';
    }
  }

  function renderMemoryFacts(facts) {
    var list = document.getElementById('memoryFacts');
    if (!list) return;
    list.innerHTML = '';
    if (!facts.length) {
      var empty = document.createElement('div');
      empty.className = 'font-body-md text-body-md text-text-secondary';
      empty.textContent = '暂无专属记忆';
      list.appendChild(empty);
      return;
    }
    facts.forEach(function(fact) {
      var item = document.createElement('div');
      item.className = 'bg-background-cream rounded-xl px-4 py-3 flex items-start gap-2.5';
      var text = document.createElement('p');
      text.className = 'font-body-md text-body-md text-on-surface-variant leading-relaxed flex-1';
      text.textContent = fact.text || '';
      item.appendChild(text);
      if (isAdmin) {
        var edit = document.createElement('button');
        edit.className = 'memory-admin-control w-8 h-8 flex items-center justify-center rounded-full active:scale-90 transition-transform flex-shrink-0';
        edit.type = 'button';
        edit.title = '编辑记忆';
        edit.innerHTML = '<span class="material-symbols-outlined text-primary text-[18px]">edit</span>';
        edit.addEventListener('click', async function() {
          var next = window.prompt('编辑这条记忆', fact.text || '');
          if (next === null) return;
          next = next.trim();
          if (!next) {
            showToast('记忆内容不能为空');
            return;
          }
          try {
            await anbanClient.updateMemoryFact(fact.factId, { deviceId: anbanConfig.deviceId, text: next });
            showToast('记忆已更新');
            loadMemoryFacts();
          } catch (error) {
            showToast(error.message || '记忆更新失败');
          }
        });
        item.appendChild(edit);

        var remove = document.createElement('button');
        remove.className = 'memory-admin-control w-8 h-8 flex items-center justify-center rounded-full active:scale-90 transition-transform flex-shrink-0';
        remove.type = 'button';
        remove.title = '删除记忆';
        remove.innerHTML = '<span class="material-symbols-outlined text-danger text-[18px]">delete</span>';
        remove.addEventListener('click', async function() {
          if (!window.confirm('删除这条记忆？')) return;
          try {
            await anbanClient.deleteMemoryFact(fact.factId, { deviceId: anbanConfig.deviceId });
            showToast('记忆已删除');
            loadMemoryFacts();
          } catch (error) {
            showToast(error.message || '记忆删除失败');
          }
        });
        item.appendChild(remove);
      }
      list.appendChild(item);
    });
  }

  var memoryInput = document.getElementById('memoryInput');
  var memoryAddButton = document.getElementById('memoryAddButton');
  async function addMemoryFact() {
    if (!isAdmin || !memoryInput) return;
    var text = memoryInput.value.trim();
    if (!text) return;
    try {
      await anbanClient.addMemoryFact({ deviceId: anbanConfig.deviceId, text: text });
      memoryInput.value = '';
      showToast('记忆已添加');
      loadMemoryFacts();
    } catch (error) {
      showToast(error.message || '记忆添加失败');
    }
  }
  if (memoryAddButton) memoryAddButton.onclick = addMemoryFact;
  if (memoryInput) {
    memoryInput.onkeydown = function(e) {
      if (e.key === 'Enter') addMemoryFact();
    };
  }
  loadMemoryFacts();
}

// ============================
// initMine
// ============================
function initMine() {
  var baseURLInput = document.getElementById('settingsBaseURL');
  var deviceIdInput = document.getElementById('settingsDeviceId');
  var saveConnectionBtn = document.getElementById('saveConnectionBtn');
  var greetingScheduleForm = document.getElementById('greetingScheduleForm');
  if (baseURLInput) baseURLInput.value = anbanConfig.baseURL;
  if (deviceIdInput) deviceIdInput.value = anbanConfig.deviceId;
  renderAccountSettings();

  async function loadMembers() {
    var list = document.getElementById('memberList');
    if (!list || !isAccountMode() || !anbanSession.binding || anbanSession.binding.role !== 'admin') return;
    try {
      var payload = await anbanClient.listMembers();
      list.innerHTML = '';
      if (!payload.members.length) {
        list.innerHTML = '<p class="font-body-sm text-body-sm text-text-secondary">暂无其他家庭成员</p>';
        return;
      }
      payload.members.forEach(function(member) {
        var row = document.createElement('div');
        row.className = 'flex items-center justify-between py-3 border-b border-divider-warm last:border-b-0';
        row.innerHTML = '<div><p class="font-label-md text-label-md text-on-surface"></p><p class="font-label-sm text-label-sm text-text-secondary"></p></div><button class="text-danger font-label-sm text-label-sm" type="button">移除</button>';
        row.querySelector('p').textContent = member.displayName;
        row.querySelectorAll('p')[1].textContent = [member.relationshipToElder, member.phone].filter(Boolean).join(' · ');
        row.querySelector('button').addEventListener('click', async function() {
          try {
            await anbanClient.removeMember(member.accountId);
            showToast('成员已移除');
            loadMembers();
          } catch (error) {
            showToast(error.message || '成员移除失败');
          }
        });
        list.appendChild(row);
      });
    } catch (error) {
      list.innerHTML = '<p class="font-body-sm text-body-sm text-danger">成员列表加载失败</p>';
    }
  }

  function greetingSlotElements(label) {
    return {
      time: document.getElementById(label + 'GreetingTime'),
      enabled: document.getElementById(label + 'GreetingEnabled'),
      tone: document.getElementById(label + 'GreetingTone'),
    };
  }

  function writeGreetingSchedule(schedule) {
    normalizeGreetingSlots(schedule && schedule.slots).forEach(function(slot) {
      var els = greetingSlotElements(slot.label);
      if (els.time) els.time.value = slot.time;
      if (els.enabled) els.enabled.checked = slot.enabled;
      if (els.tone) els.tone.value = slot.tonePreset;
    });
  }

  function readGreetingSchedule() {
    return normalizeGreetingSlots(['morning', 'noon', 'evening'].map(function(label) {
      var els = greetingSlotElements(label);
      return {
        label: label,
        time: els.time ? els.time.value : '',
        enabled: els.enabled ? els.enabled.checked : false,
        tonePreset: els.tone ? els.tone.value : 'warm',
      };
    }));
  }

  async function loadGreetingSchedule() {
    if (!greetingScheduleForm) return;
    if (!isDeviceBound()) return;
    try {
      var schedule = await anbanClient.getGreetingSchedule({ deviceId: anbanConfig.deviceId });
      writeGreetingSchedule(schedule);
    } catch (error) {
      writeGreetingSchedule({ slots: [] });
    }
  }

  if (greetingScheduleForm) {
    greetingScheduleForm.addEventListener('submit', async function(event) {
      event.preventDefault();
      var button = document.getElementById('saveGreetingScheduleBtn');
      var slots = readGreetingSchedule();
      if (button) {
        button.disabled = true;
        button.classList.add('opacity-70');
      }
      try {
        var schedule = await anbanClient.updateGreetingSchedule({
          deviceId: anbanConfig.deviceId,
          slots: slots,
        });
        writeGreetingSchedule(schedule);
        showToast('问候时段已保存');
      } catch (error) {
        showToast(error.message || '问候时段保存失败');
      } finally {
        if (button) {
          button.disabled = false;
          button.classList.remove('opacity-70');
        }
      }
    });
    loadGreetingSchedule();
  }
  loadMembers();

  if (saveConnectionBtn) {
    saveConnectionBtn.addEventListener('click', async function() {
      var baseURL = baseURLInput.value.trim().replace(/\/+$/, '');
      var deviceId = deviceIdInput.value.trim();
      if (!baseURL || !deviceId) {
        showToast('请填写后端地址和设备 ID');
        return;
      }
      try {
        var candidateClient = createAnbanClient({ baseURL: baseURL, accessCode: anbanConfig.accessCode });
        await candidateClient.getStatus({ deviceId: deviceId });
        updateAnbanConfig({ baseURL: baseURL, deviceId: deviceId });
        showToast('连接设置已保存');
      } catch (error) {
        showToast(formatLoginError(error));
      }
    });
  }

  document.getElementById('clearCacheBtn').addEventListener('click', function() {
    var label = document.getElementById('cacheSize');
    if (label.textContent !== '0.0 MB') {
      label.textContent = '清理中...';
      label.classList.add('text-primary');
      setTimeout(function() {
        label.textContent = '0.0 MB';
        label.classList.remove('text-primary');
        label.classList.add('text-success');
      }, 800);
    }
  });



  document.getElementById('aboutBtn').addEventListener('click', function() {
    showToast('安伴 v2.4.0 — 孝心安伴');
  });

  document.getElementById('logoutBtn').addEventListener('click', async function() {
    if (isAccountMode()) {
      try { await anbanClient.logout(); } catch (error) {}
    }
    setAccountSession(null);
    anbanSession.authMode = '';
    localStorage.removeItem('anban_auth_mode');
    localStorage.removeItem('anban_session');
    showToast('已退出登录');
    setTimeout(function() { navigateTo('login'); }, 800);
  });
}

// ============================
// initFamilyEdit
// ============================
async function initFamilyEdit() {
  if (isAccountMode() && (!anbanSession.binding || anbanSession.binding.role !== 'admin')) {
    showToast('只有家庭管理员可以编辑家人画像');
    navigateTo('family');
    return;
  }
  var habitIcons = ['wb_twilight','local_cafe','music_note','directions_walk','self_improvement','menu_book','potted_plant','tv','pets','shopping_bag','exercise','park'];

  var defaultData = {
    name: '妈妈', age: 72, livingSituation: '与爸爸同住', occupation: '退休教师',
    aiPortrait: '', aiPortraitMode: 'auto',
    hobbies: ['园艺', '广场舞', '京剧', '养花', '做手工'],
    habits: [
      { icon: 'wb_twilight', text: '晨间 6:30 起床，作息规律' },
      { icon: 'local_cafe', text: '饭后必喝一杯龙井，已养成多年习惯' },
      { icon: 'music_note', text: '下午固定收听京剧频道' },
      { icon: 'directions_walk', text: '晚餐后在小区散步 30 分钟' }
    ],
    health: [
      { name: '降压药管理', detail: '每日 08:30 服用，饭后半小时温水送服。早晚各测一次血压并记录。' },
      { name: '注意事项', detail: '避免高盐饮食，保持情绪稳定。如血压超过 150/95 需及时联系医生。' }
    ],
    communicationDos: ['多聊聊她年轻时的教学故事，她会很开心', '每周至少视频通话两次，她会很期待'],
    communicationDonts: ['尽量避免在深夜提及已故的外公，以免引起情绪低落']
  };

  async function loadData() {
    var localStored = localStorage.getItem('anban_family_profile_local');
    var localProfile = localStored ? JSON.parse(localStored) : {};
    try {
      var backendProfile = await anbanClient.getProfile({ deviceId: anbanConfig.deviceId });
      return mapFieldsToStitchProfile(backendProfile.fields || {}, localProfile);
    } catch (error) {
      var legacyStored = localStorage.getItem('anban_family_profile');
      if (legacyStored) { try { return JSON.parse(legacyStored); } catch(e) {} }
      return { ...JSON.parse(JSON.stringify(defaultData)), ...localProfile };
    }
  }

  var data = await loadData();

  function syncPortraitEditor() {
    var autoToggle = document.getElementById('editAiPortraitAuto');
    var portraitInput = document.getElementById('editAiPortrait');
    var help = document.getElementById('editAiPortraitHelp');
    var isAuto = data.aiPortraitMode !== 'manual';
    autoToggle.checked = isAuto;
    portraitInput.disabled = isAuto;
    portraitInput.style.opacity = isAuto ? '0.65' : '1';
    help.textContent = isAuto
      ? '开启后，AI 会根据资料和专属记忆持续整理画像'
      : '手动模式下，AI 不会覆盖管理员填写的画像';
  }

  function populateForm() {
    document.getElementById('editName').value = data.name || '';
    document.getElementById('editAge').value = data.age || '';
    document.getElementById('editLiving').value = data.livingSituation || '';
    document.getElementById('editOccupation').value = data.occupation || '';
    document.getElementById('editAiPortrait').value = data.aiPortrait || '';
    syncPortraitEditor();
    renderHealth();
    renderHobbies();
    renderHabits();
    renderDos();
    renderDonts();
  }

  document.getElementById('editAiPortraitAuto').addEventListener('change', function() {
    data.aiPortraitMode = this.checked ? 'auto' : 'manual';
    syncPortraitEditor();
    if (!this.checked) document.getElementById('editAiPortrait').focus();
  });

  function renderHobbies() {
    var container = document.getElementById('hobbyChips');
    container.innerHTML = '';
    (data.hobbies || []).forEach(function(h, i) {
      var chip = document.createElement('span');
      chip.className = 'hobby-chip bg-secondary-container/40 text-on-secondary-container px-3 py-1.5 rounded-full font-body-md text-body-md flex items-center gap-1.5';
      chip.innerHTML = h + '<span class="material-symbols-outlined text-[16px] cursor-pointer hover:text-error transition-colors" data-idx="' + i + '">close</span>';
      container.appendChild(chip);
    });
    container.querySelectorAll('.material-symbols-outlined').forEach(function(icon) {
      icon.addEventListener('click', function() {
        var idx = parseInt(this.getAttribute('data-idx'));
        data.hobbies.splice(idx, 1);
        renderHobbies();
      });
    });
  }

  function addHobby() {
    var input = document.getElementById('hobbyInput');
    var val = input.value.trim();
    if (!val) return;
    data.hobbies = data.hobbies || [];
    data.hobbies.push(val);
    input.value = '';
    renderHobbies();
  }

  document.getElementById('addHobbyBtn').addEventListener('click', addHobby);
  document.getElementById('hobbyInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') addHobby();
  });

  function renderHabits() {
    var container = document.getElementById('habitList');
    container.innerHTML = '';
    (data.habits || []).forEach(function(h, i) {
      var row = document.createElement('div');
      row.className = 'habit-row flex items-center gap-3 py-3 border-b border-divider-warm last:border-b-0';
      row.innerHTML = '<div class="relative">' +
        '<button class="w-10 h-10 rounded-xl bg-background-cream border border-divider-warm flex items-center justify-center active:scale-95 transition-all habit-icon-btn" data-idx="' + i + '" type="button">' +
          '<span class="material-symbols-outlined text-on-surface-variant text-[20px]">' + h.icon + '</span>' +
        '</button>' +
        '<div class="icon-picker hidden absolute top-full left-0 mt-1 bg-surface-white rounded-xl shadow-lg border border-divider-warm p-2 z-50 grid grid-cols-4 gap-1" style="min-width:180px">' +
          habitIcons.map(function(icon) { return '<button class="w-9 h-9 rounded-lg flex items-center justify-center hover:bg-background-cream active:scale-90 transition-all icon-option" data-icon="' + icon + '" data-idx="' + i + '" type="button"><span class="material-symbols-outlined text-on-surface-variant text-[18px]">' + icon + '</span></button>'; }).join('') +
        '</div>' +
      '</div>' +
        '<input class="flex-1 bg-transparent font-body-md text-on-surface-variant focus:outline-none border-b border-transparent focus:border-primary/30 transition-all py-1" data-idx="' + i + '" data-field="text" value="' + h.text.replace(/"/g, '&quot;') + '" type="text"/>' +
        '<button class="w-8 h-8 flex items-center justify-center rounded-full active:scale-90 transition-transform flex-shrink-0" data-idx="' + i + '" data-action="remove"><span class="material-symbols-outlined text-outline text-[18px]">close</span></button>';
      container.appendChild(row);
    });

    var addRow = document.createElement('div');
    addRow.className = 'flex items-center justify-center py-3';
    addRow.innerHTML = '<button class="flex items-center gap-2 text-primary font-label-md text-label-md active:scale-95 transition-all px-4 py-2 rounded-xl border border-dashed border-primary/30" id="addHabitBtn"><span class="material-symbols-outlined text-[18px]">add</span>添加习惯</button>';
    container.appendChild(addRow);

    container.querySelectorAll('.habit-icon-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var picker = this.parentElement.querySelector('.icon-picker');
        document.querySelectorAll('.icon-picker').forEach(function(p) {
          if (p !== picker) p.classList.add('hidden');
        });
        picker.classList.toggle('hidden');
      });
    });

    container.querySelectorAll('.icon-option').forEach(function(opt) {
      opt.addEventListener('click', function(e) {
        e.stopPropagation();
        var idx = parseInt(this.getAttribute('data-idx'));
        var icon = this.getAttribute('data-icon');
        data.habits[idx].icon = icon;
        var row = this.closest('.habit-row');
        var iconSpan = row.querySelector('.habit-icon-btn .material-symbols-outlined');
        iconSpan.textContent = icon;
        this.closest('.icon-picker').classList.add('hidden');
      });
    });

    container.querySelectorAll('input[data-field="text"]').forEach(function(inp) {
      inp.addEventListener('input', function() {
        var idx = parseInt(this.getAttribute('data-idx'));
        data.habits[idx].text = this.value;
      });
    });
    container.querySelectorAll('button[data-action="remove"]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var idx = parseInt(this.getAttribute('data-idx'));
        data.habits.splice(idx, 1);
        renderHabits();
      });
    });

    document.getElementById('addHabitBtn').addEventListener('click', function() {
      data.habits = data.habits || [];
      data.habits.push({ icon: 'wb_twilight', text: '' });
      renderHabits();
    });
  }

  function renderHealth() {
    var container = document.getElementById('healthList');
    container.innerHTML = '';
    (data.health || []).forEach(function(h, i) {
      var item = document.createElement('div');
      item.className = 'bg-secondary-container/40 rounded-xl p-4';
      item.innerHTML = '<div class="space-y-2">' +
        '<div class="flex items-center gap-2">' +
          '<input class="flex-1 bg-white/60 border border-divider-warm rounded-lg px-3 py-2 font-label-md text-on-surface font-medium focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary-container transition-all" data-idx="' + i + '" data-field="name" value="' + (h.name || '').replace(/"/g, '&quot;') + '" placeholder="项目名称" type="text"/>' +
          '<button class="w-8 h-8 flex items-center justify-center rounded-full active:scale-90 transition-transform flex-shrink-0" data-idx="' + i + '" data-action="removeHealth"><span class="material-symbols-outlined text-outline text-[18px]">close</span></button>' +
        '</div>' +
        '<textarea class="w-full bg-white/60 border border-divider-warm rounded-lg px-3 py-2 font-body-md text-on-surface focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary-container transition-all resize-none" data-idx="' + i + '" data-field="detail" placeholder="详细说明" rows="2">' + (h.detail || '') + '</textarea>' +
      '</div>';
      container.appendChild(item);
    });

    var addBtn = document.createElement('button');
    addBtn.className = 'flex items-center gap-2 text-primary font-label-md text-label-md active:scale-95 transition-all px-4 py-2 rounded-xl border border-dashed border-primary/30 w-full justify-center mt-3';
    addBtn.innerHTML = '<span class="material-symbols-outlined text-[18px]">add</span>添加健康项目';
    addBtn.id = 'addHealthBtn';
    container.appendChild(addBtn);

    container.querySelectorAll('input[data-field="name"]').forEach(function(inp) {
      inp.addEventListener('input', function() {
        data.health[parseInt(this.getAttribute('data-idx'))].name = this.value;
      });
    });
    container.querySelectorAll('textarea[data-field="detail"]').forEach(function(inp) {
      inp.addEventListener('input', function() {
        data.health[parseInt(this.getAttribute('data-idx'))].detail = this.value;
      });
    });
    container.querySelectorAll('button[data-action="removeHealth"]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        data.health.splice(parseInt(this.getAttribute('data-idx')), 1);
        renderHealth();
      });
    });

    document.getElementById('addHealthBtn').addEventListener('click', function() {
      data.health = data.health || [];
      data.health.push({ name: '', detail: '' });
      renderHealth();
    });
  }

  function renderDos() {
    var container = document.getElementById('dosList');
    container.innerHTML = '';
    (data.communicationDos || []).forEach(function(d, i) {
      var item = document.createElement('div');
      item.className = 'do-dont-item flex items-center gap-2.5 bg-success/5 rounded-xl px-4 py-3';
      item.innerHTML = '<span class="material-symbols-outlined text-success text-[18px] flex-shrink-0">check_circle</span>' +
        '<span class="flex-1 font-body-md text-body-md text-on-surface-variant">' + d + '</span>' +
        '<button class="w-7 h-7 flex items-center justify-center rounded-full active:scale-90 transition-transform flex-shrink-0" data-idx="' + i + '" data-action="removeDo"><span class="material-symbols-outlined text-outline text-[16px]">close</span></button>';
      container.appendChild(item);
    });
    container.querySelectorAll('button[data-action=removeDo]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var idx = parseInt(this.getAttribute('data-idx'));
        data.communicationDos.splice(idx, 1);
        renderDos();
      });
    });
  }

  function addDo() {
    var input = document.getElementById('doInput');
    var val = input.value.trim();
    if (!val) return;
    data.communicationDos = data.communicationDos || [];
    data.communicationDos.push(val);
    input.value = '';
    renderDos();
  }

  document.getElementById('addDoBtn').addEventListener('click', addDo);
  document.getElementById('doInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') addDo();
  });

  function renderDonts() {
    var container = document.getElementById('dontsList');
    container.innerHTML = '';
    (data.communicationDonts || []).forEach(function(d, i) {
      var item = document.createElement('div');
      item.className = 'do-dont-item flex items-center gap-2.5 bg-danger/5 rounded-xl px-4 py-3';
      item.innerHTML = '<span class="material-symbols-outlined text-danger text-[18px] flex-shrink-0">cancel</span>' +
        '<span class="flex-1 font-body-md text-body-md text-on-surface-variant">' + d + '</span>' +
        '<button class="w-7 h-7 flex items-center justify-center rounded-full active:scale-90 transition-transform flex-shrink-0" data-idx="' + i + '" data-action="removeDont"><span class="material-symbols-outlined text-outline text-[16px]">close</span></button>';
      container.appendChild(item);
    });
    container.querySelectorAll('button[data-action=removeDont]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var idx = parseInt(this.getAttribute('data-idx'));
        data.communicationDonts.splice(idx, 1);
        renderDonts();
      });
    });
  }

  function addDont() {
    var input = document.getElementById('dontInput');
    var val = input.value.trim();
    if (!val) return;
    data.communicationDonts = data.communicationDonts || [];
    data.communicationDonts.push(val);
    input.value = '';
    renderDonts();
  }

  document.getElementById('addDontBtn').addEventListener('click', addDont);
  document.getElementById('dontInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') addDont();
  });

  function collectFormData() {
    data.name = document.getElementById('editName').value.trim();
    data.age = parseInt(document.getElementById('editAge').value) || 0;
    data.livingSituation = document.getElementById('editLiving').value.trim();
    data.occupation = document.getElementById('editOccupation').value.trim();
    data.aiPortrait = document.getElementById('editAiPortrait').value.trim();
    data.aiPortraitMode = document.getElementById('editAiPortraitAuto').checked ? 'auto' : 'manual';
  }

  document.getElementById('saveBtn').addEventListener('click', async function() {
    collectFormData();
    var localProfile = {
      age: data.age,
      livingSituation: data.livingSituation,
      occupation: data.occupation,
    };
    localStorage.setItem('anban_family_profile_local', JSON.stringify(localProfile));
    var fields = mapStitchProfileToFields(data);
    try {
      var savedProfile = await anbanClient.updateProfile({
        deviceId: anbanConfig.deviceId,
        fields: fields,
      });
      data = mapFieldsToStitchProfile(savedProfile.fields || fields, localProfile);
      localStorage.setItem('anban_family_profile', JSON.stringify(data));
      SPA.initialized['family'] = false;
      showToast('已保存');
    } catch (error) {
      showToast(error.message || '画像保存失败');
    }
  });

  // Close icon pickers on outside click
  document.addEventListener('click', function(e) {
    if (!e.target.closest('.habit-icon-btn') && !e.target.closest('.icon-picker')) {
      document.querySelectorAll('.icon-picker').forEach(function(p) { p.classList.add('hidden'); });
    }
  });

  populateForm();
}
