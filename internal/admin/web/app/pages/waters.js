// 水域与鱼种：每个水域出哪些鱼、各占多少、咬钩快慢。
import { zonesEditor } from './zones.js';
import {
  h, api, apiList, post, toast, errText, pageHead, pageTabs, work, ib, h2s, h3s, hint, note, kv, bullets, pill, badge, btn, empty, loading,
  searchBox, debounce, dialog, confirmDialog, navigate, Form, savebar, setLeaveGuard, render, webURL, fmt, setCrumbTail,
} from '../core.js';

const ORDER = ['克马羚诺也湖', '埃尔克湖', '惟有诺克河', '旧奥斯特罗格湖', '白河', '廓里湖', '梅德韦杰湖', '沃尔霍夫河', '北顿涅茨河', '苏拉河', '拉多加湖', '琥珀湖', '拉多加湖群岛', '阿赫图巴河', '铜湖', '下通古斯卡河', '亚马河', '挪威海'];

export function fishIcon(key, w = 64) {
  const img = h('img', { src: webURL('/icons/fish/' + encodeURIComponent(String(key || '').toLowerCase().replace(/\./g, '_')) + '.png'), alt: '', loading: 'lazy', style: { width: w + 'px', height: (w / 2) + 'px', objectFit: 'contain', flex: 'none' } });
  img.onerror = () => { img.style.display = 'none'; };
  return img;
}

export default async function waters(params = []) {
  const [areas, zonesData] = await Promise.all([api('/api/fish/areas'), api('/api/fish/zones').catch(() => ({ zones: [] }))]);
  const list = (Array.isArray(areas) ? areas : []).slice().sort((a, b) => {
    const ia = ORDER.indexOf(a.loc), ib = ORDER.indexOf(b.loc);
    return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib);
  });
  const zones = zonesData.zones || [];
  const loc = params[0] && list.some((a) => a.loc === params[0]) ? params[0] : (list[0] && list[0].loc);
  const tab = params[1] === 'zones' ? 'zones' : 'fish';
  const TABS = [['fish', '鱼种'], ['zones', '鱼群区域']];
  setCrumbTail((TABS.find(([k]) => k === tab) || [])[1]);
  const tabsEl = pageTabs(TABS, tab, (k) => navigate('waters', loc, k));

  if (!loc) {
    return [pageHead('水域与鱼种', '每个水域出哪些鱼、各占多少。'), tabsEl,
      work([callout2('info', '还没有读到水域数据。')], [ib('说明', null, hint('检查游戏服务是否在运行。'))])];
  }

  if (tab === 'zones') {
    const cfg = await api('/api/fish/config?loc=' + encodeURIComponent(loc)).catch(() => ({ fish: [] }));
    const area = list.find((a) => a.loc === loc);
    const picker = h('select', { class: 'select', style: { width: '220px' }, 'aria-label': '选择水域', onchange: (e) => navigate('waters', e.target.value, 'zones') },
      list.map((a) => h('option', { value: a.loc, text: a.loc + (a.map_kind === 'sea' ? '（海图）' : ''), selected: a.loc === loc })));
    const head = pageHead('水域与鱼种', '在地图上画出一块区域，规定那里出哪些鱼。');
    return [head, tabsEl, work([h('div', { style: { margin: '0 0 14px', display: 'flex', gap: '10px', alignItems: 'center' } }, h('span', { class: 'hint', text: '正在看' }), picker),
      zonesEditor({ area, zones: zones.filter((z) => z.loc === loc), roster: cfg.fish || [] })])];
  }

  let cfg = { fish: [], setting: {} }, prefs = { rows: [] };
  try {
    [cfg, prefs] = await Promise.all([
      api('/api/fish/config?loc=' + encodeURIComponent(loc)),
      api('/api/bait/prefs?limit=2000&loc=' + encodeURIComponent(loc)).catch(() => ({ rows: [] })),
    ]);
  } catch (e) { cfg = { fish: [], setting: {} }; }
  const baitByKey = Object.fromEntries((prefs.rows || []).map((r) => [r.fish_key, r]));

  const area = list.find((a) => a.loc === loc) || {};
  const seg = h('div', { class: 'seg areas' }, ...list.map((a) => h('button', {
    type: 'button', class: a.loc === loc ? 'on' : '', onclick: () => { if (a.loc !== loc) navigate('waters', a.loc, tab); },
  }, a.loc)));
  const saved = (cfg.fish || []).filter((f) => f.enabled && Number(f.weight) > 0);
  const sum = saved.reduce((n, f) => n + Number(f.weight), 0) || 1;
  const otherZones = (zonesData.zones || []).filter((z) => z.loc === loc).length;

  const aside = [
    ib('已保存权重', saved.length ? null : '这个水域现在一条鱼都不出', saved.length ? h('dl', {},
      ...saved.slice(0, 10).map((f) => h('div', {}, h('dt', { text: f.name_zh || f.fish_key }), h('dd', { text: `${fmt(f.weight)} · ${(Number(f.weight) / sum * 100).toFixed(1)}%` })))) : hint('把下面的「出现」勾上几条鱼。'),
      h3s('其他水域'), h('div', { class: 'pills' }, ...list.slice(0, 8).map((a) => h('span', { class: 'pill', text: `${a.loc} ${a.fish_count} 种` })))),
    ib('这一页管什么', '只影响当前水域', bullets([
      '出现：取消勾选等于这个水域不出这种鱼。',
      '权重：相对值，占比列是换算结果。',
      '爱吃的饵：来自「鱼饵偏好」，这里只读。',
      '每个水域各自草稿，来回切换不会串。',
    ]), hint(otherZones ? `这个水域划了 ${otherZones} 个鱼群区域，在「鱼群区域」页签改。` : '还没划鱼群区域。')),
  ];

  return [
    pageHead('水域与鱼种', '每个水域出哪些鱼、各占多少；鱼群区域在这里画。'),
    tabsEl,
    work([h2s(loc), hint(`${(cfg.fish || []).length} 种鱼 · ${area.map_kind === 'sea' ? '海图' : '内陆'} · 仅影响此水域`), seg, fishView(cfg, baitByKey)], aside),
  ];
}

