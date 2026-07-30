// home.js — Chat 主页面逻辑
// 详见 docs/frontend/03-web.md §3.1

let models = [];
let configuredProviders = new Set();
let currentStream = null;
let activeConvId = null;
let activeConvTitle = '';

const modelSel = document.getElementById('model-select');
const modeSel = document.getElementById('mode-select');
const modelMultiWrap = document.getElementById('model-multi-wrap');
const modelMultiList = document.getElementById('model-multi-list');
const modelMultiHint = document.getElementById('model-multi-hint');
const modelMultiAllBtn = document.getElementById('model-multi-all');
const messagesEl = document.getElementById('messages');
const userInput = document.getElementById('user-input');
const sendBtn = document.getElementById('send-btn');
const stopBtn = document.getElementById('stop-btn');
const convTitleEl = document.getElementById('conv-title');
const convIdLabelEl = document.getElementById('conv-id-label');
const convNewBtn = document.getElementById('conv-new-btn');

function getConvIdFromURL() {
  const params = new URLSearchParams(location.search);
  return params.get('conv') || '';
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[ch]));
}

async function init() {
  try {
    const resp = await App.listModels();
    models = resp.models || [];
  } catch (e) {
    addMessage('error', '加载模型失败：' + e.message);
    return;
  }

  try {
    const resp = await App.listKeys();
    configuredProviders = new Set(resp.providers || []);
  } catch (e) {
    configuredProviders = new Set();
  }

  for (const m of models) {
    const opt = document.createElement('option');
    opt.value = m.id;
    opt.textContent = `${m.display_name} (${m.provider})`;
    modelSel.appendChild(opt);
  }
  if (models.length) modelSel.value = models[0].id;

  renderModelMulti();
  await ensureActiveConv();

  modeSel.addEventListener('change', updateModeUI);
  updateModeUI();

  if (modelMultiAllBtn) {
    modelMultiAllBtn.addEventListener('click', () => {
      const boxes = modelMultiList.querySelectorAll('input[type="checkbox"]');
      boxes.forEach(cb => {
        if (cb.dataset.configured === '1') cb.checked = true;
      });
      updateMultiHint();
    });
  }

  sendBtn.addEventListener('click', send);
  stopBtn.addEventListener('click', () => {
    if (currentStream) currentStream.close();
    currentStream = null;
    setStreaming(false);
  });

  if (convNewBtn) {
    convNewBtn.addEventListener('click', () => { location.href = 'home.html'; });
  }

  userInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      send();
    }
  });
}

function updateModeUI() {
  const mode = modeSel.value;
  if (mode === 'compare') {
    modelSel.style.display = 'none';
    if (modelMultiWrap) modelMultiWrap.style.display = 'flex';
  } else {
    modelSel.style.display = '';
    if (modelMultiWrap) modelMultiWrap.style.display = 'none';
  }
}

function renderModelMulti() {
  if (!modelMultiList) return;
  modelMultiList.innerHTML = '';
  for (const m of models) {
    const isConfigured = configuredProviders.has(m.provider);
    const row = document.createElement('label');
    row.className = 'multi-row' + (isConfigured ? '' : ' multi-unconfigured');
    row.innerHTML = `
      <input type="checkbox" value="${m.id}" data-provider="${m.provider}" data-configured="${isConfigured ? 1 : 0}">
      <span class="multi-name">${escapeHtml(m.display_name)}</span>
      <span class="multi-provider">${escapeHtml(m.provider)}</span>
      <span class="multi-status">${isConfigured ? '✓' : '○'}</span>
    `;
    modelMultiList.appendChild(row);
  }
  modelMultiList.querySelectorAll('input[type="checkbox"]').forEach(cb => {
    cb.addEventListener('change', updateMultiHint);
  });
  updateMultiHint();
}

function getSelectedProviders() {
  if (!modelMultiList) return [];
  const boxes = modelMultiList.querySelectorAll('input[type="checkbox"]:checked');
  const providers = new Set();
  boxes.forEach(cb => providers.add(cb.dataset.provider));
  return Array.from(providers);
}

function updateMultiHint() {
  if (!modelMultiHint) return;
  const n = getSelectedProviders().length;
  if (n === 0) {
    modelMultiHint.textContent = '请至少选 2 个已配 key 的 provider';
    modelMultiHint.className = 'multi-hint multi-hint-warn';
  } else if (n === 1) {
    modelMultiHint.textContent = '已选 1 个，再选 1 个可对比';
    modelMultiHint.className = 'multi-hint multi-hint-warn';
  } else {
    modelMultiHint.textContent = `已选 ${n} 个 provider，对比模式`;
    modelMultiHint.className = 'multi-hint multi-hint-ok';
  }
}

async function ensureActiveConv() {
  const urlConvId = getConvIdFromURL();
  if (urlConvId) {
    try {
      const conv = await App.fetch('/api/v1/conversations/' + encodeURIComponent(urlConvId));
      activeConvId = conv.conversation.id;
      activeConvTitle = conv.conversation.title || '';
      updateConvBar();
      renderHistory(conv.messages || []);
      if (conv.conversation.model && conv.conversation.model !== 'auto') {
        const opt = Array.from(modelSel.options).find(o => o.value === conv.conversation.model);
        if (opt) modelSel.value = opt.value;
      }
      return;
    } catch (e) {
      showToast('加载会话失败：' + e.message);
    }
  }
  try {
    const defaultModel = models.length ? models[0].id : 'mock-echo';
    const c = await App.fetch('/api/v1/conversations', {
      method: 'POST',
      body: { model: defaultModel },
    });
    activeConvId = c.id;
    activeConvTitle = c.title || '新对话';
    updateConvBar();
    const u = new URL(location.href);
    u.searchParams.set('conv', activeConvId);
    history.replaceState(null, '', u.toString());
  } catch (e) {
    showToast('创建会话失败：' + e.message);
  }
}

