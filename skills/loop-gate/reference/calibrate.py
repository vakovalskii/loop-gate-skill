#!/usr/bin/env python3
"""Калибровка порога loop-gate по готовым ответам.

Вход: jsonl, в каждой строке {"text": "..."} (ответ или рассуждение модели).
Выход: распределение двух метрик детектора, максимального числа повторов
одной N-граммы (top) и доли слов в повторах (ratio), и сколько строк
сработало бы при заданном пороге.

    python3 calibrate.py healthy.jsonl --ngram 32 --min-repeat 5 --cut 0.35
"""
import argparse
import collections
import json
import statistics


def score(text, n):
    w = text.split()
    counts = collections.Counter()
    rep = top = 0
    for i in range(len(w) - n + 1):
        k = " ".join(w[i:i + n])
        counts[k] += 1
        c = counts[k]
        rep += 2 if c == 2 else (1 if c > 2 else 0)
        top = max(top, c)
    grams = len(w) - n + 1
    return len(w), top, (rep / grams if grams > 0 else 0.0)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("path")
    ap.add_argument("--ngram", type=int, default=32)
    ap.add_argument("--arm", type=int, default=3000, help="слов до взвода детектора")
    ap.add_argument("--min-repeat", type=int, default=5)
    ap.add_argument("--cut", type=float, default=0.35)
    a = ap.parse_args()

    rows = []
    with open(a.path) as f:
        for line in f:
            if line.strip():
                rows.append(score(json.loads(line)["text"], a.ngram))
    armed = [r for r in rows if r[0] >= a.arm]
    if not armed:
        print("нет ответов длиннее", a.arm, "слов")
        return
    tops = [r[1] for r in armed]
    ratios = [r[2] for r in armed]
    print(f"строк {len(rows)}, длиннее {a.arm} слов {len(armed)}")
    print("top   медиана", statistics.median(tops), "max", max(tops))
    print("ratio медиана %.3f, p90 %.3f, max %.3f" % (
        statistics.median(ratios),
        statistics.quantiles(ratios, n=10)[-1] if len(ratios) > 1 else ratios[0],
        max(ratios)))
    fired = sum(1 for t, r in zip(tops, ratios) if t >= a.min_repeat and r >= a.cut)
    print(f"сработало бы при top>={a.min_repeat} и ratio>={a.cut}: {fired} из {len(armed)}")


if __name__ == "__main__":
    main()
