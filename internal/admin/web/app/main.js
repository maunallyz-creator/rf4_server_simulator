// RF4 管理后台入口：登录、框架、导航。各页面按需加载。
// 视觉与结构以演示页 RF4-管理工作台.html v177 为准：
// body 直接是两列网格（aside.rail + main），页面顶部是照片横幅 hero，下面是 .work 双栏。
import { h, $, api, post, icon, getToken, setToken, onAuth, route, render, navigate, toast, errText, parseHash, setHeroStats, loginDialog } from './core.js';

// 导航结构即信息架构：一条一个总类，副标题说明这一条管什么。
// 编号、图标、主标题、副标题四件套与演示页一致。
const NAV = [
  ['dashboard', '总览', '服务状态 · 在线 · 最近改动', 'home'],
  ['players', '玩家管理', '档案 · 资产 · 鱼护 · 技能 · 邮箱', 'users'],
  ['shop', '商店与库存', '改货架价格和库存', 'store'],
  ['rules', '钓场与鱼口', '玩法开关 · 出什么鱼 · 咬钩快慢', 'water'],
  ['waters', '水域与鱼种', '每个水域出哪些鱼', 'map'],
  ['hooks', '钩子手感', '真实型号 · 入口容量 · 咬口概率', 'hook'],
  ['bait', '鱼饵偏好', '每种鱼现在吃什么', 'bait'],
  ['mail', '邮件', '发消息和物品附件', 'mail'],
  ['tuning', '游戏参数', '手感 · 收益 · 活动', 'sliders'],
  ['release', '发布说明', '版本更新 · 修复内容', 'doc'],
  ['about', '关于', '项目说明 · README', 'info'],
  ['audit', '审计日志', '后台每次操作的留痕', 'list'],
];
const PAGES = {
  dashboard: () => import('./pages/dashboard.js'),
  players: () => import('./pages/players.js'),
  mail: () => import('./pages/mail.js'),
  rules: () => import('./pages/rules.js'),
  waters: () => import('./pages/waters.js'),
  bait: () => import('./pages/bait.js'),
  hooks: () => import('./pages/hooks.js'),
  tuning: () => import('./pages/tuning.js'),
  shop: () => import('./pages/shop.js'),
  audit: () => import('./pages/audit.js'),
  release: () => import('./pages/docs.js').then((m) => ({ default: m.changelog })),
  about: () => import('./pages/docs.js').then((m) => ({ default: m.about })),
};
for (const [name, load] of Object.entries(PAGES)) {
  route(name, { render: async (params) => (await load()).default(params) });
}

// 模块预取：左栏指到哪一条就把那页先下好，点下去不再等 import。
// 全部预取一次之后（首次渲染后 900ms 空闲时）切换就只剩渲染耗时。
function warm(name) { const load = PAGES[name]; if (load) load().catch(() => {}); }
let warmedAll = false;
function warmAllLater() {
  if (warmedAll) return;
  warmedAll = true;
  const run = () => Object.keys(PAGES).forEach(warm);
  if (typeof requestIdleCallback === 'function') requestIdleCallback(run, { timeout: 2500 });
  else setTimeout(run, 900);
}

const BUILD = 'v1.9';
let me = null;
let stats = null;

export function buildLabel() { return BUILD; }
function portNow() { return location.port && location.port !== '80' ? location.port : '9500'; }

function brand() {
  return h('div', { class: 'brand' },
    h('b', { text: 'RF4' }),
    h('span', { text: '管理工作台' }),
    h('em', { text: BUILD + ' · 本地 ' + portNow() }));
}

// 左栏：照片长条 + 品牌 + 总类导航 + 底部主题/账号。
function rail() {
  const nav = h('nav', { id: 'rail', 'aria-label': '总类导航' });
  NAV.forEach(([key, label, sub, ic], i) => {
    nav.append(h('a', { href: '#' + key, dataset: { route: key }, onpointerenter: () => warm(key), onfocus: () => warm(key) },
      h('span', { class: 'no', text: String(i + 1).padStart(2, '0') }),
      h('span', { class: 'cat' }, icon(ic)),
      h('span', { class: 'txt' }, h('b', { text: label }), h('small', { text: sub }))));
  });
  const name = (me && me.username) || 'admin';
  return h('aside', { class: 'rail' },
    h('div', { class: 'railband' }),
    brand(),
    nav,
    h('div', { class: 'rail-foot' },
      h('label', { text: '界面主题' }),
      themeMenu(),
      h('div', { class: 'who' },
        h('span', { text: name }),
        h('span', { text: getToken() ? '管理员' : '未登录' }),
        getToken()
          ? h('button', { class: 'who-out', type: 'button', title: '退出登录', 'aria-label': '退出登录', onclick: logout }, icon('logout', 15))
          : h('button', { class: 'who-out', type: 'button', title: '登录', 'aria-label': '登录', onclick: () => loginDialog() }, icon('logout', 15)))));
}

