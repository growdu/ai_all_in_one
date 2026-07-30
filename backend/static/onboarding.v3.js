// onboarding.js — Onboarding 页面逻辑
// 详见 docs/frontend/03-web.md §3.6

(async function() {
  const grid = document.getElementById('provider-grid');
  let models, configured;

  try {
    const modelsResp = await App.listModels();
    models = modelsResp.models || [];
  } catch (e) {
    grid.innerHTML = `<p class="muted">无法加载模型列表：${e.message}</p>`;
    return;
  }

  try {
    const keysResp = await App.listKeys();
    configured = new Set(keysResp.providers || []);
  } catch (e) {
    configured = new Set();
  }

  // 按 provider 分组
  const byProvider = {};
  for (const m of models) {
    if (!byProvider[m.provider]) byProvider[m.provider] = [];
    byProvider[m.provider].push(m);
  }

  // 渲染 provider 卡片
  const order = ['doubao', 'openai', 'deepseek', 'kimi', 'claude', 'mock', 'slow'];
  const sorted = Object.keys(byProvider).sort((a, b) => {
    const ia = order.indexOf(a); const ib = order.indexOf(b);
    return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib);
  });

  for (const provider of sorted) {
    const models = byProvider[provider];
    const isConfigured = configured.has(provider);
    const card = document.createElement('div');
    card.className = 'provider-card' + (isConfigured ? ' configured' : '');
    card.innerHTML = `
      <div class="name">${provider}</div>
      <div class="desc">${models.length} model${models.length > 1 ? 's' : ''}</div>
      <div class="status ${isConfigured ? '' : 'unset'}">
        ${isConfigured ? I18n.t('onboarding.configured') : I18n.t('onboarding.notConfigured')}
      </div>
    `;
    card.addEventListener('click', () => {
      // 跳到 settings 页面预填 provider
      location.href = 'settings.html?provider=' + encodeURIComponent(provider);
    });
    grid.appendChild(card);
  }
})();
