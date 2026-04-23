#!/usr/bin/env python3

import argparse
import json
import random
import ssl
import time
import urllib.parse
import urllib.request
from decimal import Decimal, InvalidOperation
from pathlib import Path


STORE_SOURCES = [
    {"domain": "colourpop.com", "accent": "#f9a8d4", "category": "beauty"},
    {"domain": "allbirds.com", "accent": "#86efac", "category": "footwear"},
    {"domain": "gymshark.com", "accent": "#93c5fd", "category": "activewear"},
    {"domain": "fashionnova.com", "accent": "#fca5a5", "category": "fashion"},
    {"domain": "kith.com", "accent": "#c4b5fd", "category": "fashion"},
    {"domain": "negativeunderwear.com", "accent": "#f5d0fe", "category": "apparel"},
    {"domain": "blendjet.com", "accent": "#67e8f9", "category": "appliances"},
    {"domain": "ridge.com", "accent": "#cbd5e1", "category": "accessories"},
    {"domain": "wandrd.com", "accent": "#fdba74", "category": "travel"},
    {"domain": "solgaard.co", "accent": "#fcd34d", "category": "travel"},
    {"domain": "stasherbag.com", "accent": "#a7f3d0", "category": "kitchen"},
    {"domain": "lttstore.com", "accent": "#fb7185", "category": "merch"},
    {"domain": "simplehuman.com", "accent": "#94a3b8", "category": "home"},
    {"domain": "byjasco.com", "accent": "#818cf8", "category": "electronics"},
    {"domain": "bremont.com", "accent": "#7dd3fc", "category": "watches"},
    {"domain": "misen.com", "accent": "#fbbf24", "category": "kitchen"},
    {"domain": "barebonesliving.com", "accent": "#d4d4d8", "category": "outdoors"},
    {"domain": "branchfurniture.com", "accent": "#a78bfa", "category": "furniture"},
    {"domain": "bludot.com", "accent": "#64748b", "category": "furniture"},
    {"domain": "floydhome.com", "accent": "#fb7185", "category": "furniture"},
    {"domain": "maidenhome.com", "accent": "#92400e", "category": "furniture"},
    {"domain": "masterdynamic.com", "accent": "#60a5fa", "category": "audio"},
    {"domain": "masterbuilt.com", "accent": "#f97316", "category": "outdoors"},
    {"domain": "mociun.com", "accent": "#f472b6", "category": "jewelry"},
    {"domain": "ringconcierge.com", "accent": "#e879f9", "category": "jewelry"},
    {"domain": "sundays-company.com", "accent": "#34d399", "category": "furniture"},
    {"domain": "rowingblazers.com", "accent": "#38bdf8", "category": "fashion"},
    {"domain": "therow.com", "accent": "#94a3b8", "category": "luxury-fashion"},
    {"domain": "fizik.com", "accent": "#fb7185", "category": "cycling"},
    {"domain": "shinola.com", "accent": "#f59e0b", "category": "lifestyle"},
    {"domain": "shop.hodinkee.com", "accent": "#0f172a", "category": "watches"},
    {"domain": "watchesofswitzerland.com", "accent": "#1d4ed8", "category": "watches"},
    {"domain": "wristaficionado.com", "accent": "#312e81", "category": "watches"},
    {"domain": "publicbikes.com", "accent": "#22c55e", "category": "bikes"},
    {"domain": "cwsellors.co.uk", "accent": "#6366f1", "category": "jewelry"},
    {"domain": "savoirbeds.com", "accent": "#b45309", "category": "beds"},
]

EXCLUDED_NAME_PARTS = (
    "gift card",
    "digital download",
    "carbon neutral order",
    "shipping protection",
    "package protection",
    "warranty handling fee",
    "extended warranty",
    "protection plan",
    "tips for the team",
    "donation",
    "raycon notes app",
)

TARGET_OVER_5000 = 1000
TARGET_OVER_1000_TOTAL = 3000
MAX_VARIANTS_PER_PRODUCT = 48


def fetch_json(url: str, context: ssl.SSLContext) -> dict:
    request = urllib.request.Request(
        url,
        headers={
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0 Safari/537.36",
            "Accept": "application/json",
        },
    )
    with urllib.request.urlopen(request, context=context, timeout=25) as response:
        return json.loads(response.read().decode("utf-8", errors="ignore"))


def first_image_url(product: dict) -> str:
    images = product.get("images") or []
    if images:
        src = (images[0].get("src") or "").strip()
        if src:
            return src
    image = product.get("image") or {}
    return (image.get("src") or "").strip()


def price_to_cents(raw: str) -> int:
    try:
        return int((Decimal(str(raw).strip()) * 100).quantize(Decimal("1")))
    except (InvalidOperation, ValueError):
        return 0


def base_product_ok(product: dict) -> bool:
    name = str(product.get("title") or "").strip()
    if not name:
        return False
    lower_name = name.lower()
    if lower_name.startswith("::"):
        return False
    return not any(part in lower_name for part in EXCLUDED_NAME_PARTS)


