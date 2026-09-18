const $ = (id) => document.getElementById(id);

const titles = {
  status: ['运行状态', '同时管理多个独立 frpc 连接实例'],
  connections: ['连接配置', '用表单管理 frps、代理映射和 Visitor'],
  config: ['原始配置', '直接编辑当前 frpc 配置文件'],
  settings: ['设置', '管理 frpc 路径、多个配置文件和启动行为'],
  about: ['关于', 'FRP Client Manager'],
};

let lastState = null;
let configLoadedForPath = '';
let visualLoadedPath = '';
let visualConfig = null;
let visualCatalog = null;
let selectedProxyIndex = -1;
let selectedVisitorIndex = -1;

function backend() {
  const api = window.go?.main?.App;
  if (!api) throw new Error('Wails 后端尚未就绪');
  return api;
}

async function call(name, ...args) {
  const fn = backend()[name];
  if (typeof fn !== 'function') throw new Error('后端方法不可用: ' + name);
  return fn(...args);
}

function showMessage(text, type = '') {
  const el = $('message');
  el.textContent = text;
  el.className = ('message ' + type).trim();
  clearTimeout(showMessage.timer);
  showMessage.timer = setTimeout(() => el.classList.add('hidden'), 5000);
}

function humanTime(value) {
  if (!value) return '—';
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
}

function intValue(id, fallback = 0) {
  const value = Number.parseInt($(id).value, 10);
  return Number.isFinite(value) ? value : fallback;
}

function textValue(id) {
  return ($(id).value || '').trim();
}

function setValue(id, value) {
  $(id).value = value ?? '';
}

function setChecked(id, value) {
  $(id).checked = Boolean(value);
}

function splitList(value) {
  return String(value || '')
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinList(value) {
  return Array.isArray(value) ? value.join(', ') : '';
}

function mapToLines(value) {
  if (!value || typeof value !== 'object') return '';
  return Object.keys(value).sort().map((key) => key + '=' + value[key]).join('\n');
}

function linesToMap(value) {
  const result = {};
  String(value || '').split(/\r?\n/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    const index = trimmed.indexOf('=');
    if (index <= 0) return;
    result[trimmed.slice(0, index).trim()] = trimmed.slice(index + 1).trim();
  });
  return result;
}

function boolMapToLines(value) {
  if (!value || typeof value !== 'object') return '';
  return Object.keys(value).sort().map((key) => key + '=' + (value[key] ? 'true' : 'false')).join('\n');
}

function linesToBoolMap(value) {
  const result = {};
  String(value || '').split(/\r?\n/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    const index = trimmed.indexOf('=');
    if (index <= 0) return;
    const raw = trimmed.slice(index + 1).trim().toLowerCase();
    result[trimmed.slice(0, index).trim()] = ['1', 'true', 'yes', 'on'].includes(raw);
  });
  return result;
}

function headersToLines(headers) {
  if (!Array.isArray(headers)) return '';
  return headers.map((item) => item.name + '=' + item.value).join('\n');
}

function linesToHeaders(value) {
  return String(value || '').split(/\r?\n/).map((line) => {
    const index = line.indexOf('=');
    if (index <= 0) return null;
    return {name: line.slice(0, index).trim(), value: line.slice(index + 1).trim()};
  }).filter((item) => item && item.name);
}

function fillSelect(id, values, emptyLabel = null) {
  const select = $(id);
  const current = select.value;
  select.textContent = '';
  if (emptyLabel !== null) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = emptyLabel;
    select.appendChild(option);
  }
  (values || []).forEach((value) => {
    const option = document.createElement('option');
    option.value = value;
    option.textContent = value;
    select.appendChild(option);
  });
  if ([...select.options].some((option) => option.value === current)) {
    select.value = current;
  }
}

function ensureProxy(proxy) {
  proxy.transport ||= {};
  proxy.loadBalancer ||= {};
  proxy.healthCheck ||= {};
  proxy.requestHeaders ||= {};
  proxy.requestHeaders.set ||= {};
  proxy.responseHeaders ||= {};
  proxy.responseHeaders.set ||= {};
  proxy.metadatas ||= {};
  proxy.annotations ||= {};
  proxy.natTraversal ||= {};
  proxy.plugin ||= {};
  proxy.plugin.requestHeaders ||= {};
  proxy.plugin.requestHeaders.set ||= {};
  if (!proxy.localIP && !proxy.plugin.type) proxy.localIP = '127.0.0.1';
  if (!proxy.transport.bandwidthLimitMode) proxy.transport.bandwidthLimitMode = 'client';
  return proxy;
}

function ensureVisitor(visitor) {
  visitor.transport ||= {};
  visitor.natTraversal ||= {};
  visitor.plugin ||= {};
  if (!visitor.bindAddr) visitor.bindAddr = '127.0.0.1';
  return visitor;
}

function ensureVisualConfig(config) {
  const cfg = config || {};
  cfg.auth ||= {};
  cfg.auth.tokenSource ||= {};
  cfg.auth.tokenSource.file ||= {};
  cfg.auth.oidc ||= {};
  cfg.auth.oidc.additionalEndpointParams ||= {};
  cfg.log ||= {};
  cfg.webServer ||= {};
  cfg.transport ||= {};
  cfg.transport.quic ||= {};
  cfg.transport.tls ||= {};
  cfg.virtualNet ||= {};
  cfg.metadatas ||= {};
  cfg.featureGates ||= {};
  cfg.includes = Array.isArray(cfg.includes) ? cfg.includes : [];
  cfg.proxies = Array.isArray(cfg.proxies) ? cfg.proxies.map(ensureProxy) : [];
  cfg.visitors = Array.isArray(cfg.visitors) ? cfg.visitors.map(ensureVisitor) : [];
  return cfg;
}

function activeProfileState(state = lastState) {
  const id = state?.settings?.active_profile_id;
  return (state?.profiles || []).find((item) => item.profile?.id === id) || state?.profiles?.[0] || null;
}

function renderProfileSelect(select, state) {
  if (!select) return;
  const current = state.settings?.active_profile_id || '';
  select.textContent = '';
  (state.profiles || []).forEach((item) => {
    const option = document.createElement('option');
    option.value = item.profile.id;
    option.textContent = item.profile.name + (item.process?.running ? ' ●' : '');
    select.appendChild(option);
  });
  select.value = current;
}

