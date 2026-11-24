import { h, api, apiList, fmt, tsNum, timeText, clockText, icon, navigate, empty, pageHead, work, ib, h2s, h3s, hint, note, kv, bullets, pill, brief, tbl, logsBlock, startGrid, searchBox, btn, render, setPageCleanup } from '../core.js';
import { auditLabel } from '../labels.js';
import { updateBanner, buildLabel } from '../main.js';

// 演示页的「从这里开始」是一条一条完整流程：看现状 → 改 → 核对。
const STARTS = [
  ['rules', 'hook', '调整咬钩速度', '多久咬钩、浮标和跑鱼分开调'],
  ['waters', 'water', '管理水域鱼种', '每个水域出什么鱼、出多少条'],
  ['bait', 'bait', '查鱼饵偏好', '每种鱼现在吃什么，改完再刷新'],
  ['shop', 'store', '改货架价格', '一件件改价格和库存，保存前可放弃'],
  ['players', 'users', '管理玩家', '档案、资产、鱼护、技能、邮箱'],
];
const RULES = [
  ['fish_bait_rules_enabled', '鱼会挑饵'], ['fish_distribution_enabled', '鱼群分布'], ['fish_weather_enabled', '天气与水温'],
  ['fish_ecology_enabled', '鱼种习性'], ['bite_gate_enabled', '饵要对口'], ['happy_hour_enabled', '欢乐时光'],
];

