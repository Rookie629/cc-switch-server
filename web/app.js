// cc-switch-server web panel
const API = '/api';

let providers = [];
let presets = [];

// ---- Init ----
async function init() {
  await Promise.all([loadProviders(), loadPresets(), loadProxyStatus()]);
  render();

  document.getElementById('btn-add').addEventListener('click', showAddForm);
  document.getElementById('btn-cancel').addEventListener('click', showList);
  document.getElementById('provider-form').addEventListener('submit', saveProvider);
  document.getElementById('btn-test').addEventListener('click', testConnection);
  document.getElementById('btn-toggle-key').addEventListener('click', toggleKeyVisibility);
  document.getElementById('form-preset').addEventListener('change', applyPreset);
}

// ---- Data loading ----
async function loadProviders() {
  const resp = await fetch(API + '/providers');
  const data = await resp.json();
  providers = data.providers || [];
}

async function loadPresets() {
  const resp = await fetch(API + '/presets');
  const data = await resp.json();
  presets = data.presets || [];

  const sel = document.getElementById('form-preset');
  presets.forEach(p => {
    const opt = document.createElement('option');
    opt.value = p.name;
    opt.textContent = p.name + ' (' + p.type + ')';
    sel.appendChild(opt);
  });
}

async function loadProxyStatus() {
  const resp = await fetch(API + '/proxy/status');
  const data = await resp.json();
  const badge = document.getElementById('proxy-badge');
  if (data.running) {
    badge.textContent = '代理 ON :' + data.port;
    badge.className = 'badge active';
  } else {
    badge.textContent = '代理 OFF';
    badge.className = 'badge';
  }
}

// ---- Rendering ----
function render() {
  const container = document.getElementById('provider-list');
  const active = providers.find(p => p.is_active);

  if (providers.length === 0) {
    container.innerHTML = '<div class="empty-state"><p>还没有 Provider</p><p>点击"+ 添加" 开始</p></div>';
    return;
  }

  container.innerHTML = providers.map(p => `
    <div class="provider-card ${p.is_active ? 'active' : ''}">
      <div class="provider-info">
        <h3>${p.is_active ? '&#9679; ' : '&#9675; '}${esc(p.name)}</h3>
        <div class="meta">
          <span>${esc(p.type)}</span>
          <span>${esc(p.base_url)}</span>
          <span>${maskKey(p.api_key)}</span>
        </div>
      </div>
      <div class="provider-actions">
        ${!p.is_active ? `<button class="btn-primary btn-sm" onclick="switchProvider('${esc(p.name)}')">启用</button>` : ''}
        <button class="btn-sm" onclick="editProvider('${esc(p.name)}')">编辑</button>
        ${!p.is_active ? `<button class="btn-sm btn-danger" onclick="deleteProvider('${esc(p.name)}')">删除</button>` : ''}
      </div>
    </div>
  `).join('');
}

// ---- Actions ----
async function switchProvider(name) {
  if (!confirm('切换到 ' + name + '？')) return;
  const resp = await fetch(API + '/providers/switch/' + encodeURIComponent(name), { method: 'POST' });
  const data = await resp.json();
  if (data.status === 'ok') {
    await loadProviders();
    await loadProxyStatus();
    render();
  } else {
    alert('切换失败: ' + (data.error || 'unknown'));
  }
}

async function deleteProvider(name) {
  if (!confirm('确定删除 ' + name + '？')) return;
  const resp = await fetch(API + '/providers/' + encodeURIComponent(name), { method: 'DELETE' });
  if (resp.ok) {
    await loadProviders();
    render();
  }
}

function showAddForm() {
  document.getElementById('view-list').style.display = 'none';
  document.getElementById('view-form').style.display = 'block';
  document.getElementById('form-title').textContent = '添加 Provider';
  document.getElementById('form-editing').value = '';
  resetForm();
}

function editProvider(name) {
  const p = providers.find(p => p.name === name);
  if (!p) return;
  document.getElementById('view-list').style.display = 'none';
  document.getElementById('view-form').style.display = 'block';
  document.getElementById('form-title').textContent = '编辑: ' + p.name;
  document.getElementById('form-editing').value = p.name;
  document.getElementById('form-name').value = p.name;
  document.getElementById('form-type').value = p.type;
  document.getElementById('form-key').value = p.api_key;
  document.getElementById('form-url').value = p.base_url;
  document.getElementById('form-models').value = (p.models || []).join(', ');
  document.getElementById('form-default-model').value = p.default_model || '';
}

function showList() {
  document.getElementById('view-list').style.display = 'block';
  document.getElementById('view-form').style.display = 'none';
  document.getElementById('test-result').innerHTML = '';
}

async function saveProvider(e) {
  e.preventDefault();
  const editing = document.getElementById('form-editing').value;
  const body = {
    name: document.getElementById('form-name').value,
    type: document.getElementById('form-type').value,
    api_key: document.getElementById('form-key').value,
    base_url: document.getElementById('form-url').value,
    models: document.getElementById('form-models').value.split(',').map(s => s.trim()).filter(Boolean),
    default_model: document.getElementById('form-default-model').value,
  };

  let resp;
  if (editing) {
    resp = await fetch(API + '/providers/' + encodeURIComponent(editing), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  } else {
    resp = await fetch(API + '/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }

  if (resp.ok) {
    await loadProviders();
    showList();
    render();
  } else {
    const data = await resp.json();
    alert('保存失败: ' + (data.error || 'unknown'));
  }
}

async function testConnection() {
  const body = {
    api_key: document.getElementById('form-key').value,
    base_url: document.getElementById('form-url').value,
  };
  const resp = await fetch(API + '/test-connection', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await resp.json();
  const div = document.getElementById('test-result');
  if (data.ok) {
    div.innerHTML = '&#10003; 连接成功';
    div.className = 'success';
  } else {
    div.innerHTML = '&#10007; 连接失败: ' + (data.error || '');
    div.className = 'error';
  }
}

function applyPreset() {
  const name = document.getElementById('form-preset').value;
  const p = presets.find(p => p.name === name);
  if (!p) return;
  document.getElementById('form-type').value = p.type;
  document.getElementById('form-url').value = p.base_url;
  document.getElementById('form-models').value = (p.models || []).join(', ');
  document.getElementById('form-default-model').value = p.default || '';
}

function toggleKeyVisibility() {
  const inp = document.getElementById('form-key');
  inp.type = inp.type === 'password' ? 'text' : 'password';
}

function resetForm() {
  document.getElementById('form-name').value = '';
  document.getElementById('form-type').value = 'openai_compatible';
  document.getElementById('form-key').value = '';
  document.getElementById('form-url').value = '';
  document.getElementById('form-models').value = '';
  document.getElementById('form-default-model').value = '';
  document.getElementById('form-preset').value = '';
  document.getElementById('test-result').innerHTML = '';
}

// ---- Utilities ----
function esc(s) { return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }

function maskKey(key) {
  if (!key || key.length < 7) return '****';
  return key.substring(0, 3) + '****' + key.substring(key.length - 4);
}

// ---- Boot ----
init();