function makeButton(text, className, action, disabled = false) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'button ' + (className || '');
  button.textContent = text;
  button.disabled = disabled;
  button.addEventListener('click', action);
  return button;
}

function renderProfileStatusList(state) {
  const container = $('profileStatusList');
  container.textContent = '';
  (state.profiles || []).forEach((item) => {
    const profile = item.profile;
    const process = item.process || {};
    const active = profile.id === state.settings?.active_profile_id;

    const row = document.createElement('div');
    row.className = 'profile-status-item';

    const main = document.createElement('div');
    main.className = 'profile-status-main';
    const title = document.createElement('div');
    title.className = 'profile-title-row';
    const dot = document.createElement('span');
    dot.className = 'profile-running-dot' + (process.running ? ' running' : '');
    const name = document.createElement('strong');
    name.textContent = profile.name;
    title.append(dot, name);
    if (active) {
      const badge = document.createElement('span');
      badge.className = 'profile-active-badge';
      badge.textContent = '当前配置';
      title.appendChild(badge);
    }
    if (profile.auto_start) {
      const badge = document.createElement('span');
      badge.className = 'state-pill';
      badge.textContent = '自动启动';
      title.appendChild(badge);
    }

    const meta = document.createElement('div');
    meta.className = 'profile-meta';
    const pid = process.running ? 'PID ' + process.pid : '已停止';
    meta.textContent = pid + ' · ' + profile.config_path;
    main.append(title, meta);

    const actions = document.createElement('div');
    actions.className = 'profile-actions';
    if (!active) {
      actions.appendChild(makeButton('设为当前', 'ghost', () => switchProfile(profile.id)));
    }
    actions.appendChild(makeButton('启动', 'primary', () => withAction(profile.name + ' 已启动', () => call('StartProfile', profile.id)), Boolean(process.running)));
    actions.appendChild(makeButton('停止', 'danger', () => withAction(profile.name + ' 已停止', () => call('StopProfile', profile.id)), !process.running));
    actions.appendChild(makeButton('重启', '', () => withAction(profile.name + ' 已重启', () => call('RestartProfile', profile.id)), !process.running));

    row.append(main, actions);
    container.appendChild(row);
  });
}

function renderProfileSettingsList(state) {
  const container = $('profileSettingsList');
  container.textContent = '';
  (state.profiles || []).forEach((item) => {
    const profile = item.profile;
    const active = profile.id === state.settings?.active_profile_id;
    const row = document.createElement('div');
    row.className = 'profile-settings-item';

    const main = document.createElement('div');
    main.className = 'profile-settings-main';
    const title = document.createElement('div');
    title.className = 'profile-title-row';
    const name = document.createElement('strong');
    name.textContent = profile.name;
    title.appendChild(name);
    if (active) {
      const badge = document.createElement('span');
      badge.className = 'profile-active-badge';
      badge.textContent = '当前';
      title.appendChild(badge);
    }
    if (profile.auto_start) {
      const badge = document.createElement('span');
      badge.className = 'state-pill';
      badge.textContent = '自动启动';
      title.appendChild(badge);
    }
    const meta = document.createElement('div');
    meta.className = 'profile-meta';
    meta.textContent = profile.config_path;
    main.append(title, meta);

    const actions = document.createElement('div');
    actions.className = 'profile-actions';
    actions.appendChild(makeButton('编辑', '', () => openProfileDialog(profile)));
    actions.appendChild(makeButton('删除', 'danger', () => deleteProfile(profile), (state.profiles || []).length <= 1 || Boolean(item.process?.running)));
    row.append(main, actions);
    container.appendChild(row);
  });
}

function render(state) {
  lastState = state;
  const active = activeProfileState(state);
  const process = active?.process || {};
  const running = Boolean(process.running);
  const pid = running ? process.pid : '—';
  const detached = Boolean(process.detached);
  const total = (state.profiles || []).length;
  const runningCount = Number(state.running_count || 0);

  $('sideStatusDot').className = 'status-dot ' + (runningCount > 0 ? 'running' : 'stopped');
  $('sideStatusText').textContent = runningCount + '/' + total + ' 个连接运行中';
  $('sidePid').textContent = active ? '当前：' + active.profile.name : '无连接配置';

  $('heroDot').className = 'hero-dot ' + (runningCount > 0 ? 'running' : '');
  $('heroState').textContent = runningCount + ' / ' + total + ' 运行中';
  $('heroDetail').textContent = active
    ? ('当前连接：' + active.profile.name + (running ? (detached ? ' · 已恢复后台进程' : ' · 当前会话托管') : ' · 已停止'))
    : '尚未配置连接';

  $('pidValue').textContent = pid;
  $('startedAtValue').textContent = running ? humanTime(process.started_at) : '—';
  $('managedValue').textContent = active ? active.profile.name : '—';
  $('autostartValue').textContent = state.launch_at_login ? '管理器已自启' : '管理器未自启';

  $('startButton').disabled = total === 0 || runningCount === total;
  $('stopButton').disabled = runningCount === 0;
  $('restartButton').disabled = runningCount === 0;

  const logs = Array.isArray(process.log_tail) ? process.log_tail : [];
  $('logOutput').textContent = logs.length ? logs.join('\n') : '暂无日志';

  if (document.activeElement?.id !== 'frpcPath') {
    $('frpcPath').value = state.settings?.frpc_path || '';
  }
  $('launchAtLogin').checked = Boolean(state.launch_at_login);
  $('launchAtLogin').disabled = !state.launch_at_login_supported;
  $('dataDir').textContent = state.data_dir || '—';

  const activePath = active?.profile?.config_path || '';
  $('configPathHint').textContent = activePath || '尚未设置配置路径';
  $('visualConfigPath').textContent = activePath || '尚未设置配置路径';
  renderProfileSelect($('visualProfileSelect'), state);
  renderProfileSelect($('rawProfileSelect'), state);
  renderProfileStatusList(state);
  renderProfileSettingsList(state);

  if (state.startup_error) {
    showMessage('自动启动 frpc 失败：' + state.startup_error, 'error');
  }
}

async function refresh(silent = false) {
  try {
    render(await call('GetState'));
  } catch (err) {
    if (!silent) showMessage(String(err), 'error');
  }
}

async function withAction(label, action) {
  try {
    const state = await action();
    if (state) render(state);
    showMessage(label, 'success');
  } catch (err) {
    showMessage(String(err), 'error');
  }
}

