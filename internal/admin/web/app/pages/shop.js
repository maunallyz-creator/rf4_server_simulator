// 商店：货架价格 / 库存。结构照演示页——hero + 二级页签 + 两栏（左表格右信息块）。
import { h, api, apiList, post, toast, errText, pageHead, pageTabs, work, ib, h2s, h3s, hint, note, brief, kv, pill, empty, btn, Form, setLeaveGuard, setPageCleanup, confirmDialog, fmt, navigate, route, setCrumbTail, setHeroNumbers } from '../core.js';

const plain = (c) => { const v = Number(c || 0) / 100; return (Math.round(v * 100) % 100 === 0 ? String(Math.round(v)) : v.toFixed(2)); };
const money = (c) => { const v = Number(c || 0) / 100; const digits = Math.round(v * 100) % 100 === 0 ? 0 : 2; return v.toLocaleString('zh-CN', { minimumFractionDigits: digits, maximumFractionDigits: 2 }); };

const STORE_KIND = [
  [/^fishing_?store/, '渔具店'], [/^food_?store/, '食品店'], [/^tool_store/, '工具店'], [/^workshop/, '工坊'], [/^boatstation/, '船坞'],
  [/^atm/, '取款机'], [/^admin/, '水域商店'], [/^monkeycola/, '饮料售货机'], [/^ny_?ba+za+r/i, '新年集市'], [/^cafestore/, '咖啡馆'],
  [/^bakery/, '面包店'], [/^craft_baits/, '饵料制作'], [/^craft_lures/, '拟饵制作'], [/^farmstore/, '农场商店'], [/^fishstore/, '鱼店'],
  [/^housestore/, '房屋商店'], [/^milkstore/, '乳品店'], [/^sawmill/, '锯木厂'], [/^mill/, '磨坊'], [/^paseka/, '养蜂场'],
  [/^rentbike/, '租车'], [/^tours/, '旅行社'], [/^arbuz/, '西瓜摊'],
];
const storeKind = (id) => (STORE_KIND.find(([re]) => re.test(id)) || [null, id])[1];

// 物品类别：真服 item_id 前缀本身就带品类，按前缀读，不猜。
const ITEM_KIND = [
  [/^rod_/, '鱼竿'], [/^reel_/, '渔线轮'], [/^line_/, '鱼线'], [/^bait_boil/, '饵料'], [/^bait_/, '饵料'],
  [/^lure_/, '拟饵'], [/^hook_/, '鱼钩'], [/^float_/, '浮漂'], [/^feeder/, '喂食器'], [/^container/, '容器'], [/^food_/, '食品'],
  [/^papermap|^map_/, '地图'], [/^spare_/, '备件'], [/^tool_/, '工具'],
];
const itemKind = (id) => (ITEM_KIND.find(([re]) => re.test(id || '')) || [null, ''])[1];

const TABS = [
  { key: 'price', label: '货架价格', href: '#shop/price' },
  { key: 'stock', label: '库存', href: '#shop/stock' },
];

