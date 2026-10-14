"""Trusted example checker. Candidate input is one read-only SQLite query."""

import csv
from pathlib import Path
import sqlite3
import sys


def authorize(action, table, column, database, trigger):
    if action == sqlite3.SQLITE_SELECT:
        return sqlite3.SQLITE_OK
    if action == sqlite3.SQLITE_READ and table == "input_orders" and database == "main":
        return sqlite3.SQLITE_OK
    if action == sqlite3.SQLITE_FUNCTION and column in {
        "abs", "avg", "coalesce", "count", "ifnull", "length", "lower",
        "max", "min", "nullif", "round", "sum", "upper",
    }:
        return sqlite3.SQLITE_OK
    return sqlite3.SQLITE_DENY


def check():
    root = Path(__file__).resolve().parent.parent
    with (root / "checks/orders.csv").open(newline="", encoding="utf-8") as source:
        inputs = [(int(row["order_id"]), row["customer_id"]) for row in csv.DictReader(source)]
    sql = (root / "transform.sql").read_text(encoding="utf-8")
    if len(sql) > 16384:
        raise ValueError("query exceeds this fixture's 16 KiB limit")
    with sqlite3.connect(":memory:") as connection:
        connection.execute("CREATE TABLE input_orders (order_id INTEGER, customer_id TEXT)")
        connection.executemany("INSERT INTO input_orders VALUES (?, ?)", inputs)
        connection.commit()
        # Only the approved checker is Python. Candidate SQL cannot write, attach
        # databases, load extensions or run arbitrary Python through this checker.
        connection.set_authorizer(authorize)
        steps = 0

        def stop_long_query():
            nonlocal steps
            steps += 1
            return steps > 100

        connection.set_progress_handler(stop_long_query, 1000)
        cursor = connection.execute(sql)
        if [column[0] for column in cursor.description or []] != ["order_id", "customer_id"]:
            raise ValueError("query must return columns order_id, customer_id")
        rows = cursor.fetchmany(10001)
        if len(rows) > 10000:
            raise ValueError("result exceeds this fixture's 10,000-row limit")
    keys = [row[0] for row in rows]
    nulls = sum(key is None for key in keys)
    duplicates = len(keys) - len(set(keys))
    if not rows or nulls or duplicates:
        print(f"FAIL: rows={len(rows)}, null order_id={nulls}, duplicate order_id={duplicates}")
        return 1
    print(f"PASS: {len(rows)} rows have unique, non-null order_id values")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(check())
    except Exception as error:
        # A checker/runtime failure is incomplete verification, never exit 1.
        print(f"UNVERIFIED: {error}", file=sys.stderr)
        sys.exit(2)
