// 操作记录：后台每一次修改，可按类型筛选。
import { h, apiList, pageHead, work, ib, h2s, hint, note, kv, bullets, pill, badge, empty, searchBox, timeText, relTime } from '../core.js';
import { auditLabel, AUDIT_LABELS } from '../labels.js';

const KINDS = [
  ['all', '全部', null],
  ['player', '玩家', /^(player_update|grant_item|item_delete|keepnet_|skill_|kick|ban|mail_)/],
  ['play', '钓鱼玩法', /^(tunable|fish_|bite_mode|spawn_mode|hook_profile|bait_rule)/],
  ['shop', '商店', /^shop_/],
  ['login', '登录', /^login$/],
];

export default async function audit() {
  const list = await apiList('/api/audit?limit=200');
  let kind = 'all', q = '';
  const tbody = h('tbody');
  const count = h('span', { class: 'muted small' });
  const draw = () => {
    const re = KINDS.find(([k]) => k === kind)[2];
    const rows = list.filter((e) => {
      const a = e.Action ?? e.action;
      if (re && !re.test(a)) return false;
      if (!q) return true;
      return [auditLabel(a), a, e.Target ?? e.target, e.Detail ?? e.detail, e.Admin ?? e.admin].join(' ').toLowerCase().includes(q);
    });
    count.textContent = `${rows.length} 条`;
    tbody.replaceChildren(...(rows.length ? rows.map((e) => {
      const ts = e.TS ?? e.ts, a = e.Action ?? e.action;
      return h('tr', {},
        h('td', { class: 'nowrap' }, h('span', { text: relTime(ts) }), h('span', { class: 'sub', text: timeText(ts) })),
        h('td', {}, h('b', { text: auditLabel(a) }), AUDIT_LABELS[a] ? null : h('span', { class: 'sub mono', text: a })),
        h('td', { class: 'mono small', text: (e.Target ?? e.target) || '—' }),
        h('td', { class: 'small text-2', style: { whiteSpace: 'normal', maxWidth: '520px', wordBreak: 'break-all' }, text: (e.Detail ?? e.detail) || '' }),
        h('td', { class: 'muted small', text: e.Admin ?? e.admin ?? '' }));
    }) : [h('tr', {}, h('td', { colspan: 5 }, empty('没有记录')))]));
  };
  draw();
  const filters = h('div', { class: 'seg' }, KINDS.map(([k, l]) => h('button', { type: 'button', class: k === kind ? 'on' : '', onclick: (ev) => { kind = k; [...filters.children].forEach((b) => b.classList.toggle('on', b === ev.currentTarget)); draw(); } }, l)));
  const statDl = h('dl');
  const refreshStat = () => {
    const byKind = KINDS.slice(1).map(([k, l, re]) => [l, String(list.filter((e) => re.test(e.Action ?? e.action)).length)]);
    statDl.replaceChildren(
      h('div', {}, h('dt', { text: '记录条数' }), h('dd', { text: String(list.length) })),
      ...byKind.map(([l, n]) => h('div', {}, h('dt', { text: l }), h('dd', { text: n }))));
  };
  refreshStat();
  const aside = [
    ib('最近 200 条', '按类型可以分开看', statDl, hint('只读，不能改也不能删。')),
    ib('记什么', null, bullets([
      '后台里做过的每一次修改，包括改数值、发物品、封禁、发邮件。',
      '玩家在游戏里的行为不在这里。',
      '对象列是改的是谁，内容列是改成了什么。',
    ])),
  ];
  return [
    pageHead('审计日志', '管理后台里做过的每一次修改（最近 200 条）。'),
    work([
      h2s('操作记录'),
      hint('按类型筛选，或者搜对象名、内容。'),
      h('div', { style: { display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', margin: '0 0 12px' } },
        filters,
        h('div', { style: { flex: '1 1 180px', minWidth: '140px', maxWidth: '280px' } }, searchBox('搜索对象或内容', (v) => { q = v.toLowerCase(); draw(); })),
        h('span', { style: { flex: '1 1 auto' } }), count),
      h('div', { class: 'tbl', style: { maxHeight: 'calc(100vh - 330px)', overflow: 'auto' } }, h('table', {},
        h('thead', {}, h('tr', {}, h('th', { text: '时间' }), h('th', { text: '做了什么' }), h('th', { text: '对象' }), h('th', { text: '内容' }), h('th', { text: '操作人' }))), tbody)),
    ], aside),
  ];
}
