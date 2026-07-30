// home.js — Chat 主页面逻辑
// 详见 docs/frontend/03-web.md §3.1

let models = [];
let configuredProviders = new Set();
let currentStream = null;
let activeConvId = null;
let activeConvTitle = '';
// 当前输入的附件：[{file_id, filename, mime_type, size, dataUrl?}]
let pendingAttachments = [];

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
const fileInput = document.getElementById('file-input');
const attachBtn = document.getElementById('attach-btn');
const attachPreview = document.getElementById('attachments-preview');

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
    if (e.key === 'Enter' && !e.shiftKey && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      send();
    }
  });

  // textarea 自动撑高（最多 200px）
  if (userInput) {
    const autoresize = () => {
      userInput.style.height = 'auto';
      const h = Math.min(userInput.scrollHeight, 200);
      userInput.style.height = h + 'px';
    };
    userInput.addEventListener('input', autoresize);
    autoresize();
  }

  // 附件按钮 → 触发隐藏的 file input
  if (attachBtn && fileInput) {
    attachBtn.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', handleFiles);
  }
}

// 选文件后：上传 + 加到 pendingAttachments
async function handleFiles(e) {
  const files = Array.from(e.target.files || []);
  e.target.value = ''; // 清空让同名文件可再选
  for (const file of files) {
    const tempId = 'tmp_' + Date.now() + '_' + Math.random().toString(36).slice(2, 8);
    const isImage = file.type.startsWith('image/');
    let dataUrl = null;
    if (isImage) {
      dataUrl = await readAsDataURL(file);
    }
    pendingAttachments.push({
      tempId,
      file_id: null,
      filename: file.name,
      mime_type: file.type || 'application/octet-stream',
      size: file.size,
      dataUrl,
      uploading: true,
    });
    renderAttachments();
    try {
      const resp = await App.fetch('/api/v1/files', {
        method: 'POST',
        body: (() => { const fd = new FormData(); fd.append('file', file); return fd; })(),
      });
      const idx = pendingAttachments.findIndex(a => a.tempId === tempId);
      if (idx >= 0) {
        pendingAttachments[idx].file_id = resp.id;
        pendingAttachments[idx].uploading = false;
      }
    } catch (err) {
      const idx = pendingAttachments.findIndex(a => a.tempId === tempId);
      if (idx >= 0) {
        pendingAttachments[idx].uploading = false;
        pendingAttachments[idx].error = err.message;
      }
    }
    renderAttachments();
  }
}

function readAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const fr = new FileReader();
    fr.onload = () => resolve(fr.result);
    fr.onerror = () => reject(fr.error || new Error('read fail'));
    fr.readAsDataURL(file);
  });
}

function renderAttachments() {
  if (!attachPreview) return;
  attachPreview.innerHTML = '';
  for (const a of pendingAttachments) {
    const chip = document.createElement('div');
    chip.className = 'attach-chip' + (a.uploading ? ' attach-uploading' : '') + (a.error ? ' attach-error' : '');
    const thumb = document.createElement('div');
    thumb.className = 'attach-thumb';
    if (a.dataUrl) {
      const img = document.createElement('img');
      img.src = a.dataUrl;
      img.alt = a.filename;
      img.style.width = '100%';
      img.style.height = '100%';
      img.style.objectFit = 'cover';
      img.style.borderRadius = '4px';
      thumb.appendChild(img);
    } else {
      thumb.textContent = iconForMime(a.mime_type);
    }
    chip.appendChild(thumb);
    const name = document.createElement('span');
    name.className = 'attach-name';
    name.textContent = a.filename;
    chip.appendChild(name);
    const size = document.createElement('span');
    size.className = 'attach-size';
    size.textContent = formatSize(a.size);
    chip.appendChild(size);
    const rm = document.createElement('button');
    rm.className = 'attach-remove';
    rm.type = 'button';
    rm.textContent = '×';
    rm.title = '移除';
    rm.addEventListener('click', () => {
      pendingAttachments = pendingAttachments.filter(x => x.tempId !== a.tempId);
      renderAttachments();
    });
    chip.appendChild(rm);
    attachPreview.appendChild(chip);
  }
}

