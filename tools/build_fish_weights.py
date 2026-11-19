#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""从 rf4info 周档生成每个水域的出鱼权重 SQL。

数据来源：https://github.com/ghoomt03-sketch/rf4info 的 public_data/weeks/*.sqlite3。
三张证据表按可信度加权：渔获 1.0、点位 0.5、官方周榜 1.0；每图以最大权重归一为 100，
最小钳到 1.0。名册（loc_wiki_fish）内但窗口无记录的鱼同样保留 1.0。

用法：
  python tools/build_fish_weights.py --weeks ./rf4info/public_data/weeks --db ./data/rf4server.db --out ./out
"""
import argparse
import collections
import glob
import os
import sqlite3

WEIGHTS = {
    "catch_records": 1.0,
    "community_spots": 0.5,
    "official_weekly_records": 1.0,
}


def load_roster(db_path):
    db = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    alias = {a: l for a, l in db.execute("SELECT alias, loc FROM loc_alias")}
    roster = collections.defaultdict(list)
    for loc, fish in db.execute("SELECT loc, fish_key FROM loc_wiki_fish"):
        roster[loc].append(fish)
    db.close()
    return alias, roster


def aggregate(week_paths):
    agg = collections.Counter()
    for path in week_paths:
        db = sqlite3.connect(path)
        for table, weight in WEIGHTS.items():
            try:
                rows = db.execute(
                    f"SELECT map_id, fish_id, COUNT(*) FROM {table} "
                    "WHERE fish_id IS NOT NULL AND fish_id <> '' "
                    "AND map_id IS NOT NULL AND map_id <> '' GROUP BY 1, 2"
                )
            except sqlite3.OperationalError:
                continue
            for map_id, fish_id, count in rows:
                agg[(map_id, fish_id)] += count * weight
        db.close()
    return agg


def build_sql(agg, alias, roster):
    lines = [
        "-- 18 个水域的鱼种出鱼权重（图内相对值，0-100）。",
        "-- 数据来源与时间跨度见 specs/changes/fish-weight-official/spec.md。",
        "-- 名册内但统计窗口无记录的鱼保留最低 1.0，不置 0。",
        "-- 整表重置：本迁移是该表默认内容的唯一真源，人工覆盖在此之后重新设置。",
        "DELETE FROM fish_weight_override;",
        "",
        "-- 只在名册里存在该鱼的水域写入：静态库版本较旧时名册可能少鱼，",
        "-- 直接写死会在这些库上留下「名册外的权重」，判鱼时会把该鱼塞进候选池。",
        "-- 用 VALUES 列表与 loc_wiki_fish 连接，等于按当前名册过滤一遍，可重复执行。",
        "WITH v(loc, fish_key, weight) AS (",
        "  VALUES",
    ]
    rows = []
    missing = 0
    for map_id in sorted(alias):
        loc = alias[map_id]
        per_map = {f: v for (m, f), v in agg.items() if m == map_id}
        peak = max(per_map.values()) if per_map else 1.0
        for fish in sorted(roster.get(loc, [])):
            if fish in per_map:
                weight = max(round(per_map[fish] / peak * 100, 2), 1.0)
            else:
                weight, missing = 1.0, missing + 1
            rows.append(f"('{loc}','{fish}',{weight})")
    lines.append(",\n  ".join(rows))
    lines += [
        ")",
        "INSERT INTO fish_weight_override(loc,fish_key,weight,enabled,updated_ts,updated_by)",
        "SELECT v.loc, v.fish_key, v.weight, 1, 1795000000, 'rf4info'",
        "  FROM v JOIN loc_wiki_fish w ON w.loc=v.loc AND w.fish_key=v.fish_key",
        " WHERE NOT EXISTS (",
        "   SELECT 1 FROM fish_weight_override o",
        "    WHERE o.loc=v.loc AND o.fish_key=v.fish_key",
        " );",
    ]
    return lines, len(rows), missing


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--weeks", required=True, help="rf4info public_data/weeks 目录")
    ap.add_argument("--db", required=True, help="本地 rf4server.db（只读）")
    ap.add_argument("--out", required=True, help="输出目录")
    args = ap.parse_args()

    weeks = sorted(glob.glob(os.path.join(args.weeks, "*.sqlite3")))
    if not weeks:
        raise SystemExit(f"未找到周档：{args.weeks}")
    alias, roster = load_roster(args.db)
    agg = aggregate(weeks)
    lines, total, missing = build_sql(agg, alias, roster)

    os.makedirs(args.out, exist_ok=True)
    target = os.path.join(args.out, "00071_fish_weight_official.sql")
    with open(target, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(lines) + "\n")
    # 基础权重表与覆盖表同源：迁移 00072 首次建立时把这份统计值原样抄过去，
    # 之后人工调权重只影响覆盖表，基线仍然是官方统计口径。
    base_lines = [
        "-- 官方统计基线（迁移 00072 由 fish_weight_override 首次快照生成，此处给出等价 SQL 便于重建）。",
        "INSERT OR REPLACE INTO fish_weight_base(loc,fish_key,weight,updated_ts,updated_by)",
        "SELECT loc,fish_key,weight,updated_ts,COALESCE(updated_by,'') FROM fish_weight_override;",
    ]
    base_target = os.path.join(args.out, "00072_fish_weight_base_seed.sql")
    with open(base_target, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(base_lines) + "\n")
    print(f"周档 {len(weeks)} 个，鱼×图 {len(agg)} 组，写出 {total} 行（无统计回退 1.0 的 {missing} 行）")
    print(f"输出：{target}、{base_target}")


if __name__ == "__main__":
    main()
