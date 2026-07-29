// i18n.js — 极简 i18n（zh/en）+ 主题切换
// 1.0 简化：localStorage 存 lang 和 theme
// 详见 docs/frontend/03-web.md §八

const translations = {
  zh: {
    app: { title: 'AI 助手', settings: '设置', theme: '主题' },
    theme: { light: '浅色', dark: '深色', auto: '自动' },
    onboarding: {
      welcome: '欢迎使用 AI 助手',
      intro: '先选一个 AI 服务配 API Key，开始使用',
      skip: '跳过',
      configured: '已配 Key',
      notConfigured: '未配',
    },
    home: { title: '对话' },
    mode: { single: '单个', auto: '自动选', compare: '对比' },
    chat: {
      placeholder: '输入消息...',
      send: '发送',
      stop: '停止',
      thinking: '思考中...',
    },
    settings: {
      title: '设置',
      apiKeys: '我的 API Key',
      addKey: '添加 Key',
      save: '保存',
      delete: '删除',
      systemPrompt: 'AI 角色 (System Prompt)',
      systemPromptPlaceholder: '例：你是一个 X 领域的资深工程师，回答用简体中文...',
      advanced: '高级参数',
      temperature: '温度',
      maxTokens: '最大长度',
      keyPlaceholder: 'sk-...',
    },
    history: {
      title: '历史对话',
      empty: '暂无历史对话',
    },
    errors: {
      auth_missing: '请先登录',
      auth_invalid: '登录已过期',
      model_not_found: '该模型暂不可用',
      no_provider_configured: '请先在设置里配置至少一个 AI 服务的 Key',
      no_capable_provider: '当前问题需要的能力没有可用的模型',
      user_rate_limit: '请求过于频繁',
      provider_rate_limit: '服务商繁忙',
      system_overload: '服务暂时繁忙',
      upstream_timeout: '网络超时',
      only_one_provider: '对比模式需要至少 2 个 AI 服务',
      all_providers_failed: '所有 AI 服务都失败了',
      internal_error: '服务暂时不可用',
    },
  },
  en: {
    app: { title: 'AI Assistant', settings: 'Settings', theme: 'Theme' },
    theme: { light: 'Light', dark: 'Dark', auto: 'Auto' },
    onboarding: {
      welcome: 'Welcome to AI Assistant',
      intro: 'Choose an AI service to add an API key, then start using it',
      skip: 'Skip',
      configured: 'Configured',
      notConfigured: 'Not set',
    },
    home: { title: 'Chat' },
    mode: { single: 'Single', auto: 'Auto', compare: 'Compare' },
    chat: {
      placeholder: 'Type a message...',
      send: 'Send',
      stop: 'Stop',
      thinking: 'Thinking...',
    },
    settings: {
      title: 'Settings',
      apiKeys: 'My API Keys',
      addKey: 'Add Key',
      save: 'Save',
      delete: 'Delete',
      systemPrompt: 'AI Role (System Prompt)',
      systemPromptPlaceholder: 'e.g. You are a senior Go engineer, reply in Chinese...',
      advanced: 'Advanced',
      temperature: 'Temperature',
      maxTokens: 'Max tokens',
      keyPlaceholder: 'sk-...',
    },
    history: {
      title: 'History',
      empty: 'No conversations yet',
    },
    errors: {
      auth_missing: 'Please sign in first',
      auth_invalid: 'Your session has expired',
      model_not_found: 'This model is currently unavailable',
      no_provider_configured: 'Please configure at least one AI service key in Settings',
      no_capable_provider: 'No available model supports the required capability',
      user_rate_limit: 'Too many requests',
      provider_rate_limit: 'The AI service is busy',
      system_overload: 'The service is busy',
      upstream_timeout: 'Network timeout',
      only_one_provider: 'Compare mode requires at least 2 AI services',
      all_providers_failed: 'All AI services failed',
      internal_error: 'Service temporarily unavailable',
    },
  },
};

const I18n = {
  get lang() {
    return localStorage.getItem('aiio.lang') || 'zh';
  },
  set lang(v) {
    localStorage.setItem('aiio.lang', v);
    I18n.apply();
  },
  t(key) {
    const lang = translations[I18n.lang] || translations.zh;
    const parts = key.split('.');
    let cur = lang;
    for (const p of parts) {
      if (cur == null) return key;
      cur = cur[p];
    }
    return cur == null ? key : cur;
  },
  apply(root = document) {
    root.querySelectorAll('[data-i18n]').forEach(el => {
      el.textContent = I18n.t(el.getAttribute('data-i18n'));
    });
    root.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
      el.placeholder = I18n.t(el.getAttribute('data-i18n-placeholder'));
    });
    root.querySelectorAll('[data-i18n-aria-label]').forEach(el => {
      el.setAttribute('aria-label', I18n.t(el.getAttribute('data-i18n-aria-label')));
    });
  },

  // ---- 主题 ----
  get theme() {
    return localStorage.getItem('aiio.theme') || 'auto';
  },
  set theme(v) {
    localStorage.setItem('aiio.theme', v);
    I18n.applyTheme();
  },
  applyTheme() {
    const t = I18n.theme;
    if (t === 'auto') {
      document.documentElement.removeAttribute('data-theme');
    } else {
      document.documentElement.setAttribute('data-theme', t);
    }
  },

  // ---- 顶栏初始化（语言 + 主题） ----
  initSwitch() {
    const langSel = document.getElementById('lang-switch');
    if (langSel) {
      langSel.value = I18n.lang;
      langSel.addEventListener('change', () => { I18n.lang = langSel.value; });
    }
    const themeSel = document.getElementById('theme-switch');
    if (themeSel) {
      themeSel.value = I18n.theme;
      themeSel.addEventListener('change', () => { I18n.theme = themeSel.value; });
    }
  },
};

document.addEventListener('DOMContentLoaded', () => {
  I18n.applyTheme();
  I18n.initSwitch();
  I18n.apply();
});