function openPage(page) {
  document.querySelectorAll('.nav-item').forEach((item) => {
    item.classList.toggle('active', item.dataset.page === page);
  });
  document.querySelectorAll('[data-page-panel]').forEach((panel) => {
    panel.classList.toggle('active', panel.dataset.pagePanel === page);
  });
  $('pageTitle').textContent = titles[page][0];
  $('pageSubtitle').textContent = titles[page][1];
  if (page === 'config') loadConfig();
  if (page === 'connections') loadVisualConfig();
}

async function switchProfile(id) {
  if (!id || id === lastState?.settings?.active_profile_id) return;
  try {
    const state = await call('SetActiveProfile', id);
    configLoadedForPath = '';
    visualLoadedPath = '';
    visualConfig = null;
    selectedProxyIndex = -1;
    selectedVisitorIndex = -1;
    render(state);
    const activePage = document.querySelector('.nav-item.active')?.dataset.page;
    if (activePage === 'config') await loadConfig(true);
    if (activePage === 'connections') await loadVisualConfig(true);
  } catch (err) {
    showMessage(String(err), 'error');
  }
}

function openProfileDialog(profile = null) {
  $('profileDialogTitle').textContent = profile ? '编辑连接' : '添加连接';
  $('profileID').value = profile?.id || '';
  $('profileName').value = profile?.name || '';
  $('profileConfigPath').value = profile?.config_path || '';
  $('profileAutoStart').checked = Boolean(profile?.auto_start);
  if (typeof $('profileDialog').showModal === 'function') {
    $('profileDialog').showModal();
  } else {
    $('profileDialog').setAttribute('open', '');
  }
}

async function deleteProfile(profile) {
  const stateItem = (lastState?.profiles || []).find((item) => item.profile?.id === profile.id);
  if (stateItem?.process?.running) {
    showMessage('请先停止连接后再删除', 'error');
    return;
  }
  try {
    const state = await call('DeleteProfile', profile.id);
    configLoadedForPath = '';
    visualLoadedPath = '';
    visualConfig = null;
    render(state);
    showMessage('连接已删除', 'success');
  } catch (err) {
    showMessage(String(err), 'error');
  }
}

async function loadConfig(force = false) {
  try {
    const state = lastState || await call('GetState');
    const path = activeProfileState(state)?.profile?.config_path || '';
    if (!force && configLoadedForPath === path) return;
    $('configEditor').value = await call('ReadConfig');
    configLoadedForPath = path;
  } catch (err) {
    showMessage(String(err), 'error');
  }
}

function renderServerForm() {
  const c = visualConfig;
  setValue('vcServerAddr', c.serverAddr);
  setValue('vcServerPort', c.serverPort);
  setValue('vcUser', c.user);
  setValue('vcClientID', c.clientID);
  setValue('vcAuthMethod', c.auth.method || 'token');
  setValue('vcAuthToken', c.auth.token);
  setValue('vcProtocol', c.transport.protocol || 'tcp');
  setValue('vcWireProtocol', c.transport.wireProtocol || '');
  setChecked('vcTLSEnable', c.transport.tls.enable !== false);
  setChecked('vcTCPMux', c.transport.tcpMux !== false);
  setChecked('vcLoginFailExit', c.loginFailExit !== false);

  setValue('vcOIDCClientID', c.auth.oidc.clientID);
  setValue('vcOIDCClientSecret', c.auth.oidc.clientSecret);
  setValue('vcOIDCAudience', c.auth.oidc.audience);
  setValue('vcOIDCScope', c.auth.oidc.scope);
  setValue('vcOIDCTokenEndpoint', c.auth.oidc.tokenEndpointURL);
  setValue('vcOIDCCA', c.auth.oidc.trustedCaFile);
  setValue('vcOIDCProxy', c.auth.oidc.proxyURL);
  setChecked('vcOIDCSkipVerify', c.auth.oidc.insecureSkipVerify);

  setValue('vcPoolCount', c.transport.poolCount);
  setValue('vcDialTimeout', c.transport.dialServerTimeout);
  setValue('vcDialKeepalive', c.transport.dialServerKeepalive);
  setValue('vcMuxKeepalive', c.transport.tcpMuxKeepaliveInterval);
  setValue('vcConnectLocalIP', c.transport.connectServerLocalIP);
  setValue('vcProxyURL', c.transport.proxyURL);
  setValue('vcHeartbeatInterval', c.transport.heartbeatInterval);
  setValue('vcHeartbeatTimeout', c.transport.heartbeatTimeout);
  setValue('vcTLSCert', c.transport.tls.certFile);
  setValue('vcTLSKey', c.transport.tls.keyFile);
  setValue('vcTLSCA', c.transport.tls.trustedCaFile);
  setValue('vcTLSServerName', c.transport.tls.serverName);
  setChecked('vcDisableTLSFirstByte', c.transport.tls.disableCustomTLSFirstByte !== false);
  setValue('vcQUICKeepalive', c.transport.quic.keepalivePeriod);
  setValue('vcQUICIdle', c.transport.quic.maxIdleTimeout);
  setValue('vcQUICStreams', c.transport.quic.maxIncomingStreams);

  setValue('vcWebAddr', c.webServer.addr);
  setValue('vcWebPort', c.webServer.port);
  setValue('vcWebUser', c.webServer.user);
  setValue('vcWebPassword', c.webServer.password);
  setChecked('vcPprof', c.webServer.pprofEnable);
  setValue('vcLogTo', c.log.to);
  setValue('vcLogLevel', c.log.level || 'info');
  setValue('vcLogMaxDays', c.log.maxDays);
  setValue('vcDNSServer', c.dnsServer);
  setValue('vcStunServer', c.natHoleStunServer);
  setValue('vcUDPPacketSize', c.udpPacketSize || 1500);
  setValue('vcIncludes', (c.includes || []).join('\n'));
  setValue('vcMetadata', mapToLines(c.metadatas));
  setValue('vcFeatureGates', boolMapToLines(c.featureGates));
  setValue('vcVirtualNetAddress', c.virtualNet.address);
  updateAuthVisibility();
}