def choose_product_variants(product: dict) -> list[tuple[str, int, str]]:
    variants = []
    seen = set()
    for variant in product.get("variants") or []:
        variant_id = str(variant.get("id") or "").strip()
        if not variant_id:
            continue
        price_cents = price_to_cents(variant.get("price") or "")
        if price_cents <= 0:
            continue
        variant_title = str(variant.get("title") or "").strip()
        dedupe_key = (price_cents, variant_title.lower())
        if dedupe_key in seen:
            continue
        seen.add(dedupe_key)
        variants.append((variant_id, price_cents, variant_title))

    variants.sort(key=lambda item: (item[1], item[2].lower(), item[0]))
    if len(variants) <= MAX_VARIANTS_PER_PRODUCT:
        return variants

    selected = []
    if MAX_VARIANTS_PER_PRODUCT <= 1:
        return [variants[0]]
    for idx in range(MAX_VARIANTS_PER_PRODUCT):
        picked = variants[(idx * (len(variants) - 1)) // (MAX_VARIANTS_PER_PRODUCT - 1)]
        selected.append(picked)
    out = []
    used = set()
    for item in selected:
        if item[0] in used:
            continue
        used.add(item[0])
        out.append(item)
    return out


def iter_store_products(domain: str, accent: str, category: str, context: ssl.SSLContext):
    since_id = 0
    pages = 0
    while True:
        query = urllib.parse.urlencode({"limit": 250, "since_id": since_id})
        url = f"https://{domain}/products.json?{query}"
        data = fetch_json(url, context)
        products = data.get("products") or []
        if not products:
            break
        for product in products:
            if not base_product_ok(product):
                continue
            product_id = str(product.get("id") or "").strip()
            image_url = first_image_url(product)
            if not product_id or not image_url:
                continue
            title = str(product.get("title")).strip()
            for variant_id, price_cents, variant_title in choose_product_variants(product):
                name = title
                if variant_title and variant_title.lower() != "default title":
                    name = f"{title} - {variant_title}"
                lower_name = name.lower()
                if any(part in lower_name for part in EXCLUDED_NAME_PARTS):
                    continue
                yield {
                    "id": f"{domain}:{product_id}:{variant_id}",
                    "name": name,
                    "price_cents": price_cents,
                    "image_url": image_url,
                    "accent": accent,
                    "source": domain,
                    "category": category,
                }
        since_id = products[-1]["id"]
        pages += 1
        if pages >= 12:
            break
        time.sleep(0.04)


def collect_products() -> list[dict]:
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE

    items: list[dict] = []
    seen_ids: set[str] = set()
    for source in STORE_SOURCES:
        domain = source["domain"]
        accent = source["accent"]
        category = source["category"]
        try:
            for item in iter_store_products(domain, accent, category, context):
                if item["id"] in seen_ids:
                    continue
                seen_ids.add(item["id"])
                items.append(item)
        except Exception as exc:
            print(f"warning: {domain} failed: {exc}")
    return items


def band_for_price(price_cents: int) -> str:
    if price_cents >= 500000:
        return "over_5000"
    if price_cents >= 100000:
        return "over_1000"
    return "under_1000"


def take_round_robin(items: list[dict], count: int, seed: int) -> list[dict]:
    if count <= 0 or not items:
        return []

    groups: dict[str, list[dict]] = {}
    for item in items:
        key = f"{item.get('category', 'other')}::{item.get('source', 'unknown')}"
        groups.setdefault(key, []).append(item)

    rng = random.Random(seed)
    keys = list(groups.keys())
    rng.shuffle(keys)
    for key in keys:
        groups[key].sort(key=lambda item: (item["price_cents"], item["id"]))

    picked = []
    while len(picked) < count and keys:
        next_keys = []
        for key in keys:
            bucket = groups[key]
            if not bucket:
                continue
            picked.append(bucket.pop(0))
            if len(picked) >= count:
                break
            if bucket:
                next_keys.append(key)
        if len(picked) >= count:
            break
        keys = next_keys
        rng.shuffle(keys)
    return picked


def build_snapshot(max_items: int) -> list[dict]:
    items = collect_products()
    rng = random.Random(20260422)
    rng.shuffle(items)

    over_5000 = [item for item in items if band_for_price(item["price_cents"]) == "over_5000"]
    over_1000 = [item for item in items if band_for_price(item["price_cents"]) == "over_1000"]
    under_1000 = [item for item in items if band_for_price(item["price_cents"]) == "under_1000"]

    need_over_5000 = min(TARGET_OVER_5000, max_items)
    need_over_1000 = min(max(TARGET_OVER_1000_TOTAL - need_over_5000, 0), max_items - need_over_5000)
    need_under_1000 = max_items - need_over_5000 - need_over_1000

    if len(over_5000) < need_over_5000:
        raise SystemExit(f"not enough >=$5000 products: need {need_over_5000}, got {len(over_5000)}")
    if len(over_1000) < need_over_1000:
        raise SystemExit(f"not enough $1000-$4999.99 products: need {need_over_1000}, got {len(over_1000)}")
    if len(under_1000) < need_under_1000:
        raise SystemExit(f"not enough <$1000 products: need {need_under_1000}, got {len(under_1000)}")

    selected = []
    selected.extend(take_round_robin(over_5000, need_over_5000, 101))
    selected.extend(take_round_robin(over_1000, need_over_1000, 202))
    selected.extend(take_round_robin(under_1000, need_under_1000, 303))

    rng.shuffle(selected)
    return selected[:max_items]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--out",
        default="internal/games/priceisright/products_snapshot.json",
        help="output JSON path",
    )
    parser.add_argument("--max-items", type=int, default=10000)
    args = parser.parse_args()

    items = build_snapshot(args.max_items)
    if not items:
        raise SystemExit("no products collected")

    output_path = Path(args.out)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(items, indent=2), encoding="utf-8")
    print(f"wrote {len(items)} products to {output_path}")


if __name__ == "__main__":
    main()
