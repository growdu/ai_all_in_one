// home.js — Chat 主页面逻辑
// 详见 docs/frontend/03-web.md §3.1

let models = [];
let currentStream = null;

const modelSel = document.getElementById('model-select');
const modeSel = document.getElementById('mode-select');
const messagesEl = document.getElementById('messages');
const userInput = document.getElementById('user-input');
const sendBtn = document.getElementById('send-btn');
const stopBtn = document.getElementById('stop-btn');

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

  userInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      send();
    }
  });
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

  const req = { model, messages };

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
    }
  );
  currentStream = stream;
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
  } catch (e) {
    addMessage('error', 'compare 失败：' + e.message);
  }
  setStreaming(false);
}

document.addEventListener('DOMContentLoaded', init);