function iconForMime(mime) {
  if (mime.startsWith('image/')) return '🖼';
  if (mime === 'application/pdf') return '📕';
  if (mime.startsWith('text/')) return '📄';
  if (mime.includes('json')) return '{ }';
  return '📎';
}

function formatSize(n) {
  if (n < 1024) return n + 'B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + 'K';
  return (n / (1024 * 1024)).toFixed(1) + 'M';
}

// 取已成功上传的 file_id 列表
function readyAttachmentIds() {
  return pendingAttachments
    .filter(a => a.file_id && !a.error)
    .map(a => a.file_id);
}

function clearAttachments() {
  pendingAttachments = [];
  renderAttachments();
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
    modelMultiHint.textContent = '请选 1 个或多个 provider';
    modelMultiHint.className = 'multi-hint multi-hint-warn';
  } else if (n === 1) {
    modelMultiHint.textContent = '已选 1 个 provider（居中显示）';
    modelMultiHint.className = 'multi-hint multi-hint-ok';
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

function addCompareResults(results) {
  // 删除旧 grid（如有）
  const old = messagesEl.querySelector('.compare-block');
  if (old) old.remove();

  const block = document.createElement('div');
  block.className = 'compare-block';

  // 1 个 provider → 居中显示
  // 2 个 provider → 并排
  // 3 个+ → 网格自适应
  const isSingle = results.length === 1;
  if (isSingle) block.classList.add('compare-block-single');
  const grid = document.createElement('div');
  grid.className = 'compare-grid';

  for (const r of results) {
    const el = document.createElement('div');
    el.className = 'compare-result' + (r.status === 'failed' ? ' error' : '');
    const head = document.createElement('div');
    head.className = 'compare-head';
    head.innerHTML = `
      <span class="compare-provider">${escapeHtml(r.provider)}</span>
      <span class="compare-latency">${r.latency_ms}ms</span>
      <span class="compare-status compare-status-${r.status}">${r.status}</span>
    `;
    el.appendChild(head);

    // tab 切换：渲染 / 源码
    const tabs = document.createElement('div');
    tabs.className = 'compare-tabs';
    const tabRender = document.createElement('button');
    tabRender.className = 'compare-tab active';
    tabRender.textContent = '渲染';
    tabRender.type = 'button';
    const tabSource = document.createElement('button');
    tabSource.className = 'compare-tab';
    tabSource.textContent = '源码';
    tabSource.type = 'button';
    tabs.appendChild(tabRender);
    tabs.appendChild(tabSource);
    el.appendChild(tabs);

    // 渲染视图
    const rendered = document.createElement('div');
    rendered.className = 'compare-body compare-body-rendered';
    const raw = r.status === 'succeeded' ? (r.content || '(空响应)') : (r.error?.message || '失败');
    rendered.innerHTML = renderMarkdown(raw);
    el.appendChild(rendered);

    // 源码视图
    const source = document.createElement('pre');
    source.className = 'compare-body compare-body-source';
    source.hidden = true;
    source.textContent = raw;
    el.appendChild(source);

    tabRender.addEventListener('click', () => {
      tabRender.classList.add('active');
      tabSource.classList.remove('active');
      rendered.hidden = false;
      source.hidden = true;
    });
    tabSource.addEventListener('click', () => {
      tabSource.classList.add('active');
      tabRender.classList.remove('active');
      source.hidden = false;
      rendered.hidden = true;
    });

    grid.appendChild(el);
  }
  block.appendChild(grid);

  // 综合结论（≥2 个成功响应时）
  const succeeded = results.filter(r => r.status === 'succeeded' && r.content);
  if (succeeded.length >= 2) {
    addCompareSummary(block, succeeded, results);
  }

  messagesEl.appendChild(block);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return block;
}

// 综合结论区：本地分析所有成功响应，生成对比总结
function addCompareSummary(block, succeeded, allResults) {
  // 统计：最快 / 最慢 / 平均延迟 / 字数
  const latencies = succeeded.map(r => r.latency_ms);
  const minLat = Math.min(...latencies);
  const maxLat = Math.max(...latencies);
  const avgLat = Math.round(latencies.reduce((a, b) => a + b, 0) / latencies.length);
  const lengths = succeeded.map(r => r.content.length);
  const maxLen = Math.max(...lengths);
  const minLen = Math.min(...lengths);

  // 公共片段：所有响应都包含的字符子串（最长公共前缀近似）
  // 简单实现：找最短的响应的所有 30 字以上子串，看其它响应是否包含
  const shortest = succeeded.reduce((a, b) => a.content.length <= b.content.length ? a : b);
  const longest = succeeded.reduce((a, b) => a.content.length >= b.content.length ? a : b);

  // 找分歧点：shortest 中第一个不在 longest 中出现的长连续片段
  const divergence = findDivergence(shortest.content, longest.content);

  // 共识：所有响应都包含的最长前缀
  let commonPrefix = succeeded[0].content;
  for (const r of succeeded.slice(1)) {
    let i = 0;
    while (i < commonPrefix.length && i < r.content.length && commonPrefix[i] === r.content[i]) i++;
    commonPrefix = commonPrefix.slice(0, i);
  }

  const summary = document.createElement('div');
  summary.className = 'compare-summary';
  summary.innerHTML = `
    <div class="compare-summary-head">
      <span class="compare-summary-icon">📊</span>
      <span class="compare-summary-title">综合结论</span>
      <span class="compare-summary-count">${succeeded.length} 个成功 / ${(allResults||results).length - succeeded.length} 个失败</span>
    </div>
    <div class="compare-summary-stats">
      <div class="stat"><span class="stat-label">最快</span><span class="stat-value">${minLat}ms</span></div>
      <div class="stat"><span class="stat-label">最慢</span><span class="stat-value">${maxLat}ms</span></div>
      <div class="stat"><span class="stat-label">平均</span><span class="stat-value">${avgLat}ms</span></div>
      <div class="stat"><span class="stat-label">回答长度</span><span class="stat-value">${minLen}~${maxLen} 字</span></div>
    </div>
    ${commonPrefix.length > 20 ? `
    <div class="compare-summary-section">
      <div class="summary-label">📌 共识（所有回答共有部分）</div>
      <div class="summary-body summary-common">${renderMarkdown(commonPrefix + (commonPrefix.length < shortest.content.length ? '…' : ''))}</div>
    </div>` : ''}
    ${divergence ? `
    <div class="compare-summary-section">
      <div class="summary-label">🔀 分歧（仅 ${escapeHtml(shortest.provider)} 独有）</div>
      <div class="summary-body summary-divergence">${renderMarkdown(divergence)}</div>
    </div>` : ''}
    <div class="compare-summary-section">
      <div class="summary-label">📈 各家差异速览</div>
      <div class="summary-body">
        ${succeeded.map(r => `<div class="summary-row">
          <span class="summary-row-provider">${escapeHtml(r.provider)}</span>
          <span class="summary-row-meta">${r.latency_ms}ms · ${r.content.length} 字</span>
        </div>`).join('')}
      </div>
    </div>
  `;
  block.appendChild(summary);
}

// 找 shortest 中独有、longest 中不存在的最长片段
function findDivergence(shortest, longest) {
  if (!shortest || !longest) return '';
  const sl = shortest.length;
  const ll = longest.length;
  // 取最短的前 100 字和最后 100 字之间搜索
  const searchStart = Math.min(50, Math.floor(sl / 4));
  const searchEnd = Math.max(sl - 50, Math.floor(sl * 3 / 4));
  if (searchStart >= searchEnd) return '';

  // 简单做法：取 shortest 中 [searchStart, searchEnd) 区间，看 longest 中是否包含
  // 如果不包含，就是分歧
  for (let len = searchEnd - searchStart; len >= 30; len -= 10) {
    for (let i = searchStart; i + len <= searchEnd; i++) {
      const frag = shortest.slice(i, i + len);
      if (!longest.includes(frag)) {
        // 扩展找到最大边界
        let start = i, end = i + len;
        while (start > searchStart && !longest.includes(shortest.slice(start - 5, end))) start -= 5;
        while (end < searchEnd && !longest.includes(shortest.slice(start, end + 5))) end += 5;
        return shortest.slice(Math.max(0, start - 10), Math.min(sl, end + 10)) + '…';
      }
    }
  }
  return '';
}

// 简单 markdown 渲染（**bold** / *italic* / `code` / ``` block / [text](url) / \n 换行）
// 不引入第三方库，够 95% 场景用
function renderMarkdown(text) {
  if (!text) return '';
  let html = escapeHtml(text);

  // ```code block```
  html = html.replace(/```([\s\S]*?)```/g, (m, code) => {
    return '<pre class="md-code-block"><code>' + code.replace(/\n$/, '') + '</code></pre>';
  });

  // 行内 code `xxx`
  html = html.replace(/`([^`\n]+)`/g, '<code class="md-code-inline">$1</code>');

  // **bold** 和 __bold__
  html = html.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/__([^_\n]+)__/g, '<strong>$1</strong>');

  // *italic* 和 _italic_
  html = html.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
  html = html.replace(/(^|[^_])_([^_\n]+)_/g, '$1<em>$2</em>');

  // 链接 [text](url)
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');

  // headers # ## ###
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');

  // 列表 - item
  html = html.replace(/^- (.+)$/gm, '<li>$1</li>');
  html = html.replace(/(<li>.*<\/li>\n?)+/g, (m) => '<ul>' + m + '</ul>');

  // 数字列表 1. 2.
  html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');

  // 段落：双换行 → <p>，单换行 → <br>
  // 先把 \n\n 切成段
  const paras = html.split(/\n{2,}/);
  html = paras.map(p => {
    if (/^<(h\d|ul|pre|li)/.test(p.trim())) return p;
    return '<p>' + p.replace(/\n/g, '<br>') + '</p>';
  }).join('\n');

  return html;
}

async function send() {
  const content = userInput.value.trim();
  if (!content) return;

  const mode = modeSel.value;
  const atts = readyAttachmentIds();

  if (mode === 'compare') {
    const providers = getSelectedProviders();
    if (providers.length < 1) {
      showToast('对比模式请至少选 1 个 provider');
      return;
    }
    await sendCompare(content, providers, atts);
    return;
  }

  const model = mode === 'auto' ? 'auto' : modelSel.value;
  const messages = [];
  const sysPrompt = App.getSystemPrompt();
  if (sysPrompt) messages.push({ role: 'system', content: sysPrompt });
  messages.push({ role: 'user', content });

  const req = { model, messages, conv_id: activeConvId || undefined };
  if (atts.length) req.attachments = atts;

  addMessage('user', content, atts.length ? `[附件] ${atts.length} 个` : null);
  userInput.value = '';
  clearAttachments();
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

async function sendCompare(content, providers, atts) {
  addMessage('user', content, atts.length ? `[附件] ${atts.length} 个` : null);
  userInput.value = '';
  clearAttachments();
  setStreaming(true);

  const messages = [];
  const sysPrompt = App.getSystemPrompt();
  if (sysPrompt) messages.push({ role: 'system', content: sysPrompt });
  messages.push({ role: 'user', content });
  if (atts.length) messages[messages.length - 1].attachments = atts;

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
    addCompareResults(results);
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