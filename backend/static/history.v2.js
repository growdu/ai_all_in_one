// history.js — 历史对话页（Phase 1.1.1 + v1.0.1 重构：去 IIFE）
// 详见 docs/frontend/03-web.md §十 + docs/api/01-protocol.md §1.5
//
// v1.0.1: 去掉 (async function(){...})() 包装。
// 原 IIFE 模式在某些时序下 load() 不会被触发（IIFE 内 const App = window.App
// 与 DOMContentLoaded 触发竞态）。改成与 home.js 一致的 top-level 风格。

const listEl = document.getElementById('conv-list');
const emptyEl = document.querySelector('.muted');

async function load() {
  try {
    const resp = await App.fetch('/api/v1/conversations');
    const convs = resp.conversations || [];
    render(convs);
  } catch (e) {
    showToast(I18n.t('errors.internal_error') + ': ' + e.message);
    if (emptyEl) {
      emptyEl.textContent = e.message;
      emptyEl.style.display = 'block';
    }
    if (listEl) listEl.style.display = 'none';
  }
}

function render(convs) {
  if (!listEl) return;
  listEl.innerHTML = '';
  if (!convs.length) {
    if (emptyEl) {
      emptyEl.textContent = I18n.t('history.empty');
      emptyEl.style.display = 'block';
    }
    listEl.style.display = 'none';
    return;
  }
  if (emptyEl) emptyEl.style.display = 'none';
  listEl.style.display = 'flex';

  // 顶部"新建对话"按钮
  const newBtn = document.createElement('button');
  newBtn.className = 'btn-primary';
  newBtn.style.marginBottom = '12px';
  newBtn.style.width = '100%';
  newBtn.textContent = '+ ' + (I18n.lang === 'zh' ? '新建对话' : 'New Chat');
  newBtn.addEventListener('click', createConv);
  listEl.appendChild(newBtn);

  for (const c of convs) {
    const item = document.createElement('div');
    item.className = 'conv-item';
    item.setAttribute('role', 'listitem');
    const pin = c.pinned ? '📌 ' : '';
    item.innerHTML = `
      <div class="title">${pin}${escapeHtml(c.title || '新对话')}</div>
      <div class="meta">${c.model} · ${formatTime(c.updated_at)} · ${c.id.substring(0, 12)}...</div>
      <div style="margin-top: 8px; display: flex; gap: 8px;">
        <button class="btn-secondary" data-action="open" data-id="${c.id}">${I18n.t('history.open') || '打开'}</button>
        <button class="btn-secondary" data-action="rename" data-id="${c.id}">${I18n.t('history.rename') || '改名'}</button>
        <button class="btn-danger" data-action="delete" data-id="${c.id}">${I18n.t('settings.delete')}</button>
      </div>
    `;
    listEl.appendChild(item);
  }
}

function onAction(e) {
  const btn = e.target.closest('button[data-action]');
  if (!btn) return;
  const action = btn.dataset.action;
  const id = btn.dataset.id;
  if (action === 'open') openConv(id);
  else if (action === 'rename') renameConv(id);
  else if (action === 'delete') deleteConv(id);
}

async function openConv(id) {
  try {
    const conv = await App.fetch('/api/v1/conversations/' + encodeURIComponent(id));
    localStorage.setItem('aiio.active_conv', JSON.stringify(conv));
    location.href = 'home.html?conv=' + encodeURIComponent(id);
  } catch (e) {
    showToast('打开失败：' + e.message);
  }
}

async function renameConv(id) {
  const newTitle = prompt(I18n.t('history.rename') || '新标题');
  if (!newTitle) return;
  try {
    await App.fetch('/api/v1/conversations/' + encodeURIComponent(id), {
      method: 'PATCH',
      body: { title: newTitle },
    });
    load();
  } catch (e) {
    showToast('改名失败：' + e.message);
  }
}

async function deleteConv(id) {
  if (!confirm(I18n.t('history.confirmDelete') || '确认删除？')) return;
  try {
    await App.fetch('/api/v1/conversations/' + encodeURIComponent(id), { method: 'DELETE' });
    load();
  } catch (e) {
    showToast('删除失败：' + e.message);
  }
}

async function createConv() {
  try {
    const c = await App.fetch('/api/v1/conversations', {
      method: 'POST',
      body: { model: 'mock-echo' },
    });
    openConv(c.id);
  } catch (e) {
    showToast('创建失败：' + e.message);
  }
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[ch]));
}

function formatTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  const now = new Date();
  const diff = (now - d) / 1000;
  if (diff < 60) return '刚刚';
  if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前';
  if (diff < 86400) return Math.floor(diff / 3600) + ' 小时前';
  if (diff < 604800) return Math.floor(diff / 86400) + ' 天前';
  return d.toLocaleDateString();
}

// 启动：DOMContentLoaded 触发时 load()，已 ready 就立即跑
// 用 rAF 等一帧确保 DOM 渲染完毕（保险）
function start() {
  load();
}
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', start, { once: true });
} else {
  // DOM 已 ready 但脚本可能在 DOM 完全渲染前跑（罕见）
  // rAF 等下一帧
  requestAnimationFrame(start);
}

// list 上 click 委托
listEl && listEl.addEventListener('click', onAction);