import { ApiError, createAnbanClient } from './api/client.js';
import { loadConfig, saveConfig } from './config.js';
import { formatLoginError } from './integration-core.js';
import { notImplemented as notifyNotImplemented } from './not-implemented.js';

var anbanConfig = loadConfig();
var anbanClient = createAnbanClient(anbanConfig);

window.anbanRuntime = {
  ApiError,
  config: anbanConfig,
  client: anbanClient,
};

function updateAnbanConfig(patch) {
  anbanConfig = saveConfig({ ...anbanConfig, ...patch });
  anbanClient = createAnbanClient(anbanConfig);
  window.anbanRuntime.config = anbanConfig;
  window.anbanRuntime.client = anbanClient;
  return anbanConfig;
}

function notImplemented(featureName) {
  return notifyNotImplemented(featureName, showToast);
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
  var input = document.getElementById('accessCode');

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

  // Ensure history page always refreshes
  if (name === 'history' && typeof window.initHistory === 'function') {
    window.initHistory();
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

  // Refresh home title when navigating back
  if (name === 'home') {
    var stored = localStorage.getItem('anban_family_profile');
    var profile = stored ? JSON.parse(stored) : null;
    var name2 = (profile && profile.name) || '家人';
    var hour = new Date().getHours();
    var greeting = hour < 10 ? '今天状态不错' : hour < 14 ? '今天状态不错' : hour < 18 ? '午后状态很好' : '晚上一切安好';
    var titleEl = document.getElementById('statusTitle');
    if (titleEl) titleEl.textContent = name2 + '今天状态很好';
    var descEl = document.getElementById('statusDesc');
    var timeWord = hour < 10 ? '早上' : hour < 14 ? '上午' : hour < 18 ? '下午' : '晚上';
    if (descEl) descEl.textContent = timeWord + '好，' + greeting + '。';
  }

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
var initialRoute = getRouteFromHash();
if (initialRoute === 'login') {
  // If already logged in, skip to home
  if (localStorage.getItem('anban_session')) {
    initialRoute = 'home';
    location.replace('#home');
  }
}
showSection(initialRoute);

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
  async function handleLogin() {
    var input = document.getElementById('accessCode');
    var accessCode = input.value.trim();
    if (!accessCode) {
      input.classList.add('border-danger');
      input.style.animation = 'none';
      input.offsetHeight;
      input.style.animation = 'shake 0.3s ease';
      setTimeout(function() { input.classList.remove('border-danger'); }, 500);
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
      var candidateClient = createAnbanClient({
        baseURL: anbanConfig.baseURL,
        accessCode: accessCode,
      });
      await candidateClient.getStatus({ deviceId: anbanConfig.deviceId });
      updateAnbanConfig({ accessCode });
      localStorage.setItem('anban_session', '1');
      navigateTo('home');
    } catch (error) {
      input.classList.add('border-danger');
      showToast(formatLoginError(error));
      setTimeout(function() { input.classList.remove('border-danger'); }, 1200);
    } finally {
      btnText.textContent = '登录';
      btnIcon.textContent = 'arrow_forward';
      btnIcon.style.animation = '';
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
    }
  }

  document.getElementById('loginBtn').addEventListener('click', handleLogin);
  document.getElementById('accessCode').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') handleLogin();
  });
  document.getElementById('accessCode').addEventListener('focus', function() {
    this.classList.remove('border-danger');
  });
}

