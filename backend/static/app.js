// app.js — 共享 API 客户端 + 状态
// 1.0 简化：用 fetch + localStorage 做会话管理

const App = {
  // 后端地址：同源（同一个 binary serve 静态文件 + API）
  baseURL: '',
  // dev token 1.0 简化：用户第一次访问时从 URL 拿 / 自己填
  // 2.0 接 JWT
  get token() {
    return localStorage.getItem('aiio.token') || 'devtoken';
  },
  set token(v) {
    localStorage.setItem('aiio.token', v);
  },

  // 通用 fetch 封装
  async fetch(path, opts = {}) {
    const headers = opts.headers || {};
    if (!headers['Authorization']) {
      headers['Authorization'] = 'Bearer ' + this.token;
    }
    if (opts.body && typeof opts.body === 'object' && !headers['Content-Type']) {
      headers['Content-Type'] = 'application/json';
    }
    const resp = await fetch(this.baseURL + path, { ...opts, headers });
    if (resp.status === 204) return null;
    const ct = resp.headers.get('Content-Type') || '';
    if (ct.includes('application/json')) {
      const data = await resp.json();
      if (!resp.ok) {
        const code = data?.error?.code || 'internal_error';
        const msg = I18n.t('errors.' + code) || data?.error?.message || code;
        throw new Error(msg);
      }
      return data;
    }
    if (!resp.ok) {
      throw new Error('HTTP ' + resp.status);
    }
    return resp;
  },

  // GET /api/v1/models
  async listModels() {
    return this.fetch('/api/v1/models');
  },

  // POST /api/v1/keys
  async putKey(provider, key) {
    return this.fetch('/api/v1/keys', {
      method: 'POST',
      body: { provider, key },
    });
  },

  // GET /api/v1/keys
  async listKeys() {
    return this.fetch('/api/v1/keys');
  },

  // DELETE /api/v1/keys/{provider}
  async deleteKey(provider) {
    return this.fetch('/api/v1/keys/' + encodeURIComponent(provider), { method: 'DELETE' });
  },

  // POST /api/v1/chat/completions (non-streaming)
  async chat(req) {
    return this.fetch('/api/v1/chat/completions', {
      method: 'POST',
      body: { ...req, stream: false },
    });
  },

  // POST /api/v1/chat/completions (streaming)
  // onChunk(delta) called per SSE chunk
  // returns { close: () => void }
  chatStream(req, onChunk, onError, onDone) {
    const controller = new AbortController();
    const body = JSON.stringify({ ...req, stream: true });
    fetch(this.baseURL + '/api/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + this.token,
      },
      body,
      signal: controller.signal,
    }).then(async resp => {
      if (!resp.ok) {
        let err;
        try { err = await resp.json(); } catch (_) { err = null; }
        const code = err?.error?.code || 'internal_error';
        const msg = I18n.t('errors.' + code) || err?.error?.message || ('HTTP ' + resp.status);
        onError && onError(new Error(msg));
        return;
      }
      const reader = resp.body.getReader();
      const dec = new TextDecoder();
      let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += dec.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop();
        for (const line of lines) {
          if (line.startsWith('data: ') && line !== 'data: [DONE]') {
            try {
              const chunk = JSON.parse(line.slice(6));
              onChunk && onChunk(chunk);
            } catch (e) {
              // ignore
            }
          }
        }
      }
      onDone && onDone();
    }).catch(err => {
      if (err.name !== 'AbortError') onError && onError(err);
    });
    return {
      close: () => controller.abort(),
    };
  },

  // ---- 用户偏好（localStorage） ----
  getSystemPrompt() {
    return localStorage.getItem('aiio.system_prompt') || '';
  },
  setSystemPrompt(v) {
    localStorage.setItem('aiio.system_prompt', v);
  },
  getAdvanced() {
    try { return JSON.parse(localStorage.getItem('aiio.advanced') || '{}'); }
    catch { return {}; }
  },
  setAdvanced(v) {
    localStorage.setItem('aiio.advanced', JSON.stringify(v));
  },
};