function syncServerForm() {
  const c = visualConfig;
  c.serverAddr = textValue('vcServerAddr');
  c.serverPort = intValue('vcServerPort', 7000);
  c.user = textValue('vcUser');
  c.clientID = textValue('vcClientID');
  c.auth.method = textValue('vcAuthMethod') || 'token';
  c.auth.token = textValue('vcAuthToken');
  c.transport.protocol = textValue('vcProtocol') || 'tcp';
  c.transport.wireProtocol = textValue('vcWireProtocol');
  c.transport.tls.enable = $('vcTLSEnable').checked;
  c.transport.tcpMux = $('vcTCPMux').checked;
  c.loginFailExit = $('vcLoginFailExit').checked;

  c.auth.oidc.clientID = textValue('vcOIDCClientID');
  c.auth.oidc.clientSecret = textValue('vcOIDCClientSecret');
  c.auth.oidc.audience = textValue('vcOIDCAudience');
  c.auth.oidc.scope = textValue('vcOIDCScope');
  c.auth.oidc.tokenEndpointURL = textValue('vcOIDCTokenEndpoint');
  c.auth.oidc.trustedCaFile = textValue('vcOIDCCA');
  c.auth.oidc.proxyURL = textValue('vcOIDCProxy');
  c.auth.oidc.insecureSkipVerify = $('vcOIDCSkipVerify').checked;

  c.transport.poolCount = intValue('vcPoolCount');
  c.transport.dialServerTimeout = intValue('vcDialTimeout');
  c.transport.dialServerKeepalive = intValue('vcDialKeepalive');
  c.transport.tcpMuxKeepaliveInterval = intValue('vcMuxKeepalive');
  c.transport.connectServerLocalIP = textValue('vcConnectLocalIP');
  c.transport.proxyURL = textValue('vcProxyURL');
  c.transport.heartbeatInterval = intValue('vcHeartbeatInterval');
  c.transport.heartbeatTimeout = intValue('vcHeartbeatTimeout');
  c.transport.tls.certFile = textValue('vcTLSCert');
  c.transport.tls.keyFile = textValue('vcTLSKey');
  c.transport.tls.trustedCaFile = textValue('vcTLSCA');
  c.transport.tls.serverName = textValue('vcTLSServerName');
  c.transport.tls.disableCustomTLSFirstByte = $('vcDisableTLSFirstByte').checked;
  c.transport.quic.keepalivePeriod = intValue('vcQUICKeepalive');
  c.transport.quic.maxIdleTimeout = intValue('vcQUICIdle');
  c.transport.quic.maxIncomingStreams = intValue('vcQUICStreams');

  c.webServer.addr = textValue('vcWebAddr');
  c.webServer.port = intValue('vcWebPort');
  c.webServer.user = textValue('vcWebUser');
  c.webServer.password = textValue('vcWebPassword');
  c.webServer.pprofEnable = $('vcPprof').checked;
  c.log.to = textValue('vcLogTo');
  c.log.level = textValue('vcLogLevel') || 'info';
  c.log.maxDays = intValue('vcLogMaxDays');
  c.dnsServer = textValue('vcDNSServer');
  c.natHoleStunServer = textValue('vcStunServer');
  c.udpPacketSize = intValue('vcUDPPacketSize', 1500);
  c.includes = splitList($('vcIncludes').value);
  c.metadatas = linesToMap($('vcMetadata').value);
  c.featureGates = linesToBoolMap($('vcFeatureGates').value);
  c.virtualNet.address = textValue('vcVirtualNetAddress');
}

function updateAuthVisibility() {
  const oidc = $('vcAuthMethod').value === 'oidc';
  $('oidcCard').classList.toggle('hidden', !oidc);
  $('vcTokenField').classList.toggle('hidden', oidc);
}

function proxySummary(proxy) {
  const local = proxy.plugin?.type
    ? 'Plugin ' + proxy.plugin.type
    : (proxy.localIP || '127.0.0.1') + ':' + (proxy.localPort || '—');
  if (['tcp', 'udp'].includes(proxy.type)) {
    return local + ' → :' + (proxy.remotePort === 0 ? '随机' : (proxy.remotePort || '—'));
  }
  if (['http', 'https', 'tcpmux'].includes(proxy.type)) {
    const target = (proxy.customDomains || [])[0] || proxy.subdomain || '未设置域名';
    return local + ' → ' + target;
  }
  return local;
}

function renderProxyList() {
  const list = $('proxyList');
  list.textContent = '';
  $('proxyCountBadge').textContent = String(visualConfig.proxies.length);
  if (!visualConfig.proxies.length) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.textContent = '还没有代理映射';
    list.appendChild(empty);
    return;
  }
  visualConfig.proxies.forEach((proxy, index) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'config-item' + (index === selectedProxyIndex ? ' active' : '');
    button.dataset.proxyIndex = String(index);

    const top = document.createElement('div');
    top.className = 'config-item-top';
    const name = document.createElement('strong');
    name.textContent = proxy.name || '未命名代理';
    const state = document.createElement('span');
    state.className = 'state-pill' + (proxy.enabled === false ? ' off' : '');
    state.textContent = proxy.enabled === false ? '已停用' : '启用';
    top.append(name, state);

    const meta = document.createElement('div');
    meta.className = 'config-item-meta';
    const type = document.createElement('span');
    type.className = 'type-badge';
    type.textContent = proxy.type || 'tcp';
    const summary = document.createElement('span');
    summary.textContent = proxySummary(proxy);
    meta.append(type, summary);

    button.append(top, meta);
    button.addEventListener('click', () => selectProxy(index));
    list.appendChild(button);
  });
}

