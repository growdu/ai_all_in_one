// settings.js — 设置页：API Key 管理 + AI 角色 + 高级参数
// 详见 docs/frontend/03-web.md §3.2-3.5

(async function() {
  const providerKeys = document.getElementById('provider-keys');
  const systemPrompt = document.getElementById('system-prompt');
  const saveSystemPrompt = document.getElementById('save-system-prompt');
  const temperature = document.getElementById('temperature');
  const temperatureVal = document.getElementById('temperature-val');
  const maxTokens = document.getElementById('max-tokens');

  // 加载已配 key
  let configured = [];
  try {
    const keys = await App.listKeys();
    configured = keys.providers || [];
  } catch (e) {}
  const configuredSet = new Set(configured);

  // 加载模型列表 → 派生 provider 列表
  let models = [];
  try {
    const resp = await App.listModels();
    models = resp.models || [];
  } catch (e) {}

  const byProvider = {};
  for (const m of models) {
    if (!byProvider[m.provider]) byProvider[m.provider] = [];
    byProvider[m.provider].push(m);
  }

  // URL ?provider=xxx 预填
  const preselect = new URLSearchParams(location.search).get('provider');

  for (const provider of Object.keys(byProvider)) {
    const row = document.createElement('div');
    row.className = 'key-row';
    const isSet = configuredSet.has(provider);
    row.innerHTML = `
      <div class="name">${provider}</div>
      <span class="status ${isSet ? '' : 'unset'}">${isSet ? '已配' : '未配'}</span>
      <input type="password" placeholder="${I18n.t('settings.keyPlaceholder')}" />
      <button class="btn-primary">${isSet ? '更新' : I18n.t('settings.addKey')}</button>
      <button class="btn-secondary" ${isSet ? '' : 'disabled'}>${I18n.t('settings.delete')}</button>
    `;
    const [nameEl, statusEl, input, saveBtn, delBtn] = row.children;
    if (provider === preselect) input.focus();

    saveBtn.addEventListener('click', async () => {
      const key = input.value.trim();
      if (!key) return;
      try {
        await App.putKey(provider, key);
        input.value = '';
        statusEl.className = 'status';
        statusEl.textContent = '已配';
        delBtn.disabled = false;
        saveBtn.textContent = '更新';
      } catch (e) {
        alert('保存失败：' + e.message);
      }
    });

    delBtn.addEventListener('click', async () => {
      if (!confirm('删除 ' + provider + ' 的 key？')) return;
      try {
        await App.deleteKey(provider);
        statusEl.className = 'status unset';
        statusEl.textContent = '未配';
        delBtn.disabled = true;
        saveBtn.textContent = I18n.t('settings.addKey');
      } catch (e) {
        alert('删除失败：' + e.message);
      }
    });

    providerKeys.appendChild(row);
  }

  // System prompt
  systemPrompt.value = App.getSystemPrompt();
  saveSystemPrompt.addEventListener('click', () => {
    App.setSystemPrompt(systemPrompt.value);
    saveSystemPrompt.textContent = '✓ 已保存';
    setTimeout(() => saveSystemPrompt.textContent = I18n.t('settings.save'), 1500);
  });

  // 高级参数
  const adv = App.getAdvanced();
  if (typeof adv.temperature === 'number') {
    temperature.value = adv.temperature;
  }
  if (typeof adv.max_tokens === 'number') {
    maxTokens.value = adv.max_tokens;
  }
  temperatureVal.textContent = parseFloat(temperature.value).toFixed(1);
  temperature.addEventListener('input', () => {
    temperatureVal.textContent = parseFloat(temperature.value).toFixed(1);
    App.setAdvanced({ ...App.getAdvanced(), temperature: parseFloat(temperature.value) });
  });
  maxTokens.addEventListener('change', () => {
    App.setAdvanced({ ...App.getAdvanced(), max_tokens: parseInt(maxTokens.value) });
  });
})();