function callout2(kind, text) {
  return h('div', { class: 'note' }, h('b', { text: '提示' }), h('span', { text }));
}

// ---------- 鱼种名册 ----------
// 爱吃的饵：静态规则来自鱼饵偏好；轮换只认名册行里后端给的 bait_groups / bait_rotate_sec
function baitLine(row, pref) {
  const allow = (pref && pref.allow) || [];
  const groups = (row.bait_groups || []).filter((g) => g && g.length);
  const rotating = groups.length > 1 && Number(row.bait_rotate_sec) > 0;
  const txt = allow.length
    ? '吃 ' + allow.slice(0, 3).join('、') + (allow.length > 3 ? ` 等 ${allow.length} 种` : '')
    : '';
  if (!txt && !rotating) return null;
  return h('div', { class: 'hint', style: { margin: '2px 0 0', fontSize: '11.8px' }, text: txt + (rotating ? (txt ? ' · ' : '') + `轮换 ${groups.length} 组 / 每 ${Math.round(row.bait_rotate_sec / 60)} 分` : '') });
}

function fishView(cfg, baitByKey = {}) {
  const fish = cfg.fish.map((f) => ({ ...f }));
  const s = cfg.setting || {};
  const form = new Form();
  let dirtyRows = false;
  const isDirty = () => form.isDirty() || dirtyRows;
  const bar = savebar(form, {
    onSave: async () => {
      const setting = { ...s,
        bite_probability: +form.value('bite_probability'), bite_delay_fixed: +form.value('bite_delay_fixed'), bite_delay_scale: +form.value('bite_delay_scale'),
        bite_delay_max: +form.value('bite_delay_max'), bite_delay_lure_max: +form.value('bite_delay_lure_max') };
      await post('/api/fish/config', { area: cfg.area, setting, fish });
      dirtyRows = false;
      toast('已保存，立即生效');
      setTimeout(render, 200);
    },
  });
  // 鱼种表的改动不经过 Form 字段，手动把它算进「未保存」
  const origDirty = form.dirtyKeys.bind(form);
  form.dirtyKeys = () => { const k = origDirty(); return dirtyRows && !k.length ? ['_fish'] : k; };
  const origValues = form.values.bind(form);
  form.values = (keys) => origValues((keys || []).filter((k) => k !== '_fish'));
  const origReset = form.reset.bind(form);
  form.reset = () => { dirtyRows = false; origReset(); render(); };
  setLeaveGuard(isDirty);
  const touch = () => { dirtyRows = true; form.refresh(); };

  let q = '';
  const tbody = h('tbody');
  const total = () => fish.filter((f) => f.enabled && Number(f.weight) > 0).reduce((n, f) => n + Number(f.weight), 0) || 1;
  const draw = () => {
    const sum = total();
    const rows = fish.filter((f) => !q || `${f.fish_key} ${f.name_zh}`.toLowerCase().includes(q));
    tbody.replaceChildren(...(rows.length ? rows.map((f) => {
      const on = h('input', { type: 'checkbox', 'aria-label': '启用 ' + f.name_zh, onchange: (e) => { f.enabled = e.target.checked; touch(); draw(); } });
      on.checked = !!f.enabled;
      const w = h('input', { class: 'input num', style: { width: '90px', height: '30px' }, value: f.weight, onchange: (e) => { const n = Number(e.target.value); f.weight = e.target.value.trim() !== '' && Number.isFinite(n) ? n : f.base_weight; f.has_override = true; touch(); draw(); } });
      const pct = f.enabled && Number(f.weight) > 0 ? (Number(f.weight) / sum * 100) : 0;
      return h('tr', { style: { opacity: f.enabled ? 1 : 0.5 } },
        h('td', {}, on),
        h('td', {}, h('div', { style: { display: 'flex', alignItems: 'center', gap: '10px' } }, fishIcon(f.fish_key, 42),
          h('div', { style: { minWidth: 0 } }, h('b', { text: f.name_zh }), h('div', { class: 'mono', style: { fontSize: '11.6px', color: 'var(--muted)' }, text: f.fish_key }),
            baitLine(f, baitByKey[f.fish_key])))),
        h('td', {}, w, h('span', { class: 'sub', text: '默认 ' + f.base_weight })),
        h('td', { style: { width: '148px' } }, h('div', { style: { display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'nowrap' } },
          h('div', { style: { flex: '0 0 84px', height: '6px', borderRadius: '3px', background: 'var(--line-soft)', overflow: 'hidden' } }, h('div', { style: { width: Math.min(100, pct) + '%', height: '100%', background: 'var(--accent)' } })),
          h('span', { class: 'hint', style: { width: '46px', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }, text: pct ? pct.toFixed(1) + '%' : '—' }))),
        h('td', { class: 'num', style: { whiteSpace: 'nowrap' } },
          f.has_override ? badge('改过', 'warn') : null,
          btn('移除', () => {
            const i = fish.findIndex((x) => x.fish_key === f.fish_key);
            if (i >= 0) { fish.splice(i, 1); touch(); draw(); }
          }, 'ghost sm')));
    }) : [h('tr', {}, h('td', { colspan: 5 }, empty('没有匹配的鱼')))]));
  };
  draw();

  const params = h('div', {},
    form.row({ key: 'bite_probability', label: '每次抛竿有鱼的概率', help: '1 表示每竿必有鱼咬。', kind: 'float', min: 0, max: 1, def: '1' }, s.bite_probability),
    form.row({ key: 'bite_delay_scale', label: '咬钩速度', help: '0.5 = 快一倍，2 = 慢一倍。', kind: 'float', min: 0, unit: '倍', def: '1' }, s.bite_delay_scale),
    form.row({ key: 'bite_delay_max', label: '最长等待', help: '普通钓法等这么久一定有鱼咬。', kind: 'float', min: 1, unit: '秒', def: '20' }, s.bite_delay_max),
    form.row({ key: 'bite_delay_lure_max', label: '路亚最长等待', kind: 'float', min: 0.1, unit: '秒', def: '6' }, s.bite_delay_lure_max),
    form.row({ key: 'bite_delay_fixed', label: '固定咬钩时间', help: '0 表示随机；填了数字后每次都在这个时间咬钩。', kind: 'float', min: 0, unit: '秒', def: '0' }, s.bite_delay_fixed));

  return h('div', { class: 'stack' },
    h('details', { class: 'advanced', style: { borderTop: 0, marginTop: '-6px' } }, h('summary', {}, '这个水域的咬钩快慢 ', h('small', { text: `（现在：${Number(s.bite_delay_fixed) > 0 ? '固定 ' + s.bite_delay_fixed + ' 秒' : '最长 ' + s.bite_delay_max + ' 秒'}）` })), params),
    h('div', { style: { display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', margin: '0 0 10px' } },
      h('div', { style: { flex: '1 1 200px', minWidth: '150px', maxWidth: '320px' } }, searchBox('筛选鱼名', (v) => { q = v.toLowerCase(); draw(); })),
      btn(q ? '启用筛选到的' : '全部启用', () => { fish.forEach((f) => { if (!q || `${f.fish_key} ${f.name_zh}`.toLowerCase().includes(q)) f.enabled = true; }); touch(); draw(); }, 'ghost sm'),
      btn(q ? '关闭筛选到的' : '全部关闭', () => { fish.forEach((f) => { if (!q || `${f.fish_key} ${f.name_zh}`.toLowerCase().includes(q)) f.enabled = false; }); touch(); draw(); }, 'ghost sm'),
      btn('恢复默认', async () => {
        if (!(await confirmDialog({ title: `把「${cfg.area}」恢复默认？`, desc: '清除这个水域所有的修改，回到游戏自带的鱼种和比例。', confirm: '恢复默认', danger: true }))) return;
        try { await post('/api/fish/config/reset', { area: cfg.area }); toast('已恢复默认'); dirtyRows = false; render(); } catch (e) { toast(errText(e), 'bad'); }
      }, 'ghost sm'),
      btn('添加鱼种', () => addFish(fish, () => { touch(); draw(); }), 'sm', { icon: 'plus' })),
    h('p', { class: 'muted small', text: '「权重」决定这种鱼出现的多少：权重越大越常见，右边的百分比是换算后的占比。关闭或权重填 0 表示这里不出这种鱼。' }),
    h('div', { class: 'tbl', style: { maxHeight: 'calc(100vh - 360px)', overflow: 'auto' } }, h('table', {},
      h('thead', {}, h('tr', {}, h('th', { text: '出现' }), h('th', { text: '鱼' }), h('th', { class: 'num', text: '权重' }), h('th', { class: 'num', text: '占比' }), h('th', { class: 'num', text: '' }))), tbody)));
}

function addFish(fish, done) {
  const results = h('div', { class: 'card', style: { maxHeight: '400px', overflow: 'auto' } }, empty('输入鱼名开始搜索'));
  const search = debounce(async (q) => {
    const list = await apiList('/api/fish/pool?limit=80&q=' + encodeURIComponent(q)).catch(() => []);
    results.replaceChildren(list.length ? h('div', { class: 'list' }, list.map((x) => {
      const has = fish.some((f) => f.fish_key === x.fish_key);
      return h('div', { class: 'list-item' + (has ? '' : ' clickable'), style: { opacity: has ? 0.5 : 1 }, onclick: has ? null : () => {
        fish.push({ fish_key: x.fish_key, name_zh: x.name_zh, base_weight: 0, weight: 1, enabled: true, has_override: true });
        close(); done(); toast(`已加入「${x.name_zh}」，记得保存`);
      } }, fishIcon(x.fish_key), h('div', { class: 'grow' }, h('div', { class: 'title', text: x.name_zh }), h('div', { class: 'sub mono', text: x.fish_key })),
      has ? badge('已在名单里') : h('span', { class: 'muted small', text: `${fmt(x.min_g)}–${fmt(x.max_g)} g` }));
    })) : empty('没有找到'));
  }, 250);
  const { close } = dialog({ title: '添加鱼种', size: 'lg', body: h('div', { class: 'stack' }, searchBox('输入鱼名，如 狗鱼', search), results) });
}