function renderProxyEditor() {
  const proxy = visualConfig.proxies[selectedProxyIndex];
  $('proxyEmpty').classList.toggle('hidden', Boolean(proxy));
  $('proxyEditor').classList.toggle('hidden', !proxy);
  if (!proxy) return;

  ensureProxy(proxy);
  $('proxyEditorTitle').textContent = proxy.name || '未命名代理';
  $('proxyEditorSummary').textContent = proxySummary(proxy);
  setValue('pxName', proxy.name);
  setValue('pxType', proxy.type || 'tcp');
  setChecked('pxEnabled', proxy.enabled !== false);
  setValue('pxLocalIP', proxy.localIP);
  setValue('pxLocalPort', proxy.localPort);
  setValue('pxRemotePort', proxy.remotePort);
  setValue('pxDomains', joinList(proxy.customDomains));
  setValue('pxSubdomain', proxy.subdomain);
  setValue('pxLocations', joinList(proxy.locations));
  setValue('pxHTTPUser', proxy.httpUser);
  setValue('pxHTTPPassword', proxy.httpPassword);
  setValue('pxRouteUser', proxy.routeByHTTPUser);
  setValue('pxHostRewrite', proxy.hostHeaderRewrite);
  setValue('pxMultiplexer', proxy.multiplexer || 'httpconnect');
  setValue('pxSecretKey', proxy.secretKey);
  setValue('pxAllowUsers', joinList(proxy.allowUsers));

  setChecked('pxEncryption', proxy.transport.useEncryption);
  setChecked('pxCompression', proxy.transport.useCompression);
  setValue('pxBandwidth', proxy.transport.bandwidthLimit);
  setValue('pxBandwidthMode', proxy.transport.bandwidthLimitMode || 'client');
  setValue('pxProxyProtocol', proxy.transport.proxyProtocolVersion);
  setValue('pxGroup', proxy.loadBalancer.group);
  setValue('pxGroupKey', proxy.loadBalancer.groupKey);

  setValue('pxHealthType', proxy.healthCheck.type);
  setValue('pxHealthTimeout', proxy.healthCheck.timeoutSeconds);
  setValue('pxHealthMaxFailed', proxy.healthCheck.maxFailed);
  setValue('pxHealthInterval', proxy.healthCheck.intervalSeconds);
  setValue('pxHealthPath', proxy.healthCheck.path);
  setValue('pxHealthHeaders', headersToLines(proxy.healthCheck.httpHeaders));

  setValue('pxRequestHeaders', mapToLines(proxy.requestHeaders.set));
  setValue('pxResponseHeaders', mapToLines(proxy.responseHeaders.set));
  setValue('pxMetadata', mapToLines(proxy.metadatas));
  setValue('pxAnnotations', mapToLines(proxy.annotations));
  setChecked('pxDisableAssisted', proxy.natTraversal.disableAssistedAddrs);

  setValue('pxPluginType', proxy.plugin.type);
  setValue('pxPluginUnixPath', proxy.plugin.unixPath);
  setValue('pxPluginLocalAddr', proxy.plugin.localAddr);
  setValue('pxPluginLocalPath', proxy.plugin.localPath);
  setValue('pxPluginStripPrefix', proxy.plugin.stripPrefix);
  setValue('pxPluginUsername', proxy.plugin.username);
  setValue('pxPluginPassword', proxy.plugin.password);
  setValue('pxPluginHTTPUser', proxy.plugin.httpUser);
  setValue('pxPluginHTTPPassword', proxy.plugin.httpPassword);
  setValue('pxPluginCert', proxy.plugin.crtPath);
  setValue('pxPluginKey', proxy.plugin.keyPath);
  setValue('pxPluginHostRewrite', proxy.plugin.hostHeaderRewrite);
  setValue('pxPluginRequestHeaders', mapToLines(proxy.plugin.requestHeaders?.set));

  updateProxyVisibility();
}

function clearPluginByType(plugin, type) {
  const fresh = {type, requestHeaders: {set: {}}};
  if (type === 'unix_domain_socket') fresh.unixPath = textValue('pxPluginUnixPath');
  if (type === 'http_proxy') {
    fresh.httpUser = textValue('pxPluginHTTPUser');
    fresh.httpPassword = textValue('pxPluginHTTPPassword');
  }
  if (type === 'socks5') {
    fresh.username = textValue('pxPluginUsername');
    fresh.password = textValue('pxPluginPassword');
  }
  if (type === 'static_file') {
    fresh.localPath = textValue('pxPluginLocalPath');
    fresh.stripPrefix = textValue('pxPluginStripPrefix');
    fresh.httpUser = textValue('pxPluginHTTPUser');
    fresh.httpPassword = textValue('pxPluginHTTPPassword');
  }
  if (['https2http', 'https2https'].includes(type)) {
    fresh.localAddr = textValue('pxPluginLocalAddr');
    fresh.crtPath = textValue('pxPluginCert');
    fresh.keyPath = textValue('pxPluginKey');
    fresh.hostHeaderRewrite = textValue('pxPluginHostRewrite');
    fresh.requestHeaders = {set: linesToMap($('pxPluginRequestHeaders').value)};
  }
  if (['http2https', 'http2http'].includes(type)) {
    fresh.localAddr = textValue('pxPluginLocalAddr');
    fresh.hostHeaderRewrite = textValue('pxPluginHostRewrite');
    fresh.requestHeaders = {set: linesToMap($('pxPluginRequestHeaders').value)};
  }
  if (type === 'tls2raw') {
    fresh.localAddr = textValue('pxPluginLocalAddr');
    fresh.crtPath = textValue('pxPluginCert');
    fresh.keyPath = textValue('pxPluginKey');
  }
  Object.keys(plugin).forEach((key) => delete plugin[key]);
  Object.assign(plugin, fresh);
}