// ============================
// initHome
// ============================
function initHome() {
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
  window.sendQuickMsg = function() {
    var input = document.getElementById('quickMsgInput');
    var text = input.value.trim();
    if (!text) return;
    var now = new Date();
    var time = now.getHours().toString().padStart(2,'0') + ':' + now.getMinutes().toString().padStart(2,'0');
    var stored = localStorage.getItem('anban_messages');
    var messages = stored ? JSON.parse(stored) : [];
    messages.unshift({ text: text, time: time });
    localStorage.setItem('anban_messages', JSON.stringify(messages));
    updateRecentMessages(text, time);
    input.value = '';
    closeQuickMsg();
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
  window.sendFromSheet = function() {
    var input = document.getElementById('sheetInput');
    var text = input.value.trim();
    if (!text) return;
    var stored = localStorage.getItem('anban_messages');
    var messages = stored ? JSON.parse(stored) : [];
    var now = new Date();
    var time = now.getHours().toString().padStart(2,'0') + ':' + now.getMinutes().toString().padStart(2,'0');
    messages.push({ text: text, time: time });
    localStorage.setItem('anban_messages', JSON.stringify(messages));
    input.value = '';
    closeSheet();
    updateRecentMessages(text, time);
  };
  window.updateRecentMessages = function(text, time) {
    var msgItems = document.querySelectorAll('#recentMsgList > div');
    var target = msgItems[1];
    if (target) {
      var titleEl = target.querySelector('.font-label-md');
      var timeEl = target.querySelector('.font-body-md');
      if (titleEl) titleEl.textContent = text;
      if (timeEl) timeEl.textContent = '刚刚 · ' + (text === '❤️' ? '❤️报平安' : '文本转语音');
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
  window.saveQuickReminder = function() {
    var input = document.getElementById('reminderSheetInput');
    var text = input.value.trim();
    if (!text) return;
    var time = document.getElementById('reminderTimeDisplay').textContent;
    var freq = document.getElementById('sheetFreqDisplay').textContent;
    var stored = localStorage.getItem('anban_reminders');
    var reminders = stored ? JSON.parse(stored) : [];
    reminders.push({ text: text, time: time, freq: freq });
    localStorage.setItem('anban_reminders', JSON.stringify(reminders));
    input.value = '';
    closeSheet();
    showToast('提醒已保存');
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

  // Weather fetch
  (function fetchWeather() {
    function updateStatusCard(temp) {
      var stored = localStorage.getItem('anban_family_profile');
      var profile = stored ? JSON.parse(stored) : null;
      var name = (profile && profile.name) || '家人';
      var hour = new Date().getHours();
      var timeWord = hour < 10 ? '早上' : hour < 14 ? '上午' : hour < 18 ? '下午' : '晚上';
      var greeting = hour < 10 ? '新的一天开始了' : hour < 14 ? '今天状态不错' : hour < 18 ? '午后状态很好' : '晚上一切安好';
      var tempDesc = temp ? (temp > 30 ? '天气有点热，记得开空调' : temp < 10 ? '天气较冷，记得添衣服' : '温度舒适') : '';
      var titleEl = document.getElementById('statusTitle');
      var descEl = document.getElementById('statusDesc');
      var timeEl = document.getElementById('statusTime');
      if (titleEl) titleEl.textContent = name + '今天状态很好';
      if (descEl) descEl.textContent = timeWord + '好，' + greeting + '。' + (tempDesc ? '室外' + temp + '°C，' + tempDesc + '。' : '');
      if (timeEl) timeEl.textContent = hour + ':' + new Date().getMinutes().toString().padStart(2,'0') + ' 更新';
    }
    updateStatusCard(null);
    if (!navigator.geolocation) return;
    navigator.geolocation.getCurrentPosition(
      function(pos) {
        var url = 'https://api.open-meteo.com/v1/forecast?latitude=' + pos.coords.latitude + '&longitude=' + pos.coords.longitude + '&current=temperature_2m,relative_humidity_2m';
        fetch(url)
          .then(function(r) { return r.json(); })
          .then(function(d) {
            if (d && d.current) {
              var realTemp = Math.round(d.current.temperature_2m);
              document.getElementById('envTemp').textContent = realTemp + '°C';
              document.getElementById('envHumidity').textContent = Math.round(d.current.relative_humidity_2m) + '%';
              updateStatusCard(realTemp);
            }
          })
          .catch(function() { updateStatusCard(null); });
      },
      function() { updateStatusCard(null); }
    );
  })();
}

window.initHistory = function() {
    var list = document.getElementById('historyList');
    var empty = document.getElementById('historyEmpty');
    if (!list || !empty) return;
    list.innerHTML = '';

    localStorage.removeItem('anban_reminder_history');

    // Demo data - important reminders (e.g. medication) NEVER have "忙碌未提醒" status
    // because important reminders bypass busy check and always get announced
    var records = [
        { text: '吃降压药', time: '08:30', freq: '每天', date: '6月15日', status: '已提醒', important: true },
        { text: '喝水', time: '10:00', freq: '每天', date: '6月15日', status: '已提醒' },
        { text: '吃午饭', time: '12:00', freq: '每天', date: '6月15日', status: '已提醒' },
        { text: '吃降压药', time: '08:30', freq: '每天', date: '6月14日', status: '已提醒', important: true },
        { text: '喝水', time: '10:00', freq: '每天', date: '6月14日', status: '已提醒' },
        { text: '吃午饭', time: '12:00', freq: '每天', date: '6月14日', status: '忙碌未提醒' },
        { text: '睡觉', time: '21:30', freq: '每天', date: '6月14日', status: '已提醒' },
        { text: '吃降压药', time: '08:30', freq: '每天', date: '6月13日', status: '已提醒', important: true },
        { text: '喝水', time: '10:00', freq: '每天', date: '6月13日', status: '忙碌未提醒' },
        { text: '吃午饭', time: '12:00', freq: '每天', date: '6月13日', status: '已提醒' },
        { text: '去社区医院体检', time: '09:00', freq: '6月20日', date: '6月13日', status: '已提醒' },
        { text: '吃降压药', time: '08:30', freq: '每天', date: '6月12日', status: '已提醒', important: true },
        { text: '喝水', time: '10:00', freq: '每天', date: '6月12日', status: '已提醒' },
        { text: '吃午饭', time: '12:00', freq: '每天', date: '6月12日', status: '忙碌未提醒' },
        { text: '睡觉', time: '21:30', freq: '每天', date: '6月12日', status: '已提醒' },
        { text: '吃降压药', time: '08:30', freq: '每天', date: '6月11日', status: '已提醒', important: true },
        { text: '喝水', time: '10:00', freq: '每天', date: '6月11日', status: '忙碌未提醒' },
        { text: '吃午饭', time: '12:00', freq: '每天', date: '6月11日', status: '已提醒' },
        { text: '睡觉', time: '21:30', freq: '每天', date: '6月11日', status: '已提醒' },
        { text: '吃降压药', time: '08:30', freq: '每天', date: '6月10日', status: '已提醒', important: true },
        { text: '喝水', time: '10:00', freq: '每天', date: '6月10日', status: '忙碌未提醒' },
        { text: '吃午饭', time: '12:00', freq: '每天', date: '6月10日', status: '已提醒' },
        { text: '去社区医院体检', time: '09:00', freq: '6月10日', date: '6月10日', status: '已提醒' },
        { text: '睡觉', time: '21:30', freq: '每天', date: '6月10日', status: '已提醒' }
    ];

    if (records.length === 0) { empty.style.display = ''; return; }
    empty.style.display = 'none';

    // Group by date
    var groups = {};
    for (var i = 0; i < records.length; i++) {
      var r = records[i];
      var dateKey = r.date || '未知日期';
      if (!groups[dateKey]) groups[dateKey] = [];
      groups[dateKey].push(r);
    }

    var dateKeys = Object.keys(groups);
    for (var d = 0; d < dateKeys.length; d++) {
      var dateLabel = dateKeys[d];
      var items = groups[dateLabel];

      var header = document.createElement('div');
      header.className = 'font-label-md text-label-md text-text-secondary mb-2 mt-6';
      if (d === 0) header.classList.remove('mt-6');
      header.textContent = dateLabel;
      list.appendChild(header);

      for (var j = 0; j < items.length; j++) {
        var r = items[j];
        var isBusy = r.status === '忙碌未提醒';
        var isImportant = r.important === true;
        var div = document.createElement('div');
        div.className = 'flex items-center gap-4 py-3 ' + (j < items.length - 1 ? 'border-b border-divider-warm/30' : '');
        var iconColor = isBusy ? 'text-warning' : 'text-success';
        var icon = isBusy ? 'event_busy' : 'check_circle';
        var statusColor = isBusy ? 'text-warning' : 'text-success';
        var statusText = isBusy ? '忙碌未提醒' : '已提醒';
        var importantTag = isImportant ? ' <span class="bg-warning/20 text-warning text-[9px] px-1.5 py-0.5 rounded-full font-bold">重要</span>' : '';
        var nameHtml = '<span class="font-body-md text-body-md text-on-surface">' + r.text + '</span>' + importantTag;
        div.innerHTML = '<span class="material-symbols-outlined ' + iconColor + ' text-[20px]" style="font-variation-settings:\'FILL\' 1">' + icon + '</span><div class="flex-1 flex items-center gap-2">' + nameHtml + '</div><div class="flex items-center gap-3"><span class="font-label-sm text-label-sm text-on-surface-variant">' + r.time + '</span><span class="font-label-sm text-label-sm ' + statusColor + '">' + statusText + '</span></div>';
        list.appendChild(div);
      }
    }
  };;

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

window.doDeleteReminder = function() {
  var card = _detailCard;
  if (card) {
    card.style.transition = 'all 0.3s ease';
    card.style.opacity = '0';
    card.style.transform = 'translateX(60px)';
    setTimeout(function() { card.remove(); }, 300);
  }
  _detailCard = null;
  closeDeleteConfirm();
  navigateTo('warn');
};


// ============================
// initMessage
// ============================
function initMessage() {
  function loadMessages() {
    var stored = localStorage.getItem('anban_messages');
    if (stored) {
      var messages = JSON.parse(stored);
      var chatArea = document.getElementById('chatArea');
      messages.forEach(function(msg) {
        var div = document.createElement('div');
        div.className = 'flex flex-col items-end max-w-[85%] self-end';
        var now = new Date();
        var time = now.getHours().toString().padStart(2,'0') + ':' + now.getMinutes().toString().padStart(2,'0');
        div.innerHTML = '<div class="bg-secondary-container text-on-secondary-container bubble-right p-4 rounded-2xl"><p class="font-body-md text-body-md">' + msg.text + '</p></div><div class="flex items-center gap-2 mt-1.5 px-1"><span class="text-[11px] text-on-surface-variant/50">' + msg.time + '</span><span class="text-[11px] font-medium text-warning/80 flex items-center gap-0.5"><span class="material-symbols-outlined text-[12px]" style="font-variation-settings: \'wght\' 500;">schedule</span>待播报</span></div>';
        chatArea.appendChild(div);
      });
      var container = document.getElementById('messagesContainer');
      requestAnimationFrame(function() {
        requestAnimationFrame(function() {
          if (container) container.scrollTop = container.scrollHeight;
        });
      });
    }
  }

  window.handleSend = function() {
    var input = document.getElementById('messageInput');
    var text = input.value.trim();
    if (!text) return;
    var chatArea = document.getElementById('chatArea');
    var div = document.createElement('div');
    div.className = 'flex flex-col items-end max-w-[85%] self-end';
    var now = new Date();
    var time = now.getHours().toString().padStart(2,'0') + ':' + now.getMinutes().toString().padStart(2,'0');
    div.innerHTML = '<div class="bg-secondary-container text-on-secondary-container bubble-right p-4 rounded-2xl"><p class="font-body-md text-body-md">' + text + '</p></div><div class="flex items-center gap-2 mt-1.5 px-1"><span class="text-[11px] text-on-surface-variant/50">刚刚</span><span class="text-[11px] font-medium text-warning/80 flex items-center gap-0.5"><span class="material-symbols-outlined text-[12px]" style="font-variation-settings: \'wght\' 500;">schedule</span>待播报</span></div>';
    chatArea.appendChild(div);
    var stored = localStorage.getItem('anban_messages');
    var messages = stored ? JSON.parse(stored) : [];
    messages.push({ text: text, time: time });
    localStorage.setItem('anban_messages', JSON.stringify(messages));
    input.value = '';
    input.style.height = '48px';
    var container = document.getElementById('messagesContainer');
    if (container) container.scrollTop = container.scrollHeight;
  };

  var tx = document.getElementById('messageInput');
  if (tx) {
    tx.addEventListener('input', function() {
      this.style.height = 'auto';
      this.style.height = (this.scrollHeight) + 'px';
    });
  }

  loadMessages();
  setTimeout(function() {
    var container = document.getElementById('messagesContainer');
    if (container) {
      requestAnimationFrame(function() {
        requestAnimationFrame(function() {
          container.scrollTop = container.scrollHeight;
        });
      });
    }
  }, 300);
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
  var selectedFreq = '每天';
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

  window.moveToHistory = function(el) {
    var card = el.closest('.bg-surface-white');
    if (!card) return;
    var title = card.querySelector('h4');
    var timeEl = card.querySelector('p.font-bold.text-title-lg');
    var freq = card.querySelector('span.bg-primary-container');
    var histItem = {
      text: title ? title.textContent : '',
      time: timeEl ? timeEl.textContent : '',
      freq: freq ? freq.textContent : '',
      pausedAt: new Date().toLocaleDateString('zh-CN')
    };
    var stored = localStorage.getItem('anban_reminder_history');
    var history = stored ? JSON.parse(stored) : [];
    history.unshift(histItem);
    localStorage.setItem('anban_reminder_history', JSON.stringify(history));
    card.style.transition = 'all 0.3s ease';
    card.style.opacity = '0';
    card.style.transform = 'translateX(60px)';
    setTimeout(function() { card.remove(); }, 300);
  };

  window.handleToggle = function(el) {
    var card = el.closest('.bg-surface-white');
    if (el.classList.contains('on')) {
      el.classList.remove('on');
      el.classList.add('off');
      if (card) card.classList.add('reminder-paused');
    } else {
      el.classList.remove('off');
      el.classList.add('on');
      if (card) card.classList.remove('reminder-paused');
    }
  };




  window.saveReminder = function() {
    var input = document.getElementById('reminderContent');
    var text = input.value.trim();
    if (!text) {
      input.style.borderColor = '#F78C6B';
      input.style.animation = 'shake 0.4s ease';
      input.setAttribute('placeholder', '请输入提醒内容');
      setTimeout(function() { input.style.borderColor = ''; input.style.animation = ''; }, 800);
      return;
    }
    var time = document.getElementById('timeDisplay').textContent;
    var freq = document.getElementById('freqDisplay').textContent;
    var importantToggle = document.getElementById('importantToggle');
    var isImportant = importantToggle && importantToggle.classList.contains('on');
    var list = document.getElementById('reminderList');
    var div = document.createElement('div');
    div.className = 'bg-surface-white rounded-2xl p-5 soft-shadow flex items-center justify-between';
    div.setAttribute('data-important', isImportant ? '1' : '0');
    var importantTag = isImportant ? '<span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-0.5"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span>' : '';
    var iconClass = isImportant ? 'text-warning' : 'text-primary';
    div.innerHTML = '<div class="flex gap-4 cursor-pointer active:opacity-80 transition-opacity" onclick="openDetail(this.parentElement)"><div class="w-12 h-12 rounded-full bg-primary-container/10 flex items-center justify-center flex-shrink-0"><span class="material-symbols-outlined ' + iconClass + '">notifications</span></div><div><div class="flex items-center gap-2 mb-1"><h4 class="font-bold text-body-lg text-body-lg text-on-surface">' + text + '</h4><span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold">' + freq + '</span>' + importantTag + '</div><p class="font-bold text-title-lg text-title-lg text-on-surface mb-1">' + time + '</p><p class="font-label-sm text-label-sm text-text-secondary">备注：自定义提醒</p></div></div><div class="toggle-track on flex-shrink-0" onclick="event.stopPropagation();handleToggle(this)"><div class="toggle-thumb"></div></div>';
    list.insertBefore(div, list.firstChild);
    input.value = '';
    input.setAttribute('placeholder', '例如：吃早饭、吃降压药、喝水等');
    if (importantToggle) { importantToggle.classList.remove('on'); importantToggle.classList.add('off'); }
    if (counter) counter.textContent = '0/20';
    window.updateCount();
  };

  function loadSavedReminders() {
    var stored = localStorage.getItem('anban_reminders');
    if (!stored) return;
    var reminders = JSON.parse(stored);
    var list = document.getElementById('reminderList');
    for (var i = 0; i < reminders.length; i++) {
      var r = reminders[i];
      var div = document.createElement('div');
      div.className = 'bg-surface-white rounded-2xl p-5 soft-shadow flex items-center justify-between';
      div.setAttribute('data-important', r.important ? '1' : '0');
      div.setAttribute('data-name', r.text);
      div.setAttribute('data-time', r.time);
      div.setAttribute('data-freq', r.freq || '每天');
      div.setAttribute('data-note', r.note || '快速添加');
      div.setAttribute('data-icon', 'notifications');
      div.setAttribute('data-iconcolor', r.important ? 'warning' : 'primary');
      div.innerHTML = '<div class="flex gap-4 cursor-pointer active:opacity-80 transition-opacity" onclick="openDetail(this.parentElement)"><div class="w-12 h-12 rounded-full bg-primary-container/10 flex items-center justify-center flex-shrink-0"><span class="material-symbols-outlined ' + (r.important ? 'text-warning' : 'text-primary') + '">notifications</span></div><div><div class="flex items-center gap-2 mb-1"><h4 class="font-bold text-body-lg text-body-lg text-on-surface">' + r.text + '</h4><span class="bg-primary-container/20 text-on-primary-container text-[10px] px-2 py-0.5 rounded-full font-bold">' + (r.freq || '每天') + '</span>' + (r.important ? '<span class="bg-warning/20 text-warning text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-0.5"><span class="material-symbols-outlined" style="font-size:12px;font-variation-settings:\'FILL\' 1">priority_high</span>重要</span>' : '') + '</div><p class="font-bold text-title-lg text-title-lg text-on-surface mb-1">' + r.time + '</p><p class="font-label-sm text-label-sm text-text-secondary">备注：' + (r.note || '快速添加') + '</p></div></div><div class="toggle-track on flex-shrink-0" onclick="event.stopPropagation();handleToggle(this)"><div class="toggle-thumb"></div></div>';
      list.insertBefore(div, list.firstChild);
    }
    window.updateCount();
  }

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
  loadSavedReminders();
}

// ============================
// initFamily
// ============================
function initFamily() {
  var defaultProfile = {
    name: '妈妈', age: 72, livingSituation: '与爸爸同住', occupation: '退休教师',
    aiPortrait: '性格温和，心思细腻，是全家人的定心丸。她热爱生活，即便年岁已高也保持着一颗对新鲜事物好奇的心。退休前曾是一名优秀教师，对待晚辈总是耐心开导，是家族里最受尊敬的长辈。',
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

  var stored = localStorage.getItem('anban_family_profile');
  var profile = stored ? JSON.parse(stored) : defaultProfile;

  document.getElementById('profileName').textContent = profile.name || '妈妈';
  document.getElementById('profileSubtitle').innerHTML = (profile.age || 72) + '岁 · ' + (profile.livingSituation || '与爸爸同住') + ' · ' + (profile.occupation || '退休教师');
  document.getElementById('aiPortraitText').textContent = profile.aiPortrait || defaultProfile.aiPortrait;

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
}

// ============================
// initMine
// ============================
function initMine() {
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

  document.getElementById('logoutBtn').addEventListener('click', function() {
    localStorage.removeItem('anban_session');
    showToast('已退出登录');
    setTimeout(function() { navigateTo('login'); }, 800);
  });
}

// ============================
// initFamilyEdit
// ============================
function initFamilyEdit() {
  var habitIcons = ['wb_twilight','local_cafe','music_note','directions_walk','self_improvement','menu_book','potted_plant','tv','pets','shopping_bag','exercise','park'];

  var defaultData = {
    name: '妈妈', age: 72, livingSituation: '与爸爸同住', occupation: '退休教师',
    aiPortrait: '性格温和，心思细腻，是全家人的定心丸。她热爱生活，即便年岁已高也保持着一颗对新鲜事物好奇的心。退休前曾是一名优秀教师，对待晚辈总是耐心开导，是家族里最受尊敬的长辈。',
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

  function loadData() {
    var stored = localStorage.getItem('anban_family_profile');
    if (stored) { try { return JSON.parse(stored); } catch(e) {} }
    return JSON.parse(JSON.stringify(defaultData));
  }

  var data = loadData();

  function populateForm() {
    document.getElementById('editName').value = data.name || '';
    document.getElementById('editAge').value = data.age || '';
    document.getElementById('editLiving').value = data.livingSituation || '';
    document.getElementById('editOccupation').value = data.occupation || '';
    document.getElementById('editAiPortrait').value = data.aiPortrait || '';
    renderHealth();
    renderHobbies();
    renderHabits();
    renderDos();
    renderDonts();
  }

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
  }

  document.getElementById('saveBtn').addEventListener('click', function() {
    collectFormData();
    localStorage.setItem('anban_family_profile', JSON.stringify(data));
    // Reset family init so it re-renders on next visit
    SPA.initialized['family'] = false;
    showToast('已保存');
  });

  // Close icon pickers on outside click
  document.addEventListener('click', function(e) {
    if (!e.target.closest('.habit-icon-btn') && !e.target.closest('.icon-picker')) {
      document.querySelectorAll('.icon-picker').forEach(function(p) { p.classList.add('hidden'); });
    }
  });

  populateForm();
}