export default async function dashboard() {
  const [d, audits, tuns, mode, stores] = await Promise.all([
    api('/api/dashboard'),
    apiList('/api/audit?limit=24').catch(() => []),
    api('/api/tunables').catch(() => ({})),
    api('/api/fish/bite_mode').catch(() => ({})),
    apiList('/api/stores').catch(() => []),
  ]);
  setPageCleanup(null);

  const modeName = { realistic: '真实鱼汛', legacy: '全随机（旧版）', mixed: '自定义' }[mode.mode] || '未上报';
  const on = (k) => (tuns[k] === undefined ? k === 'happy_hour_enabled' : tuns[k] === '1');
  const enabled = RULES.filter(([k]) => on(k)).map(([, n]) => n);
  const overrideCount = stores.reduce((n, s) => n + (s.override_count || 0), 0);
  const pg = d.pg || '';
  const port = location.port || '9500';

  const updates = [...audits]
    .sort((a, b) => tsNum(b.TS ?? b.ts) - tsNum(a.TS ?? a.ts))
    .map((e) => ({
      ts: e.TS ?? e.ts,
      admin: e.Admin ?? e.admin ?? '',
      what: auditLabel(e.Action ?? e.action),
      target: (e.Target ?? e.target) || '',
      detail: (e.Detail ?? e.detail) || '',
    }));

  // 关键口径：只写能从接口读出来的东西，读不到就不出现。
  const canons = [];
  canons.push(['发鱼方式', mode.spawn_mode === 'tier' ? '档位比例' : (mode.spawn_mode || modeName), 'rules']);
  if (tuns.grade_pct_qualified !== undefined) {
    canons.push(['四档占比', `未达标 ${tuns.grade_pct_below || '0'}% · 合格 ${tuns.grade_pct_qualified}% · 奖杯 ${tuns.grade_pct_trophy || '0'}% · 蓝冠 ${tuns.grade_pct_super || '0'}%`, 'rules']);
  }
  if (tuns.bite_delay_max !== undefined) {
    canons.push(['咬钩节奏', `普通上限 ${tuns.bite_delay_max} 秒${tuns.bite_delay_lure_max ? ' / 饵上限 ' + tuns.bite_delay_lure_max + ' 秒' : ''}`, 'rules']);
  }
  canons.push(['玩法开关', enabled.length ? enabled.join(' · ') : '全部关闭', 'rules']);
  if (stores.length) canons.push(['商店覆盖', overrideCount ? `${overrideCount} 件商品有覆盖价` : '没有覆盖，全部走基础价', 'shop']);
  canons.push(['饵要对口', on('bite_gate_enabled') ? '开：饵不对口不咬' : '关：只要饵能入口就咬', 'bait']);

  const search = { q: '' };
  const logsBox = h('div', {});
  const drawLogs = () => {
    const q = search.q.trim().toLowerCase();
    const rows = updates.filter((u) => !q || (u.what + u.target + u.admin + u.detail).toLowerCase().includes(q)).slice(0, 12);
    logsBox.replaceChildren(rows.length
      ? logsBlock(rows.map((u) => ({ time: clockText(u.ts), who: u.admin || 'system', text: `${u.what} ${u.target}` })))
      : empty('没有匹配的记录', '换个关键词，或点「刷新」重新拉一次审计表。'));
  };
  drawLogs();

  const updSlot = h('div', {});
  updateBanner(d.update, updSlot);

  return [
    pageHead('总览', '服务器现在什么状态、当前生效的是哪套参数、最近动过什么。', h('a', {
      class: 'btn primary home-link',
      href: '/',
      target: '_top',
      title: '控制中心：开始游戏、停止服务、看进程状态和日志',
    }, '控制中心 →')),
    work([
      updSlot,
      brief([
        [fmt(d.sessions), '在线玩家'],
        [fmt(d.players), '档案'],
        [fmt(d.tunables), '可调参数'],
        [fmt(d.items), '物品'],
      ]),
      h2s('从这里开始'),
      hint('每条进去都是一整条流程：先看现在是什么，改完核对，最后保存。'),
      startGrid(STARTS.map(([r, ic, t, s]) => ({
        icon: ic, title: t, sub: s, href: '#' + r,
        onClick: (e) => { e.preventDefault(); navigate(r); },
      }))),
      h3s('运行日志'),
      hint('后台审计表里最近的记录，可以按动作、对象或管理员筛。', { margin: '0 0 8px' }),
      h('div', { class: 'search' },
        searchBox('搜动作、对象或管理员', (v) => { search.q = v; drawLogs(); }),
        btn('刷新', () => render(), 'ghost')),
      logsBox,
      note('日志的来源。', '这里读的是后台审计表，不是游戏进程的 stdout；谁在什么时候改了哪一项，都在这里。'),
      h3s('当前生效的关键口径'),
      tbl(['项目', '现在是什么', '去哪改'], canons.map(([k, v, r]) => [
        k,
        v,
        h('a', { class: 'pill', href: '#' + r, text: '去改', onclick: (e) => { e.preventDefault(); navigate(r); } }),
      ]), { cls: 'canon' }),
    ], [
      ib('服务器概况', '这个页面读到的实时值。', [
        kv([
          ['服务地址', location.host || '127.0.0.1:' + port],
          ['数据库', pg === 'up' ? '已连接' : (pg || '未上报')],
          ['在线玩家', fmt(d.sessions)],
          ['建档角色', fmt(d.players)],
          ['物品总数', fmt(d.items)],
          ['可调参数', fmt(d.tunables) + ' 项'],
          ['审计记录', fmt(d.audits) + ' 条'],
          ['前端版本', buildLabel()],
        ]),
      ]),
      ib('最近修改', '按时间倒序，只列最近几条。', [
        updates.length
          ? kv(updates.slice(0, 5).map((u) => [clockText(u.ts), h('span', {}, h('b', { text: u.what }), u.target ? h('span', { class: 'mono', text: ' · ' + u.target }) : null)]))
          : empty('还没有修改记录'),
        h('p', { class: 'hint' }, h('a', { href: '#audit', text: '看全部审计日志 →', onclick: (e) => { e.preventDefault(); navigate('audit'); } })),
      ]),
      ib('这一页的数据', '没有假数字，读不到就写「未上报」。', bullets([
        h('span', {}, '在线、档案、物品、参数条数来自 ', h('b', { text: '/api/dashboard' }), '。'),
        h('span', {}, '关键口径来自 ', h('b', { text: '/api/tunables' }), ' 与 ', h('b', { text: '/api/fish/bite_mode' }), '。'),
        h('span', {}, '日志来自审计表 ', h('b', { text: '/api/audit' }), '，不是进程日志。'),
      ])),
    ]),
  ];
}