function syncSelectedProxyForm() {
  const proxy = visualConfig?.proxies?.[selectedProxyIndex];
  if (!proxy || $('proxyEditor').classList.contains('hidden')) return;

  proxy.name = textValue('pxName');
  proxy.type = textValue('pxType') || 'tcp';
  proxy.enabled = $('pxEnabled').checked;
  proxy.localIP = textValue('pxLocalIP');
  proxy.localPort = intValue('pxLocalPort');

  if (['tcp', 'udp'].includes(proxy.type)) {
    proxy.remotePort = intValue('pxRemotePort');
  } else {
    proxy.remotePort = 0;
  }

  if (['http', 'https', 'tcpmux'].includes(proxy.type)) {
    proxy.customDomains = splitList($('pxDomains').value);
    proxy.subdomain = textValue('pxSubdomain');
  } else {
    proxy.customDomains = [];
    proxy.subdomain = '';
  }

  if (proxy.type === 'http') {
    proxy.locations = splitList($('pxLocations').value);
    proxy.hostHeaderRewrite = textValue('pxHostRewrite');
    proxy.requestHeaders.set = linesToMap($('pxRequestHeaders').value);
    proxy.responseHeaders.set = linesToMap($('pxResponseHeaders').value);
  } else {
    proxy.locations = [];
    proxy.hostHeaderRewrite = '';
    proxy.requestHeaders.set = {};
    proxy.responseHeaders.set = {};
  }

  if (['http', 'tcpmux'].includes(proxy.type)) {
    proxy.httpUser = textValue('pxHTTPUser');
    proxy.httpPassword = textValue('pxHTTPPassword');
    proxy.routeByHTTPUser = textValue('pxRouteUser');
  } else {
    proxy.httpUser = '';
    proxy.httpPassword = '';
    proxy.routeByHTTPUser = '';
  }

  proxy.multiplexer = proxy.type === 'tcpmux' ? (textValue('pxMultiplexer') || 'httpconnect') : '';

  if (['stcp', 'sudp', 'xtcp'].includes(proxy.type)) {
    proxy.secretKey = textValue('pxSecretKey');
    proxy.allowUsers = splitList($('pxAllowUsers').value);
  } else {
    proxy.secretKey = '';
    proxy.allowUsers = [];
  }

  proxy.natTraversal.disableAssistedAddrs = proxy.type === 'xtcp' && $('pxDisableAssisted').checked;

  proxy.transport.useEncryption = $('pxEncryption').checked;
  proxy.transport.useCompression = $('pxCompression').checked;
  proxy.transport.bandwidthLimit = textValue('pxBandwidth');
  proxy.transport.bandwidthLimitMode = textValue('pxBandwidthMode') || 'client';
  proxy.transport.proxyProtocolVersion = textValue('pxProxyProtocol');
  proxy.loadBalancer.group = textValue('pxGroup');
  proxy.loadBalancer.groupKey = textValue('pxGroupKey');

  proxy.healthCheck.type = textValue('pxHealthType');
  proxy.healthCheck.timeoutSeconds = intValue('pxHealthTimeout');
  proxy.healthCheck.maxFailed = intValue('pxHealthMaxFailed');
  proxy.healthCheck.intervalSeconds = intValue('pxHealthInterval');
  if (proxy.healthCheck.type === 'http') {
    proxy.healthCheck.path = textValue('pxHealthPath');
    proxy.healthCheck.httpHeaders = linesToHeaders($('pxHealthHeaders').value);
  } else {
    proxy.healthCheck.path = '';
    proxy.healthCheck.httpHeaders = [];
  }

  proxy.metadatas = linesToMap($('pxMetadata').value);
  proxy.annotations = linesToMap($('pxAnnotations').value);

  const pluginType = textValue('pxPluginType');
  if (!pluginType) {
    proxy.plugin = {requestHeaders: {set: {}}};
  } else {
    clearPluginByType(proxy.plugin, pluginType);
  }
}

function updateProxyVisibility() {
  const type = $('pxType').value;
  const domain = ['http', 'https', 'tcpmux'].includes(type);
  const secret = ['stcp', 'sudp', 'xtcp'].includes(type);
  $('pxRemoteFields').classList.toggle('hidden', !['tcp', 'udp'].includes(type));
  $('pxDomainFields').classList.toggle('hidden', !domain);
  $('pxSecretFields').classList.toggle('hidden', !secret);
  $('pxMultiplexerField').classList.toggle('hidden', type !== 'tcpmux');
  $('pxLocationsField').classList.toggle('hidden', type !== 'http');
  $('pxHTTPUserField').classList.toggle('hidden', !['http', 'tcpmux'].includes(type));
  $('pxHTTPPasswordField').classList.toggle('hidden', !['http', 'tcpmux'].includes(type));
  $('pxRouteUserField').classList.toggle('hidden', !['http', 'tcpmux'].includes(type));
  $('pxHostRewriteField').classList.toggle('hidden', type !== 'http');
  $('pxPluginFields').classList.toggle('hidden', !$('pxPluginType').value);
  const httpHealth = $('pxHealthType').value === 'http';
  $('pxHealthPathField').classList.toggle('hidden', !httpHealth);
  $('pxHealthHeadersField').classList.toggle('hidden', !httpHealth);
}

function selectProxy(index) {
  syncSelectedProxyForm();
  selectedProxyIndex = index;
  renderProxyList();
  renderProxyEditor();
}

function addProxy() {
  syncSelectedProxyForm();
  let suffix = visualConfig.proxies.length + 1;
  const names = new Set(visualConfig.proxies.map((item) => item.name));
  while (names.has('tcp-' + suffix)) suffix += 1;
  visualConfig.proxies.push(ensureProxy({
    name: 'tcp-' + suffix,
    type: 'tcp',
    enabled: true,
    localIP: '127.0.0.1',
    localPort: 80,
    remotePort: 6000 + visualConfig.proxies.length,
  }));
  selectedProxyIndex = visualConfig.proxies.length - 1;
  renderProxyList();
  renderProxyEditor();
}

function deleteProxy() {
  if (selectedProxyIndex < 0) return;
  visualConfig.proxies.splice(selectedProxyIndex, 1);
  selectedProxyIndex = Math.min(selectedProxyIndex, visualConfig.proxies.length - 1);
  renderProxyList();
  renderProxyEditor();
}

function visitorSummary(visitor) {
  const target = (visitor.serverUser ? visitor.serverUser + '.' : '') + (visitor.serverName || '未设置服务');
  return (visitor.bindAddr || '127.0.0.1') + ':' + (visitor.bindPort ?? '—') + ' → ' + target;
}

function renderVisitorList() {
  const list = $('visitorList');
  list.textContent = '';
  $('visitorCountBadge').textContent = String(visualConfig.visitors.length);
  if (!visualConfig.visitors.length) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.textContent = '还没有 Visitor';
    list.appendChild(empty);
    return;
  }
  visualConfig.visitors.forEach((visitor, index) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'config-item' + (index === selectedVisitorIndex ? ' active' : '');
    const top = document.createElement('div');
    top.className = 'config-item-top';
    const name = document.createElement('strong');
    name.textContent = visitor.name || '未命名 Visitor';
    const state = document.createElement('span');
    state.className = 'state-pill' + (visitor.enabled === false ? ' off' : '');
    state.textContent = visitor.enabled === false ? '已停用' : '启用';
    top.append(name, state);
    const meta = document.createElement('div');
    meta.className = 'config-item-meta';
    const type = document.createElement('span');
    type.className = 'type-badge';
    type.textContent = visitor.type || 'stcp';
    const summary = document.createElement('span');
    summary.textContent = visitorSummary(visitor);
    meta.append(type, summary);
    button.append(top, meta);
    button.addEventListener('click', () => selectVisitor(index));
    list.appendChild(button);
  });
}