export default async function shop(params) {
  const tabKey = params[1] === 'stock' ? 'stock' : 'price';
  const tab = TABS.find((t) => t.key === tabKey);
  setCrumbTail(tab.label);

  const [stores, dash] = await Promise.all([apiList('/api/stores'), api('/api/dashboard').catch(() => ({}))]);
  const catalogTotal = Number(dash.items || 0);
  const storeId = params[0] && stores.some((s) => s.store_id === params[0]) ? params[0] : (stores[0] && stores[0].store_id);
  const cur = stores.find((s) => s.store_id === storeId);

  const body = h('div', {}, empty('读取中…'));
  const aside = h('div');
  const tabsEl = pageTabs(TABS.map((t) => [t.key, t.label]), tabKey, (k) => navigate('shop', (storeId ? storeId + '/' : '') + k));

  const form = new Form();
  setLeaveGuard(() => form.isDirty());
  setPageCleanup(() => form.reset());

  let items = [];

  const priceCell = (x, field) => {
    const key = x.item_id + '|' + field;
    const base = x['base_' + field];
    const shown = x.has_override ? (field === 'stock' ? String(x[field] ?? '') : plain(x[field])) : '';
    let f = form.fields.get(key);
    if (!f) f = form.field({ key, label: field, kind: field === 'stock' ? 'int' : 'float', min: 0 }, shown);
    const input = h('input', {
      class: 'input num',
      type: 'number', min: '0', step: field === 'stock' ? '1' : '0.01',
      'aria-label': `${x.name || x.item_id}覆盖${field === 'stock' ? '库存' : '价'}`,
      value: String(f.input ? f.input.value : f.original),
      placeholder: base == null || base === 0 ? '' : (field === 'stock' ? String(base) : plain(base)),
      oninput: () => { f.input = input; form.refresh(); },
    });
    f.input = input;
    return h('span', { class: 'unit' }, input, h('i', { text: field === 'stock' ? '件' : '银' }));
  };

  const saveBtn = btn('核对并保存', async () => {
    const keys = form.dirtyKeys();
    if (!keys.length) return;
    if (!form.validate(keys)) { toast('有填写不正确的项，请检查输入框', 'bad'); return; }
    saveBtn.disabled = true;
    try {
      const touched = new Set(keys.map((k) => k.split('|')[0]));
      const read = (id, field) => {
        const f = form.fields.get(id + '|' + field);
        const raw = f && f.input ? String(f.input.value).trim() : '';
        if (raw === '') return null;
        return field === 'stock' ? Math.floor(+raw) : Math.round(+raw * 100);
      };
      let saved = 0, reset = 0;
      for (const id of touched) {
        const x = items.find((it) => it.item_id === id) || {};
        const sv = read(id, 'silver'), gv = read(id, 'gold'), stv = read(id, 'stock');
        if (sv === null && gv === null && stv === null) {
          await post('/api/stores/items/reset', { store_id: storeId, item_id: id });
          reset += 1;
        } else {
          await post('/api/stores/items/update', {
            store_id: storeId, item_id: id,
            silver: sv === null ? (x.silver || 0) : sv,
            gold: gv === null ? (x.gold || 0) : gv,
            stock: stv === null ? (x.stock || 0) : stv,
          });
          saved += 1;
        }
      }
      toast(`已保存 ${saved} 件` + (reset ? `，${reset} 件恢复默认` : ''));
      form.commit(form.values(keys));
      setTimeout(() => { form.reset(); redraw(); }, 120);
    } catch (e) {
      toast(errText(e), 'bad');
    } finally {
      saveBtn.disabled = false;
    }
  }, 'primary');
  saveBtn.disabled = true;

  const actions = h('footer', { class: 'actions' },
    btn('放弃修改', () => { form.reset(); redraw(); }, 'ghost'),
    saveBtn);

  const syncActions = () => { saveBtn.disabled = !form.isDirty(); };
  form.onChange = syncActions;

  function tableEl() {
    const isPrice = tabKey === 'price';
    const head = isPrice
      ? ['物品', '类别', ['基础价', 'num'], ['覆盖价', 'num'], '状态', ['', 'num']]
      : ['物品', '类别', ['默认库存', 'num'], ['覆盖库存', 'num'], '状态', ['', 'num']];
    const rows = items.map((x) => {
      const kind = itemKind(x.item_id);
      const tds = [
        h('td', {}, h('span', { class: 'ic', text: (x.name || x.item_id || '?').slice(0, 1) }), h('b', { text: x.name || x.item_id }), h('div', { class: 'mono', text: x.item_id })),
        h('td', { text: kind || '—' }),
      ];
      if (isPrice) {
        tds.push(h('td', { class: 'num', text: money(x.base_silver) + (x.base_gold ? ' / ' + money(x.base_gold) : '') }));
        tds.push(h('td', { class: 'num' }, priceCell(x, 'silver'), x.base_gold ? priceCell(x, 'gold') : null));
      } else {
        tds.push(h('td', { class: 'num', text: x.base_stock == null ? '—' : String(x.base_stock) }));
        tds.push(h('td', { class: 'num' }, priceCell(x, 'stock')));
      }
      tds.push(h('td', {}, pill(x.has_override ? '覆盖' : '默认', x.has_override ? 'ok' : '')));
      tds.push(h('td', { class: 'num' }, x.has_override
        ? btn('恢复默认', async () => {
          if (!(await confirmDialog({ title: `把「${x.name || x.item_id}」恢复默认？`, desc: '价格和库存回到游戏自带的数值。', confirm: '恢复' }))) return;
          try { await post('/api/stores/items/reset', { store_id: storeId, item_id: x.item_id }); toast('已恢复默认'); form.reset(); reload(); } catch (e) { toast(errText(e), 'bad'); }
        }, 'ghost')
        : h('span', { class: 'hint', style: { margin: '0' }, text: '—' })));
      return h('tr', {}, tds);
    });
    if (!rows.length) rows.push(h('tr', {}, h('td', { colspan: '6' }, empty('这家店还没登记商品', '服务端只给每个水域商店登记了少量基础货架。'))));
    return h('div', { class: 'tbl shop' }, h('table', {}, h('thead', {}, h('tr', {}, head.map((c) => {
      const [label, cls] = Array.isArray(c) ? c : [c, ''];
      return h('th', { class: cls || null, text: label });
    }))), h('tbody', {}, ...rows)));
  }

  const storeLabel = (s) => (s.map_key ? s.map_key.replace(/^level_\d+_?/, '') || s.map_key : s.store_id);
  // 205 家商店排成按钮墙看不清，这里按类别分组的下拉，一行就够。
  function pickerEl() {
    const groups = new Map();
    stores.slice().sort((a, b) => {
      const ka = storeKind(a.store_id); const kb = storeKind(b.store_id);
      return ka === kb ? a.store_id.localeCompare(b.store_id) : ka.localeCompare(kb);
    }).forEach((s) => {
      const k = storeKind(s.store_id);
      if (!groups.has(k)) groups.set(k, []);
      groups.get(k).push(s);
    });
    const sel = h('select', {
      id: 'storeSel', style: { width: '300px', maxWidth: '46vw' },
      onchange: (e) => navigate('shop', e.target.value + '/' + tabKey),
    }, ...[...groups.entries()].map(([k, list]) => h('optgroup', { label: `${k}（${list.length}）` },
      ...list.map((s) => h('option', { value: s.store_id, selected: s.store_id === storeId },
        `${storeLabel(s)} · ${s.item_count || 0} 件`)))));
    return h('div', { class: 'storebar', style: { display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap', margin: '0 0 14px' } },
      h('span', { class: 'lab', style: { fontSize: '12.4px', color: 'var(--muted)' }, text: '商店' }),
      sel,
      cur ? h('span', { class: 'hint', style: { margin: '0' }, text: `${storeKind(cur.store_id)} · ${storeLabel(cur)} · ${items.length} 件商品` }) : null,
      h('span', { style: { flex: '1 1 auto' } }),
      btn('刷新', () => reload(), 'ghost'));
  }

  function redraw() {
    const overrides = items.filter((x) => x.has_override).length;
    const isPrice = tabKey === 'price';
    body.replaceChildren(
      h2s(isPrice ? '货架价格' : '库存'),
      hint(isPrice
        ? '覆盖价留空 = 按基础价走；金币区商品只按金币计价。改完玩家刷新商店就看到。'
        : '覆盖库存留空 = 按默认库存走；填 0 表示这个店卖光了。', { margin: '0 0 10px' }),
      note('为什么看起来商品很少：', `物品目录里有 ${fmt(catalogTotal)} 件可上架物品，但服务端给每个水域商店只登记了 1–3 件（基础货架），所以这里每店只列得出几条。这是真服现状，不是没加载出来。`, { margin: '0 0 14px' }),
      brief([[stores.length, '商店'], [items.length, '本店商品'], [overrides, '本店覆盖'], [catalogTotal, '目录物品']]),
      pickerEl(),
      tableEl(),
      actions,
      storesBlock(),
    );
    introAside();
    syncActions();
  }

  function storeName(s) {
    const key = String(s.map_key || '').replace(/^level_\d+_?/, '').replace(/[_-]+/g, ' ').trim();
    return key || s.store_id;
  }

  // 205 家商店在右栏只能看前 8 家，放到主栏下面完整列出，能滚。
  function storesBlock() {
    const sorted = stores.slice().sort((a, b) => {
      const ka = storeKind(a.store_id); const kb = storeKind(b.store_id);
      return ka === kb ? a.store_id.localeCompare(b.store_id) : ka.localeCompare(kb);
    });
    const rows = sorted.map((s) => h('tr', { class: s.store_id === storeId ? 'on' : null, style: { cursor: 'pointer' }, onclick: () => navigate('shop', s.store_id + '/' + tabKey) },
      h('td', {}, h('b', { text: storeName(s) }), h('div', { class: 'mono', text: s.store_id })),
      h('td', { class: 'dim', text: storeKind(s.store_id) }),
      h('td', { class: 'num', text: String(s.item_count || 0) }),
      h('td', { class: 'num', text: s.override_count ? String(s.override_count) : '—' })));
    return h('div', {},
      h3s(`全部商店（${stores.length} 家）`),
      hint('点一行切到那家店。件数是这家的货架条目数，覆盖是你改过的条目数。', { margin: '0 0 8px' }),
      h('div', { class: 'tbl shops' }, h('table', {},
        h('thead', {}, h('tr', {}, h('th', { text: '商店' }), h('th', { text: '类别' }), h('th', { class: 'num', text: '件' }), h('th', { class: 'num', text: '覆盖' }))),
        h('tbody', {}, ...rows))));
  }

  function introAside() {
    const overrides = items.filter((x) => x.has_override).length;
    const effRows = items.map((x) => h('div', {},
      h('dt', { text: x.name || x.item_id }),
      h('dd', { text: money(x.silver) + ' 银' + (x.gold ? ' / ' + money(x.gold) + ' 金' : '') })));
    const kind = cur ? storeKind(cur.store_id) : '';
    const sameKind = stores.filter((x) => storeKind(x.store_id) === kind);
    const totalItems = sameKind.reduce((n, x) => n + (x.item_count || 0), 0);
    const totalOver = sameKind.reduce((n, x) => n + (x.override_count || 0), 0);
    const missing = sameKind.filter((x) => !x.item_count).length;
    aside.replaceChildren(
      ib('这家店', cur ? storeKind(cur.store_id) + (cur.map_key ? ' · ' + storeName(cur) : '') : '未选择商店',
        kv([['货架条目', String(items.length)], ['覆盖条目', String(overrides)], ['店 ID', storeId || '—']]),
        h3s('当前生效'),
        effRows.length ? h('dl', {}, ...effRows) : hint('这家店还没有商品。', { margin: '0' })),
      ib('同类商店', kind || null,
        kv([['店数', String(sameKind.length)], ['条目合计', String(totalItems)], ['覆盖合计', String(totalOver)], ['空货架', String(missing)]]),
        hint('「全部商店」的完整列表在主栏下面，能滚。', { margin: '10px 0 0' })),
      ib('为什么商品这么少', null,
        hint(`物品目录 ${fmt(catalogTotal)} 件，但每个店只登记 1–3 件基础货架。没登记的店在游戏里是空的。`, { margin: '0' })),
    );
  }

  async function reload() {
    try {
      items = await apiList('/api/stores/items?limit=1000&store_id=' + encodeURIComponent(storeId) + '&q=');
      setHeroNumbers([
        [stores.length, '商店'],
        [items.length, '本店商品'],
        [items.filter((x) => x.has_override).length, '覆盖'],
        [catalogTotal, '目录物品'],
      ]);
      redraw();
    } catch (e) {
      body.replaceChildren(h2s(tab.label), empty('读取失败', errText(e)));
    }
  }

  await reload();
  return [
    pageHead('商店与库存', '按商店改覆盖价与覆盖库存；没有覆盖的行按基础价走，随时可以恢复默认。',
      { facts: [['生效', '保存后约 30 秒'], ['作用范围', '当前商店'], ['要重登', '不需要'], ['上次保存', '—']] }),
    tabsEl,
    work(body, aside),
  ];
}