function inFrame() { try { return window.top !== window.self; } catch (e) { return false; } }

// ---- 顶部条：面包屑 + 版本戳 + 刷新 ----
function topBar() {
  return h('div', { class: 'top' },
    h('div', { class: 'crumb', id: 'crumb' },
      h('span', { text: 'RF4' }), h('i', { text: '/' }), h('span', { id: 'crumbGroup', text: '总览' })),
    h('div', { class: 'tools' },
      // 在控制中心的 iframe 里才出现「回控制中心」，单独打开 /admin/ 时不打扰
      inFrame() ? h('a', { class: 'ghost back-home', href: '/', target: '_top', text: '← 控制中心' }) : null,
      h('span', { id: 'stamp', text: '本地 · ' + portNow() + ' · ' + BUILD }),
      getToken() ? null : h('button', { class: 'ghost', id: 'loginBtn', type: 'button', onclick: () => loginDialog(), text: '登录' }),
      h('button', { class: 'ghost', id: 'reload', type: 'button', onclick: () => location.reload(), text: '刷新本页' })));
}

function pageFoot() {
  return h('footer', { class: 'pagefoot' },
    h('span', { text: '本地管理后台 · 数据来自正在运行的服务器' }),
    h('span', { text: '每一步操作都会写进「审计日志」' }));
}

function setCrumb(name) {
  const item = NAV.find(([k]) => k === name);
  const g = $('#crumbGroup');
  if (g) g.textContent = item ? item[1] : '总览';
  // 换页时清掉上一页留下的末级（二级页签名），避免面包屑挂错分类。
  const c = document.getElementById('crumb');
  if (c) c.querySelectorAll('i.tail,b').forEach((n) => n.remove());
  document.documentElement.dataset.route = name;
}

document.addEventListener('rf4:route', (e) => {
  for (const a of document.querySelectorAll('#rail a')) {
    if (a.dataset.route === e.detail.name) a.setAttribute('aria-current', 'page');
    else a.removeAttribute('aria-current');
  }
  setCrumb(e.detail.name);
});

// ---- 主题 ----
const THEMES = [['system', '跟随系统'], ['light', '浅色 · 骨白'], ['dark', '深色 · 墨黑']];
function applyTheme(v) {
  const root = document.documentElement;
  if (v === 'system') delete root.dataset.theme; else root.dataset.theme = v;
  try { localStorage.setItem('rf4.theme', v); } catch (e) {}
  const now = $('#themeNow');
  if (now) now.textContent = (THEMES.find(([k]) => k === v) || THEMES[0])[1];
  const menu = $('#themeMenu');
  if (menu) for (const li of menu.children) li.setAttribute('aria-selected', String(li.dataset.v === v));
}
function themeMenu() {
  let saved = 'system';
  try { saved = localStorage.getItem('rf4.theme') || 'system'; } catch (e) {}
  const menu = h('ul', { id: 'themeMenu', role: 'listbox', hidden: true },
    ...THEMES.map(([v, label]) => h('li', { role: 'option', dataset: { v }, onclick: () => { applyTheme(v); menu.hidden = true; } }, label)));
  const btn = h('button', { id: 'themeBtn', type: 'button', 'aria-haspopup': 'listbox', 'aria-expanded': 'false', onclick: () => {
    menu.hidden = !menu.hidden;
    btn.setAttribute('aria-expanded', String(!menu.hidden));
  } }, h('span', { id: 'themeNow', text: '跟随系统' }));
  applyTheme(saved);
  return h('div', { class: 'tselect' }, btn, menu);
}

// ---- 更新提示 ----
function safeURL(u) {
  return typeof u === 'string' && (u.startsWith('https://github.com/') || u.startsWith('https://gh-proxy.com/https://github.com/')) ? u : '';
}
export function updateBanner(u, slotEl) {
  const slot = slotEl || $('#updateSlot');
  if (!slot) return;
  if (!u || !u.available) { slot.replaceChildren(); return; }
  const links = [
    [u.download_mirror_url, '下载（国内加速）', 'primary'],
    [u.download_url, '从 GitHub 下载', ''],
    [u.channel === 'preview' ? '' : u.preview_download_mirror, '抢先体验预览版', 'ghost'],
    [u.notes_url, '看看更新了什么', 'ghost'],
  ].filter(([url]) => safeURL(url)).map(([url, label, kind]) => h('a', { class: 'pill ' + (kind === 'primary' ? 'ok' : ''), href: safeURL(url), target: '_blank', rel: 'noreferrer', text: label }));
  const dismiss = u.forced ? null : h('button', {
    class: 'ghost', text: '跳过这个版本', onclick: async () => {
      try {
        const r = await fetch('/api/update/dismiss', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ version: u.latest_version }) });
        if (!r.ok) throw new Error('HTTP ' + r.status);
        slot.replaceChildren();
      } catch (e) { toast('操作失败：' + errText(e), 'bad'); }
    },
  });
  slot.replaceChildren(h('div', { class: 'upd', style: { margin: '0 0 14px' } },
    h('b', { text: u.forced ? '需要升级' : '有新版本' }),
    h('span', { class: 'ver', text: u.latest_version }),
    h('span', { class: 'hint', style: { margin: '0' }, text: '当前 ' + BUILD + (u.size_text ? ' · 更新包 ' + u.size_text : '') }),
    h('span', { class: 'sp' }),
    links[0] || null,
    dismiss || null));
}