function renderVisitorEditor() {
  const visitor = visualConfig.visitors[selectedVisitorIndex];
  $('visitorEmpty').classList.toggle('hidden', Boolean(visitor));
  $('visitorEditor').classList.toggle('hidden', !visitor);
  if (!visitor) return;
  ensureVisitor(visitor);

  $('visitorEditorTitle').textContent = visitor.name || '未命名 Visitor';
  setValue('vsName', visitor.name);
  setValue('vsType', visitor.type || 'stcp');
  setChecked('vsEnabled', visitor.enabled !== false);
  setValue('vsServerUser', visitor.serverUser);
  setValue('vsServerName', visitor.serverName);
  setValue('vsSecretKey', visitor.secretKey);
  setValue('vsBindAddr', visitor.bindAddr);
  setValue('vsBindPort', visitor.bindPort);
  setChecked('vsEncryption', visitor.transport.useEncryption);
  setChecked('vsCompression', visitor.transport.useCompression);
  setChecked('vsKeepTunnel', visitor.keepTunnelOpen);
  setValue('vsMaxRetries', visitor.maxRetriesAnHour);
  setValue('vsMinRetry', visitor.minRetryInterval);
  setValue('vsFallbackTo', visitor.fallbackTo);
  setValue('vsFallbackTimeout', visitor.fallbackTimeoutMs);
  setChecked('vsDisableAssisted', visitor.natTraversal.disableAssistedAddrs);
  setValue('vsPluginType', visitor.plugin.type);
  setValue('vsDestinationIP', visitor.plugin.destinationIP);
  updateVisitorVisibility();
}

function syncSelectedVisitorForm() {
  const visitor = visualConfig?.visitors?.[selectedVisitorIndex];
  if (!visitor || $('visitorEditor').classList.contains('hidden')) return;
  visitor.name = textValue('vsName');
  visitor.type = textValue('vsType') || 'stcp';
  visitor.enabled = $('vsEnabled').checked;
  visitor.serverUser = textValue('vsServerUser');
  visitor.serverName = textValue('vsServerName');
  visitor.secretKey = textValue('vsSecretKey');
  visitor.bindAddr = textValue('vsBindAddr') || '127.0.0.1';
  visitor.bindPort = intValue('vsBindPort');
  visitor.transport.useEncryption = $('vsEncryption').checked;
  visitor.transport.useCompression = $('vsCompression').checked;

  if (visitor.type === 'xtcp') {
    visitor.keepTunnelOpen = $('vsKeepTunnel').checked;
    visitor.maxRetriesAnHour = intValue('vsMaxRetries');
    visitor.minRetryInterval = intValue('vsMinRetry');
    visitor.fallbackTo = textValue('vsFallbackTo');
    visitor.fallbackTimeoutMs = intValue('vsFallbackTimeout');
    visitor.natTraversal.disableAssistedAddrs = $('vsDisableAssisted').checked;
  } else {
    visitor.keepTunnelOpen = false;
    visitor.maxRetriesAnHour = 0;
    visitor.minRetryInterval = 0;
    visitor.fallbackTo = '';
    visitor.fallbackTimeoutMs = 0;
    visitor.natTraversal.disableAssistedAddrs = false;
  }

  const pluginType = textValue('vsPluginType');
  visitor.plugin = pluginType
    ? {type: pluginType, destinationIP: textValue('vsDestinationIP')}
    : {};
}

function updateVisitorVisibility() {
  $('visitorXTCPFields').classList.toggle('hidden', $('vsType').value !== 'xtcp');
}

function selectVisitor(index) {
  syncSelectedVisitorForm();
  selectedVisitorIndex = index;
  renderVisitorList();
  renderVisitorEditor();
}

function addVisitor() {
  syncSelectedVisitorForm();
  let suffix = visualConfig.visitors.length + 1;
  const names = new Set(visualConfig.visitors.map((item) => item.name));
  while (names.has('visitor-' + suffix)) suffix += 1;
  visualConfig.visitors.push(ensureVisitor({
    name: 'visitor-' + suffix,
    type: 'stcp',
    enabled: true,
    serverName: '',
    secretKey: '',
    bindAddr: '127.0.0.1',
    bindPort: 9000 + visualConfig.visitors.length,
  }));
  selectedVisitorIndex = visualConfig.visitors.length - 1;
  renderVisitorList();
  renderVisitorEditor();
}

function deleteVisitor() {
  if (selectedVisitorIndex < 0) return;
  visualConfig.visitors.splice(selectedVisitorIndex, 1);
  selectedVisitorIndex = Math.min(selectedVisitorIndex, visualConfig.visitors.length - 1);
  renderVisitorList();
  renderVisitorEditor();
}

function syncVisualForms() {
  if (!visualConfig) return;
  syncServerForm();
  syncSelectedProxyForm();
  syncSelectedVisitorForm();
}

async function loadVisualConfig(force = false) {
  const path = activeProfileState(lastState)?.profile?.config_path || '';
  if (!force && visualConfig && visualLoadedPath === path) return;
  try {
    const [catalog, config] = await Promise.all([
      call('GetVisualConfigCatalog'),
      call('GetVisualConfig'),
    ]);
    visualCatalog = catalog;
    visualConfig = ensureVisualConfig(config);
    visualLoadedPath = path;
    fillSelect('pxType', visualCatalog.proxy_types || []);
    fillSelect('pxPluginType', visualCatalog.plugin_types || [], '不使用 Plugin');
    fillSelect('vsType', visualCatalog.visitor_types || []);
    selectedProxyIndex = visualConfig.proxies.length ? 0 : -1;
    selectedVisitorIndex = visualConfig.visitors.length ? 0 : -1;
    $('visualUnavailable').classList.add('hidden');
    $('visualWorkspace').classList.remove('hidden');
    renderServerForm();
    renderProxyList();
    renderProxyEditor();
    renderVisitorList();
    renderVisitorEditor();
  } catch (err) {
    visualConfig = null;
    visualLoadedPath = '';
    $('visualWorkspace').classList.add('hidden');
    $('visualUnavailable').classList.remove('hidden');
    $('visualUnavailableReason').textContent = String(err);
  }
}

async function previewVisualConfig() {
  if (!visualConfig) return;
  try {
    syncVisualForms();
    const content = await call('PreviewVisualConfig', visualConfig);
    $('visualPreviewOutput').textContent = content;
    if (typeof $('visualPreviewDialog').showModal === 'function') {
      $('visualPreviewDialog').showModal();
    } else {
      $('visualPreviewDialog').setAttribute('open', '');
    }
  } catch (err) {
    showMessage(String(err), 'error');
  }
}

