// home.js — Chat 主页面逻辑
// 详见 docs/frontend/03-web.md §3.1

let models = [];
let currentStream = null;
// Phase 1.2：当前会话上下文
let activeConvId = null;
let activeConvTitle = '';

const modelSel = document.getElementById('model-select');
const modeSel = document.getElementById('mode-select');
const messagesEl = document.getElementById('messages');
const userInput = document.getElementById('user-input');
const sendBtn = document.getElementById('send-btn');
const stopBtn = document.getElementById('stop-btn');
const convTitleEl = document.getElementById('conv-title');
const convIdLabelEl = document.getElementById('conv-id-label');
const convNewBtn = document.getElementById('conv-new-btn');

// 从 URL 拿 ?conv=<id>
function getConvIdFromURL() {
  const params = new URLSearchParams(location.search);
  return params.get('conv') || '';
}

async function init() {
  // 加载模型
  try {
    const resp = await App.listModels();
    models = resp.models || [];
  } catch (e) {
    addMessage('error', '加载模型失败：' + e.message);
    return;
  }

  // 填充 model select
  for (const m of models) {
    const opt = document.createElement('option');
    opt.value = m.id;
    opt.textContent = `${m.display_name} (${m.provider})`;
    modelSel.appendChild(opt);
  }

  // 默认选第一个
  if (models.length) modelSel.value = models[0].id;

  // Phase 1.2：加载或创建会话
  await ensureActiveConv();

  // mode 切换
  modeSel.addEventListener('change', () => {
    // single 模式 = 用具体 model；auto = model="auto"；compare = 加 compare 字段
    // 这里前端简化：只根据 mode 调不同请求体
  });

  // 按钮事件
  sendBtn.addEventListener('click', send);
  stopBtn.addEventListener('click', () => {
    if (currentStream) currentStream.close();
    currentStream = null;
    setStreaming(false);
  });

  // 新建会话按钮
  if (convNewBtn) {
    convNewBtn.addEventListener('click', () => {
      location.href = 'home.html';
    });
  }

  userInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      send();
    }
  });
}

// Phase 1.2：确保有 active conv —— URL 优先，否则新建一个
async function ensureActiveConv() {
  const urlConvId = getConvIdFromURL();
  if (urlConvId) {
    // 加载已有会话
    try {
      const conv = await App.fetch('/api/v1/conversations/' + encodeURIComponent(urlConvId));
      activeConvId = conv.conversation.id;
      activeConvTitle = conv.conversation.title || '';
      updateConvBar();
      // 渲染历史消息
      renderHistory(conv.messages || []);
      // 恢复 mode/model 选择（1.0 简化：只恢复 model）
      if (conv.conversation.model && conv.conversation.model !== 'auto') {
        const opt = Array.from(modelSel.options).find(o => o.value === conv.conversation.model);
        if (opt) modelSel.value = opt.value;
      }
      return;
    } catch (e) {
      showToast('加载会话失败：' + e.message);
      // fallback: 新建
    }
  }
  // 新建会话（不立刻落库，等首次 send 时让后端落）
  // 简化：前端立刻建，标题默认
  try {
    const defaultModel = models.length ? models[0].id : 'mock-echo';
    const c = await App.fetch('/api/v1/conversations', {
      method: 'POST',
      body: { model: defaultModel },
    });
    activeConvId = c.id;
    activeConvTitle = c.title || '新对话';
    updateConvBar();
    // 更新 URL（不刷新页面）
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

// Phase 1.2：渲染历史消息
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

async function send() {
  const content = userInput.value.trim();
  if (!content) return;

  const mode = modeSel.value;
  const model = mode === 'auto' ? 'auto' : modelSel.value;

  // 构造请求
  const messages = [];
  const sysPrompt = App.getSystemPrompt();
  if (sysPrompt) {
    messages.push({ role: 'system', content: sysPrompt });
  }
  messages.push({ role: 'user', content });

  // Phase 1.2：附带 conv_id 让后端自动落消息
  const req = { model, messages, conv_id: activeConvId || undefined };

  if (mode === 'compare') {
    // compare 模式：1.0 不支持 stream，并行发 N 个
    await sendCompare(req);
    return;
  }

  addMessage('user', content);
  userInput.value = '';

  // single / auto 模式
  if (mode === 'auto') {
    // auto: model="auto" 后端选
  }

  setStreaming(true);
  const placeholder = addMessage('assistant', I18n.t('chat.thinking'));

  const stream = App.chatStream(req,
    (chunk) => {
      placeholder.textContent = (placeholder.textContent === I18n.t('chat.thinking') ? '' : placeholder.textContent) + chunk.delta;
      messagesEl.scrollTop = messagesEl.scrollHeight;
      if (chunk.finish_reason === 'stop') {
        if (chunk.usage) {
          const meta = document.createElement('div');
          meta.className = 'meta';
          meta.textContent = `${chunk.id} · ${chunk.usage.total_tokens} tokens`;
          placeholder.appendChild(meta);
        }
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
      // Phase 1.2：第一条 user 后，自动用前 16 字生成 title
      maybeUpdateConvTitle();
    }
  );
  currentStream = stream;
}

// Phase 1.2：第一次 send 后，把标题更新为 user 内容的前 16 字（一次性）
async function maybeUpdateConvTitle() {
  if (!activeConvId || activeConvTitle !== '新对话') return;
  // 找最后一条 user 消息
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

async function sendCompare(req) {
  addMessage('user', req.messages[req.messages.length - 1].content);
  userInput.value = '';
  setStreaming(true);

  // 1.0 简化：只列已配 key 的 provider
  let configured = [];
  try {
    const keys = await App.listKeys();
    configured = keys.providers || [];
  } catch (e) {}

  req.model = req.model === 'auto' ? 'mock-echo' : req.model; // dummy
  req.compare = { providers: configured.length ? configured : ['mock', 'slow'], format: 'stacked' };

  try {
    const resp = await App.chat(req);
    const results = resp.compare?.results || [];
    for (const r of results) {
      const el = addMessage('assistant',
        r.status === 'succeeded' ? r.content : `[${r.provider}] ${r.error?.message || '失败'}`,
        `${r.provider} · ${r.latency_ms}ms · ${r.status}`);
      if (r.status === 'succeeded') el.className = 'message assistant';
      else el.className = 'message error';
    }
    maybeUpdateConvTitle();
  } catch (e) {
    addMessage('error', 'compare 失败：' + e.message);
  }
  setStreaming(false);
}

document.addEventListener('DOMContentLoaded', init);