// history.js — 历史对话页（1.0 阶段只显示占位）
// 详见 docs/frontend/03-web.md §十
// 1.0 阶段：后端 conversations 接口未实现，提示用户"功能即将上线"
(async function() {
  const list = document.getElementById('conv-list');
  const empty = document.querySelector('.muted');
  // 1.0 简化：直接显示"功能即将上线"
  empty.textContent = '历史对话功能即将上线（Phase 1.1）';
  empty.style.display = 'block';
  list.style.display = 'none';
})();