async function saveVisualConfig(restart) {
  if (!visualConfig) return;
  try {
    syncVisualForms();
    await call('PreviewVisualConfig', visualConfig);
    const state = await call('SaveVisualConfig', visualConfig);
    render(state);
    configLoadedForPath = '';
    visualLoadedPath = activeProfileState(state)?.profile?.config_path || visualLoadedPath;
    if (restart) {
      const next = await call('RestartFRPC');
      render(next);
      showMessage('配置已保存，frpc 已重启', 'success');
    } else {
      showMessage('可视化配置已保存', 'success');
    }
  } catch (err) {
    showMessage(String(err), 'error');
  }
}

document.querySelectorAll('.nav-item').forEach((item) => {
  item.addEventListener('click', () => openPage(item.dataset.page));
});
document.querySelectorAll('[data-open-page]').forEach((item) => {
  item.addEventListener('click', () => openPage(item.dataset.openPage));
});
document.querySelectorAll('.subtab').forEach((item) => {
  item.addEventListener('click', () => {
    syncVisualForms();
    const tab = item.dataset.configTab;
    document.querySelectorAll('.subtab').forEach((x) => x.classList.toggle('active', x === item));
    document.querySelectorAll('[data-config-panel]').forEach((x) => {
      x.classList.toggle('active', x.dataset.configPanel === tab);
    });
  });
});

$('refreshButton').addEventListener('click', () => refresh());
$('startButton').addEventListener('click', () => withAction('所有连接已启动', () => call('StartAllProfiles')));
$('stopButton').addEventListener('click', () => withAction('所有连接已停止', () => call('StopAllProfiles')));
$('restartButton').addEventListener('click', () => withAction('所有连接已重启', () => call('RestartAllProfiles')));

$('visualProfileSelect').addEventListener('change', (event) => switchProfile(event.target.value));
$('rawProfileSelect').addEventListener('change', (event) => switchProfile(event.target.value));

$('reloadVisualButton').addEventListener('click', () => loadVisualConfig(true));
$('previewVisualButton').addEventListener('click', previewVisualConfig);
$('saveVisualButton').addEventListener('click', () => saveVisualConfig(false));
$('saveRestartVisualButton').addEventListener('click', () => saveVisualConfig(true));
$('closePreviewButton').addEventListener('click', () => $('visualPreviewDialog').close());
$('addProfileButton').addEventListener('click', () => openProfileDialog());
$('addProfileFromStatusButton').addEventListener('click', () => openProfileDialog());
$('closeProfileDialogButton').addEventListener('click', () => $('profileDialog').close());
$('chooseProfileConfigButton').addEventListener('click', async () => {
  try {
    const value = await call('ChooseConfigFile');
    if (value) $('profileConfigPath').value = value;
  } catch (err) {
    showMessage(String(err), 'error');
  }
});
$('profileForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  try {
    const id = $('profileID').value.trim();
    const name = $('profileName').value.trim();
    const path = $('profileConfigPath').value.trim();
    const autoStart = $('profileAutoStart').checked;
    let state;
    if (id) {
      state = await call('UpdateProfile', {id, name, config_path: path, auto_start: autoStart});
    } else {
      state = await call('CreateProfile', name, path, autoStart);
    }
    $('profileDialog').close();
    configLoadedForPath = '';
    visualLoadedPath = '';
    visualConfig = null;
    render(state);
    showMessage(id ? '连接设置已保存' : '连接已添加', 'success');
  } catch (err) {
    showMessage(String(err), 'error');
  }
});
$('vcAuthMethod').addEventListener('change', updateAuthVisibility);

$('addProxyButton').addEventListener('click', addProxy);
$('deleteProxyButton').addEventListener('click', deleteProxy);
$('pxType').addEventListener('change', () => {
  syncSelectedProxyForm();
  renderProxyList();
  renderProxyEditor();
});
$('pxPluginType').addEventListener('change', updateProxyVisibility);
$('pxHealthType').addEventListener('change', updateProxyVisibility);
['pxName', 'pxLocalIP', 'pxLocalPort', 'pxRemotePort'].forEach((id) => {
  $(id).addEventListener('change', () => {
    syncSelectedProxyForm();
    renderProxyList();
    $('proxyEditorTitle').textContent = visualConfig.proxies[selectedProxyIndex]?.name || '未命名代理';
    $('proxyEditorSummary').textContent = proxySummary(visualConfig.proxies[selectedProxyIndex] || {});
  });
});

$('addVisitorButton').addEventListener('click', addVisitor);
$('deleteVisitorButton').addEventListener('click', deleteVisitor);
$('vsType').addEventListener('change', () => {
  syncSelectedVisitorForm();
  renderVisitorList();
  renderVisitorEditor();
});
['vsName', 'vsServerName', 'vsBindAddr', 'vsBindPort'].forEach((id) => {
  $(id).addEventListener('change', () => {
    syncSelectedVisitorForm();
    renderVisitorList();
    $('visitorEditorTitle').textContent = visualConfig.visitors[selectedVisitorIndex]?.name || '未命名 Visitor';
  });
});

$('loadConfigButton').addEventListener('click', async () => {
  await loadConfig(true);
  showMessage('已重新读取配置', 'success');
});

$('saveConfigButton').addEventListener('click', () => {
  withAction('配置已保存', async () => {
    const state = await call('SaveConfig', $('configEditor').value);
    visualLoadedPath = '';
    return state;
  });
});

$('validateButton').addEventListener('click', async () => {
  try {
    const message = await call('ValidateConfig');
    showMessage(message || '配置验证通过', 'success');
  } catch (err) {
    showMessage(String(err), 'error');
  }
});

$('chooseFrpcButton').addEventListener('click', async () => {
  try {
    const value = await call('ChooseFRPCExecutable');
    if (value) $('frpcPath').value = value;
  } catch (err) {
    showMessage(String(err), 'error');
  }
});

$('saveSettingsButton').addEventListener('click', async () => {
  try {
    const desiredLaunch = $('launchAtLogin').checked;
    const settings = {
      frpc_path: $('frpcPath').value.trim(),
      active_profile_id: lastState?.settings?.active_profile_id || '',
      profiles: (lastState?.profiles || []).map((item) => item.profile),
    };
    let state = await call('SaveSettings', settings);
    if (desiredLaunch !== Boolean(state.launch_at_login)) {
      state = await call('SetLaunchAtLogin', desiredLaunch);
    }
    render(state);
    showMessage('设置已保存', 'success');
  } catch (err) {
    showMessage(String(err), 'error');
  }
});

refresh();
setInterval(() => refresh(true), 2500);
