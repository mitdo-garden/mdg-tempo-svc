#!/usr/bin/env python3
"""Rebuild mdg_tempo user_id=2 state from decoded mysqlbinlog and emit SQL."""
from pathlib import Path
import re
import sys

DECODED = Path(sys.argv[1] if len(sys.argv) > 1 else "/tmp/binlog_decoded.txt")
OUT = Path(sys.argv[2] if len(sys.argv) > 2 else "/tmp/restore_user2.sql")

text = DECODED.read_text()
lines = text.splitlines()

wipe_idx = None
for i, line in enumerate(lines):
    if "10:24:35" in line and "Table_map: `mdg_tempo`.`links`" in line:
        wipe_idx = i
        break
if wipe_idx is None:
    raise SystemExit("wipe transaction not found in decoded binlog")

wipe_end = None
for i in range(wipe_idx, len(lines)):
    if lines[i].startswith("#") and "Xid =" in lines[i]:
        wipe_end = i
        break
if wipe_end is None:
    raise SystemExit("wipe Xid not found")

print(f"wipe lines {wipe_idx}-{wipe_end}")


def parse_val(raw):
    raw = raw.strip()
    if raw == "NULL":
        return None
    if raw.startswith("'") and raw.endswith("'"):
        s = raw[1:-1]
        s = s.replace(r"\'", "'").replace(r"\\", "\\")
        return s
    try:
        if "." in raw or "e" in raw.lower():
            return float(raw)
        return int(raw)
    except ValueError:
        return raw


def parse_row_block(block_lines):
    vals = {}
    for line in block_lines:
        m = re.match(r"###   @(\d+)=(.*)$", line)
        if not m:
            continue
        vals[int(m.group(1))] = parse_val(m.group(2))
    if not vals:
        return None
    return [vals[i] for i in range(1, max(vals) + 1)]


COLS = {
    "tempos": [
        "id", "user_id", "type", "name", "parent_id", "content", "confidence", "due",
        "ease", "interval_days", "reminder", "pos_x", "pos_y", "created_at", "updated_at",
    ],
    "links": ["user_id", "a", "b", "created_at"],
    "review_history": ["id", "user_id", "tempo_id", "rating", "reviewed_at"],
    "activity_log": ["id", "user_id", "tempo_id", "channel", "status", "created_at"],
    "tempo_tags": ["tempo_id", "tag"],
    "examples": ["id", "tempo_id", "text", "sort_order", "created_at"],
    "media": ["id", "tempo_id", "kind", "url", "title", "sort_order", "created_at"],
}

restored = {t: [] for t in COLS}
i = wipe_idx
while i < wipe_end:
    line = lines[i]
    m = re.match(r"### DELETE FROM `mdg_tempo`\.`(\w+)`", line)
    if m:
        table = m.group(1)
        j = i + 1
        block = []
        while j < wipe_end and lines[j].startswith("###   @"):
            block.append(lines[j])
            j += 1
        row = parse_row_block(block)
        if row and table in restored:
            restored[table].append(row)
        i = j
        continue
    i += 1

print({k: len(v) for k, v in restored.items() if v})
tempo_ids = {r[0] for r in restored["tempos"]}


def apply_events(end_idx):
    state = {"tempo_tags": {}, "examples": {}, "media": {}}
    i = 0
    while i < end_idx:
        line = lines[i]
        m = re.match(r"### (INSERT INTO|UPDATE|DELETE FROM) `mdg_tempo`\.`(\w+)`", line)
        if not m:
            i += 1
            continue
        op, table = m.group(1), m.group(2)
        if table not in ("tempo_tags", "examples", "media"):
            i += 1
            continue
        j = i + 1
        if op == "UPDATE":
            where_block, set_block = [], []
            mode = None
            while j < end_idx:
                if lines[j].startswith("### WHERE"):
                    mode = "where"
                    j += 1
                    continue
                if lines[j].startswith("### SET"):
                    mode = "set"
                    j += 1
                    continue
                if lines[j].startswith("###   @"):
                    (where_block if mode == "where" else set_block).append(lines[j])
                    j += 1
                    continue
                break
            where_row = parse_row_block(where_block)
            set_row = parse_row_block(set_block)
            if table == "tempo_tags" and where_row and set_row:
                state["tempo_tags"][(where_row[0], where_row[1])] = set_row
            elif table == "examples" and where_row and set_row:
                state["examples"][where_row[0]] = set_row
            elif table == "media" and where_row and set_row:
                state["media"][where_row[0]] = set_row
            i = j
            continue

        if j < end_idx and (lines[j].startswith("### WHERE") or lines[j].startswith("### SET")):
            j += 1
        block = []
        while j < end_idx and lines[j].startswith("###   @"):
            block.append(lines[j])
            j += 1
        row = parse_row_block(block)
        if row:
            if op == "INSERT INTO":
                if table == "tempo_tags":
                    state["tempo_tags"][(row[0], row[1])] = row
                elif table == "examples":
                    state["examples"][row[0]] = row
                elif table == "media":
                    state["media"][row[0]] = row
            elif op == "DELETE FROM":
                if table == "tempo_tags":
                    state["tempo_tags"].pop((row[0], row[1]), None)
                elif table == "examples":
                    state["examples"].pop(row[0], None)
                elif table == "media":
                    state["media"].pop(row[0], None)
        i = j
    return state


child = apply_events(wipe_idx)
tags = [r for r in child["tempo_tags"].values() if r[0] in tempo_ids]
examples = [r for r in child["examples"].values() if r[1] in tempo_ids]
media = [r for r in child["media"].values() if r[1] in tempo_ids]
print("tags", len(tags), "examples", len(examples), "media", len(media))


def sql_quote(v):
    if v is None:
        return "NULL"
    if isinstance(v, float):
        return repr(v)
    if isinstance(v, int) and not isinstance(v, bool):
        return str(v)
    s = str(v).replace("\\", "\\\\").replace("'", "''")
    return "'" + s + "'"


def insert_sql(table, cols, rows):
    out = []
    for row in rows:
        vals = list(row) + [None] * max(0, len(cols) - len(row))
        vals = vals[: len(cols)]
        out.append(
            f"INSERT INTO {table} ({', '.join(cols)}) VALUES ({', '.join(sql_quote(v) for v in vals)});"
        )
    return out


sql = [
    "SET FOREIGN_KEY_CHECKS=0;",
    "START TRANSACTION;",
    "DELETE FROM links WHERE user_id=2;",
    "DELETE FROM review_history WHERE user_id=2;",
    "DELETE FROM activity_log WHERE user_id=2;",
    "DELETE FROM tempos WHERE user_id=2;",
]
sql += insert_sql("tempos", COLS["tempos"], restored["tempos"])
sql += insert_sql("tempo_tags", COLS["tempo_tags"], tags)
sql += insert_sql("examples", COLS["examples"], examples)
sql += insert_sql("media", COLS["media"], media)
sql += insert_sql("links", COLS["links"], restored["links"])
sql += insert_sql("review_history", COLS["review_history"], restored["review_history"])
sql += insert_sql("activity_log", COLS["activity_log"], restored["activity_log"])
sql += ["COMMIT;", "SET FOREIGN_KEY_CHECKS=1;"]

OUT.write_text("\n".join(sql) + "\n")
print("wrote", OUT, "statements", len(sql))