// ---- 公告条 ----
function noticeBar() {
  const K = [114, 70, 52, 35, 50, 48, 50, 54];
  const C = ['lNqYypOJ1a3co7Gu2oSL087G', '0pmi3466lvuoy7K11KrYoKCV', '17+k0sn90J6n2IaPldKczI68', '2obar6yR2pKZ39jR25++1amt', 'lv6ecmPUiJKU87XEjJTdiuh1', 'AxoAAwEDRnbXo7MCAANGcAQb', 'AADRtvN3BREKAQcER38CwLKx', 'AwdBcgIVBwcGAA=='];
  const KEY = 'rf4.n0t1ce.v2';
  try { if (localStorage.getItem(KEY) === '1') return null; } catch (e) {}
  let txt = '';
  try {
    const raw = atob(C.join(''));
    let s = '';
    for (let i = 0; i < raw.length; i++) s += String.fromCharCode(raw.charCodeAt(i) ^ K[i % K.length]);
    try { txt = decodeURIComponent(escape(s)); } catch (e) { txt = s; }
  } catch (e) { return null; }
  if (!txt) return null;
  const p = txt.lastIndexOf('：');
  const msg = p > 0 ? [txt.slice(0, p + 1), h('b', { text: txt.slice(p + 1) })] : [txt];
  const bar = h('div', { class: 'notice-bar' }, h('span', {}, msg),
    h('button', { 'aria-label': '关闭', title: '关闭', onclick: () => { bar.remove(); try { localStorage.setItem(KEY, '1'); } catch (e) {} } }, icon('x', 16)));
  return bar;
}

// ---- 登录 ----
function loginView() {
  const user = h('input', { id: 'lu', autocomplete: 'username', required: true });
  const pass = h('input', { id: 'lp', type: 'password', autocomplete: 'current-password', required: true });
  const submit = h('button', { class: 'primary', style: { width: '100%' }, type: 'submit', text: '登录' });
  const err = h('p', { class: 'alert', hidden: true });
  const form = h('form', {
    class: 'login-card',
    onsubmit: async (e) => {
      e.preventDefault();
      submit.disabled = true;
      err.hidden = true;
      try {
        const d = await post('/api/login', { username: user.value, password: pass.value });
        setToken(d.token);
      } catch (ex) {
        err.hidden = false;
        err.textContent = ex.status === 401 || ex.status === 403 ? '账号或密码不对。' : errText(ex);
      } finally { submit.disabled = false; }
    },
  },
  h('h1', { text: '登录管理后台' }),
  h('p', { class: 'lede', text: '管理玩家、钓鱼玩法、商店和系统设置' }),
  err,
  h('label', { class: 'field' }, h('span', { text: '账号' }), user),
  h('label', { class: 'field' }, h('span', { text: '密码' }), pass),
  submit,
  h('p', { class: 'hint', html: '忘记密码？账号和密码写在服务器目录的 <code>config.yaml</code> 里（<code>admin.username</code> / <code>admin.password</code>）。发布包默认是 <code>admin</code> / <code>admin123</code>。' }));
  return h('div', { class: 'login' }, form);
}

async function boot() {
  const app = $('#app');
  me = null;
  if (getToken()) {
    try { me = await api('/api/me'); }
    catch (e) {
      if (e.status === 401 || e.status === 403) { setToken(''); }
      else if (!e.status) {
        app.replaceChildren(h('div', { class: 'login' }, h('p', { class: 'alert' }, '连接不上管理后台：' + errText(e))));
        return;
      }
    }
  }
  try { stats = await api('/api/dashboard'); } catch (e) { stats = null; }
  setHeroStats(stats);
  const notice = noticeBar();
  app.replaceChildren(
    rail(),
    h('main', {},
      topBar(),
      h('div', { id: 'view' }),
      h('div', { id: 'updateSlot' }),
      pageFoot(),
      h('div', { id: 'noticeSlot' }, notice || '')));
  updateBanner((me && me.update) || (stats && stats.update));
  render();
  warmAllLater();
}

export function logout() {
  setToken('');
}
onAuth(() => boot());
boot();
export function currentUser() { return me; }
export { parseHash };