function updateConvBar() {
  if (convTitleEl) convTitleEl.textContent = activeConvTitle || '新对话';
  if (convIdLabelEl) {
    convIdLabelEl.textContent = activeConvId ? `· ${activeConvId.substring(0, 12)}` : '';
  }
}

function renderHistory(msgs) {
  messagesEl.innerHTML = '';
  for (const m of msgs) {
    if (m.role === 'system') continue;
    const el = addMessage(m.role, m.content);
    if (m.attachments && m.attachments.length) {
      const meta = document.createElement('div');
      meta.className = 'meta';
      meta.textContent = `[附件] ${m.attachments.length} 个`;
      el.appendChild(meta);
    }
  }
}

function setStreaming(on) {
  sendBtn.style.display = on ? 'none' : 'inline-block';
  stopBtn.style.display = on ? 'inline-block' : 'none';
  userInput.disabled = on;
}

function addMessage(role, text, meta) {
  const el = document.createElement('div');
  el.className = 'message ' + role;
  el.textContent = text;
  if (meta) {
    const m = document.createElement('div');
    m.className = 'meta';
    m.textContent = meta;
    el.appendChild(m);
  }
  messagesEl.appendChild(el);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return el;
}

function addCompareResult(r) {
  const el = document.createElement('div');
  el.className = 'message compare-result' + (r.status === 'failed' ? ' error' : '');
  const head = document.createElement('div');
  head.className = 'compare-head';
  head.innerHTML = `
    <span class="compare-provider">${escapeHtml(r.provider)}</span>
    <span class="compare-latency">${r.latency_ms}ms</span>
    <span class="compare-status compare-status-${r.status}">${r.status}</span>
  `;
  el.appendChild(head);
  const body = document.createElement('div');
  body.className = 'compare-body';
  if (r.status === 'succeeded') {
    body.textContent = r.content || '(空响应)';
  } else {
    body.textContent = r.error?.message || '失败';
  }
  el.appendChild(body);
  messagesEl.appendChild(el);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return el;
}

async function send() {
  const content = userInput.value.trim();
  if (!content) return;

  const mode = modeSel.value;

  if (mode === 'compare') {
    const providers = getSelectedProviders();
    if (providers.length < 2) {
      showToast('对比模式请至少选 2 个 provider');
      return;
    }
    await sendCompare(content, providers);
    return;
  }

  const model = mode === 'auto' ? 'auto' : modelSel.value;
  const messages = [];
  const sysPrompt = App.getSystemPrompt();
  if (sysPrompt) messages.push({ role: 'system', content: sysPrompt });
  messages.push({ role: 'user', content });

  const req = { model, messages, conv_id: activeConvId || undefined };

  addMessage('user', content);
  userInput.value = '';
  setStreaming(true);
  const placeholder = addMessage('assistant', I18n.t('chat.thinking'));

  const stream = App.chatStream(req,
    (chunk) => {
      placeholder.textContent = (placeholder.textContent === I18n.t('chat.thinking') ? '' : placeholder.textContent) + chunk.delta;
      messagesEl.scrollTop = messagesEl.scrollHeight;
      if (chunk.finish_reason === 'stop' && chunk.usage) {
        const meta = document.createElement('div');
        meta.className = 'meta';
        meta.textContent = `${chunk.id} · ${chunk.usage.total_tokens} tokens`;
        placeholder.appendChild(meta);
      }
    },
    (err) => {
      placeholder.className = 'message error';
      placeholder.textContent = '错误：' + err.message;
      setStreaming(false);
    },
    () => {
      setStreaming(false);
      currentStream = null;
      maybeUpdateConvTitle();
    }
  );
  currentStream = stream;
}

async function sendCompare(content, providers) {
  addMessage('user', content);
  userInput.value = '';
  setStreaming(true);

  const messages = [];
  const sysPrompt = App.getSystemPrompt();
  if (sysPrompt) messages.push({ role: 'system', content: sysPrompt });
  messages.push({ role: 'user', content });

  const firstConfiguredModel = models
    .filter(m => providers.includes(m.provider))
    .map(m => m.id)[0] || 'mock-echo';

  const req = {
    model: firstConfiguredModel,
    messages,
    conv_id: activeConvId || undefined,
    compare: { providers, format: 'stacked' },
  };

  try {
    const resp = await App.chat(req);
    const results = resp.compare?.results || [];
    if (!results.length) {
      addMessage('error', '对比模式无返回结果');
    }
    for (const r of results) {
      addCompareResult(r);
    }
    maybeUpdateConvTitle();
  } catch (e) {
    addMessage('error', 'compare 失败：' + e.message);
  }
  setStreaming(false);
}

async function maybeUpdateConvTitle() {
  if (!activeConvId || activeConvTitle !== '新对话') return;
  const userMsgs = Array.from(messagesEl.querySelectorAll('.message.user'));
  const last = userMsgs[userMsgs.length - 1];
  if (!last) return;
  const text = (last.textContent || '').trim();
  if (!text) return;
  const newTitle = text.substring(0, 16) + (text.length > 16 ? '…' : '');
  try {
    await App.fetch('/api/v1/conversations/' + encodeURIComponent(activeConvId), {
      method: 'PATCH',
      body: { title: newTitle },
    });
    activeConvTitle = newTitle;
    updateConvBar();
  } catch (_) {}
}

document.addEventListener('DOMContentLoaded', init);