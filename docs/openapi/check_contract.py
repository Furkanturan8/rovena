"""Checks docs/openapi/*.yaml against the running stack (docker compose up).

For every call it verifies that the status code is documented, the JSON body matches the
documented schema, and no undocumented fields appear. It creates rows whose names start with
"speccheck" (Inventory: SKUs starting with "SPC-", Payment: idempotency keys starting with "speccheck-") and removes
them at the end (psql inside the postgres container).

    python3 -m venv .venv && .venv/bin/pip install openapi-spec-validator jsonschema pyyaml requests
    .venv/bin/python docs/openapi/check_contract.py

Uses INTERNAL_API_KEY (environment variable, else .env) for the Inventory and Payment /internal endpoints (service ports
8083 and 8085). It must be the key the running containers were started with.
Payments are created the way production does it: a PaymentRequested event is published to RabbitMQ (through its
management API on port 15672) and the running Payment service handles it.
Uses ~12 of the 20 requests/minute that /auth/* allows per IP, so do not run it twice within a minute.
"""
import atexit, json, os, sys, time, subprocess, uuid, yaml, requests
from openapi_schema_validator import OAS30Validator

ROOT = str(__import__("pathlib").Path(__file__).resolve().parents[2])
G = "http://localhost:8080"
INTERNAL = "http://localhost:8083"  # inventory /internal/* is not routed by the gateway
PAYMENT = "http://localhost:8085"   # payment /internal/* neither
INTERNALS = {"inventory": INTERNAL, "payment": PAYMENT}
specs = {n: yaml.safe_load(open(f"{ROOT}/docs/openapi/{n}.yaml")) for n in ("identity", "catalog", "inventory", "payment")}
problems, seen = [], set()

def resolve(doc, node):
    while isinstance(node, dict) and "$ref" in node:
        cur = doc
        for part in node["$ref"][2:].split("/"):
            cur = cur[part]
        node = cur
    return node

def strict_keys(doc, schema, value, path="$"):
    """Report response keys that the schema does not declare."""
    schema = resolve(doc, schema)
    if isinstance(value, dict) and "properties" in schema and "additionalProperties" not in schema:
        for k, v in value.items():
            if k not in schema["properties"]:
                problems.append(f"undocumented field {path}.{k}")
            else:
                strict_keys(doc, schema["properties"][k], v, f"{path}.{k}")
    elif isinstance(value, dict) and isinstance(schema.get("additionalProperties"), dict):
        for k, v in value.items():
            strict_keys(doc, schema["additionalProperties"], v, f"{path}.{k}")
    elif isinstance(value, list) and "items" in schema:
        for i, v in enumerate(value):
            strict_keys(doc, schema["items"], v, f"{path}[{i}]")

def check(api, method, template, resp, note=""):
    doc = specs[api]
    label = f"{api}: {method} {template} -> {resp.status_code} {note}".strip()
    seen.add((api, method.lower(), template, str(resp.status_code)))
    op = doc["paths"].get(template, {}).get(method.lower())
    if op is None:
        problems.append(f"{label}: operation is not in the spec"); return
    code = str(resp.status_code)
    if code not in op["responses"]:
        problems.append(f"{label}: status not documented (documented: {sorted(op['responses'])}) body={resp.text[:150]}"); return
    r = resolve(doc, op["responses"][code])
    content = r.get("content", {}).get("application/json")
    if not content:
        if resp.content:
            problems.append(f"{label}: spec says no body but got {resp.text[:100]}")
        return
    body = resp.json()
    schema = dict(resolve(doc, content["schema"]), components=doc["components"])
    for err in OAS30Validator(schema).iter_errors(body):
        problems.append(f"{label}: schema violation at {list(err.absolute_path)}: {err.message[:120]}")
    strict_keys(doc, content["schema"], body)
    for h, hd in r.get("headers", {}).items():
        want = resolve(doc, hd)["schema"].get("example")
        if want and resp.headers.get(h) != want:
            problems.append(f"{label}: header {h} = {resp.headers.get(h)!r}, spec says {want!r}")

def call(api, method, template, path=None, token=None, json_body=None, raw=None, note="", base=G, key=None):
    headers = {}
    if token: headers["Authorization"] = f"Bearer {token}"
    if key: headers["X-Internal-Key"] = key
    if raw is not None:
        headers["Content-Type"] = "application/json"
    time.sleep(0.06)  # stay under the gateway's 30 req/s limit
    resp = requests.request(method, base + (path or template), headers=headers,
                            json=json_body, data=raw)
    check(api, method, template, resp, note)
    return resp

# ---- cleanup of everything this script creates; runs even when the script crashes
def psql(db, sql):
    subprocess.run(["docker", "compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", db, "-q", "-c", sql],
                   cwd=ROOT, check=True, capture_output=True)

def cleanup():
    psql("catalog", "DELETE FROM products WHERE name LIKE 'speccheck%'; DELETE FROM categories WHERE name LIKE 'speccheck%' AND parent_id IS NOT NULL; DELETE FROM categories WHERE name LIKE 'speccheck%';")
    psql("identity", "DELETE FROM users WHERE email LIKE 'speccheck-%'")
    psql("inventory", """
        DELETE FROM stock_movements WHERE variant_id IN (SELECT variant_id FROM stock_items WHERE sku LIKE 'SPC-%');
        DELETE FROM reservations WHERE id IN (SELECT reservation_id FROM reservation_items WHERE variant_id IN (SELECT variant_id FROM stock_items WHERE sku LIKE 'SPC-%'));
        DELETE FROM stock_items WHERE sku LIKE 'SPC-%';""")
    psql("payment", """
        DELETE FROM outbox_events WHERE envelope->>'correlation_id' LIKE 'speccheck-%';
        DELETE FROM payments WHERE idempotency_key LIKE 'speccheck-%';""")

atexit.register(cleanup)

# ---- static: every example in the spec must satisfy its own schema
for api, doc in specs.items():
    def examples(n, where=""):
        if isinstance(n, dict):
            if "schema" in n and "example" in n:
                s = dict(resolve(doc, n["schema"]), components=doc["components"])
                for err in OAS30Validator(s).iter_errors(n["example"]):
                    problems.append(f"{api}: example at {where} violates its schema: {err.message[:100]}")
            for k, v in n.items(): examples(v, f"{where}/{k}")
        elif isinstance(n, list):
            for i, v in enumerate(n): examples(v, f"{where}[{i}]")
    examples(doc)

# ================================================================= identity
tag = uuid.uuid4().hex[:8]
email = f"speccheck-{tag}@example.com"
pw = "s3cretpass1"
I = "identity"

r = call(I, "POST", "/api/v1/auth/register", json_body={"email": email, "password": pw, "first_name": "Spec", "last_name": "Check"})
user_id = r.json()["data"]["user"]["id"]; access = r.json()["data"]["tokens"]["access_token"]; refresh = r.json()["data"]["tokens"]["refresh_token"]
call(I, "POST", "/api/v1/auth/register", json_body={"email": email, "password": pw})                     # 409
call(I, "POST", "/api/v1/auth/register", json_body={"email": "x", "password": "y"})                       # 422
call(I, "POST", "/api/v1/auth/register", raw="{")                                                          # 400
call(I, "POST", "/api/v1/auth/login", json_body={"email": email, "password": pw})                         # 200
call(I, "POST", "/api/v1/auth/login", json_body={"email": email, "password": "nope-nope"})                # 401
call(I, "POST", "/api/v1/auth/login", json_body={"email": email})                                         # 422
rr = call(I, "POST", "/api/v1/auth/refresh", json_body={"refresh_token": refresh})                        # 200
new_access = rr.json()["data"]["access_token"]; new_refresh = rr.json()["data"]["refresh_token"]
call(I, "POST", "/api/v1/auth/refresh", json_body={"refresh_token": refresh}, note="(reuse)")             # 401
call(I, "POST", "/api/v1/auth/logout", json_body={"refresh_token": new_refresh})                          # 204
call(I, "POST", "/api/v1/auth/logout", json_body={})                                                       # 422

me_tok = access
call(I, "GET", "/api/v1/users/me", token=me_tok)                                                           # 200
call(I, "GET", "/api/v1/users/me")                                                                         # 401
call(I, "PATCH", "/api/v1/users/me", token=me_tok, json_body={"first_name": "Spec2"})                      # 200
call(I, "PATCH", "/api/v1/users/me", token=me_tok, json_body={"first_name": "x" * 101})                    # 422
call(I, "PATCH", "/api/v1/users/me", token=me_tok, raw="{")                                                # 400
call(I, "GET", "/api/v1/users", token=me_tok)                                                              # 403
call(I, "GET", "/api/v1/users/{id}", path=f"/api/v1/users/{user_id}", token=me_tok)                        # 403

env = dict(l.split("=", 1) for l in open(f"{ROOT}/.env").read().splitlines() if "=" in l and not l.startswith("#"))
al = call(I, "POST", "/api/v1/auth/login", json_body={"email": env["ADMIN_EMAIL"], "password": env["ADMIN_PASSWORD"]})
admin = al.json()["data"]["tokens"]["access_token"]
call(I, "GET", "/api/v1/users", path="/api/v1/users?page=1&page_size=2", token=admin)                     # 200
call(I, "GET", "/api/v1/users/{id}", path=f"/api/v1/users/{user_id}", token=admin)                         # 200
call(I, "GET", "/api/v1/users/{id}", path=f"/api/v1/users/{uuid.uuid4()}", token=admin)                    # 404
call(I, "GET", "/api/v1/users/{id}", path="/api/v1/users/not-a-uuid", token=admin)                         # 400
call(I, "PATCH", "/api/v1/users/{id}/role", path=f"/api/v1/users/{user_id}/role", token=admin, json_body={"role": "admin"})     # 200
call(I, "PATCH", "/api/v1/users/{id}/role", path=f"/api/v1/users/{user_id}/role", token=admin, json_body={"role": "customer"})  # 200
call(I, "PATCH", "/api/v1/users/{id}/role", path=f"/api/v1/users/{user_id}/role", token=admin, json_body={"role": "boss"})      # 422
call(I, "PATCH", "/api/v1/users/{id}/role", path=f"/api/v1/users/{uuid.uuid4()}/role", token=admin, json_body={"role": "admin"}) # 404
admin_id = requests.get(G + "/api/v1/users/me", headers={"Authorization": f"Bearer {admin}"}).json()["data"]["id"]
n_admins = sum(1 for u in requests.get(G + "/api/v1/users?page_size=100", headers={"Authorization": f"Bearer {admin}"}).json()["data"]["items"] if u["role"] == "admin")
if n_admins == 1:
    call(I, "PATCH", "/api/v1/users/{id}/role", path=f"/api/v1/users/{admin_id}/role", token=admin, json_body={"role": "customer"})  # 409
for hp in ("healthz", "readyz"):
    resp = requests.get(f"http://localhost:8081/{hp}"); check(I, "GET", f"/{hp}", resp)

# ================================================================= catalog
C = "catalog"
mark = f"speccheck {tag}"
call(C, "GET", "/api/v1/categories")
cats = requests.get(G + "/api/v1/categories").json()["data"]
slug = cats[0]["slug"]
call(C, "GET", "/api/v1/categories/{slug}", path=f"/api/v1/categories/{slug}")
call(C, "GET", "/api/v1/categories/{slug}", path="/api/v1/categories/does-not-exist")
for q in ("", "?q=canta", "?category=giyim&sort=price_asc", "?size=M&color=Siyah&min_price=1&max_price=999999999", "?page=2&page_size=1", "?page=99"):
    call(C, "GET", "/api/v1/products", path="/api/v1/products" + q, note=q)
call(C, "GET", "/api/v1/products", path="/api/v1/products?sort=evil")
call(C, "GET", "/api/v1/products", path="/api/v1/products?min_price=abc")
first = requests.get(G + "/api/v1/products").json()["data"]["items"][0]
call(C, "GET", "/api/v1/products/{idOrSlug}", path=f"/api/v1/products/{first['slug']}")
call(C, "GET", "/api/v1/products/{idOrSlug}", path=f"/api/v1/products/{first['id']}")
call(C, "GET", "/api/v1/products/{idOrSlug}", path="/api/v1/products/does-not-exist")

# admin: category
call(C, "POST", "/api/v1/admin/categories", json_body={"name": mark})                                      # 401
call(C, "POST", "/api/v1/admin/categories", token=me_tok, json_body={"name": mark})                        # 403
cr = call(C, "POST", "/api/v1/admin/categories", token=admin, json_body={"name": mark, "description": "tmp"})
cat_id = cr.json()["data"]["id"]
sub = call(C, "POST", "/api/v1/admin/categories", token=admin, json_body={"name": mark + " sub", "parent_id": cat_id})
sub_id = sub.json()["data"]["id"]
call(C, "POST", "/api/v1/admin/categories", token=admin, json_body={"name": "x"})                          # 422
call(C, "POST", "/api/v1/admin/categories", token=admin, json_body={"name": mark, "parent_id": str(uuid.uuid4())})  # 404
call(C, "POST", "/api/v1/admin/categories", token=admin, raw="{")                                          # 400
call(C, "PATCH", "/api/v1/admin/categories/{id}", path=f"/api/v1/admin/categories/{cat_id}", token=admin, json_body={"description": "tmp2"})
call(C, "PATCH", "/api/v1/admin/categories/{id}", path=f"/api/v1/admin/categories/{uuid.uuid4()}", token=admin, json_body={"description": "x"})
call(C, "PATCH", "/api/v1/admin/categories/{id}", path="/api/v1/admin/categories/nope", token=admin, json_body={})
call(C, "DELETE", "/api/v1/admin/categories/{id}", path=f"/api/v1/admin/categories/{cat_id}", token=admin, note="(has subcategory)")  # 409

# admin: product
body = {"name": mark + " product", "brand": "Spec", "category_id": sub_id,
        "images": [{"url": "https://img.test/a.jpg", "alt": "a"}],
        "variants": [{"sku": f"spc-{tag}-m", "size": "M", "color": "Mavi", "price_cents": 12345},
                     {"sku": f"spc-{tag}-l", "size": "L", "color": "Mavi", "price_cents": 12345}]}
call(C, "POST", "/api/v1/admin/products", json_body=body)                                                  # 401
pr = call(C, "POST", "/api/v1/admin/products", token=admin, json_body=body)                                # 201
pid = pr.json()["data"]["id"]; v1, v2 = [v["id"] for v in pr.json()["data"]["variants"]]
call(C, "POST", "/api/v1/admin/products", token=admin, json_body=body, note="(duplicate sku)")             # 409
call(C, "POST", "/api/v1/admin/products", token=admin, json_body={"name": "x", "category_id": "nope", "images": [{"url": "javascript:1"}]})  # 422
call(C, "POST", "/api/v1/admin/products", token=admin, json_body={"name": mark + " ghost", "category_id": str(uuid.uuid4())})                # 404
call(C, "POST", "/api/v1/admin/products", token=admin, raw="{")                                            # 400
call(C, "GET", "/api/v1/admin/products", path="/api/v1/admin/products?status=draft", token=admin)
call(C, "GET", "/api/v1/admin/products", path="/api/v1/admin/products?status=bogus", token=admin)
call(C, "GET", "/api/v1/admin/products", token=me_tok)                                                     # 403
call(C, "GET", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{pid}", token=admin)
call(C, "GET", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{uuid.uuid4()}", token=admin)
call(C, "GET", "/api/v1/admin/products/{id}", path="/api/v1/admin/products/nope", token=admin)
call(C, "PATCH", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{pid}", token=admin, json_body={"status": "active", "images": []})
call(C, "PATCH", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{pid}", token=admin, json_body={"name": "x"})   # 422
call(C, "PATCH", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{uuid.uuid4()}", token=admin, json_body={"brand": "x"})  # 404
empty = call(C, "POST", "/api/v1/admin/products", token=admin, json_body={"name": mark + " empty", "category_id": sub_id}).json()["data"]["id"]
call(C, "PATCH", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{empty}", token=admin, json_body={"status": "active"})       # 409
# variants
call(C, "POST", "/api/v1/admin/products/{id}/variants", path=f"/api/v1/admin/products/{pid}/variants", token=admin, json_body={"sku": f"spc-{tag}-xl", "size": "XL", "price_cents": 999})   # 201
call(C, "POST", "/api/v1/admin/products/{id}/variants", path=f"/api/v1/admin/products/{pid}/variants", token=admin, json_body={"sku": f"spc-{tag}-xl", "size": "XXL", "price_cents": 999}) # 409
call(C, "POST", "/api/v1/admin/products/{id}/variants", path=f"/api/v1/admin/products/{pid}/variants", token=admin, json_body={"sku": "bad sku!", "price_cents": 5})                      # 422
call(C, "POST", "/api/v1/admin/products/{id}/variants", path=f"/api/v1/admin/products/{uuid.uuid4()}/variants", token=admin, json_body={"sku": "ZZ-9", "price_cents": 5})                  # 404
call(C, "PATCH", "/api/v1/admin/products/{id}/variants/{variantId}", path=f"/api/v1/admin/products/{pid}/variants/{v1}", token=admin, json_body={"price_cents": 20000})                   # 200
call(C, "PATCH", "/api/v1/admin/products/{id}/variants/{variantId}", path=f"/api/v1/admin/products/{pid}/variants/{v1}", token=admin, json_body={"price_cents": 0})                       # 422
call(C, "PATCH", "/api/v1/admin/products/{id}/variants/{variantId}", path=f"/api/v1/admin/products/{pid}/variants/{uuid.uuid4()}", token=admin, json_body={"active": True})               # 404
for vid in (v1, v2):
    pass
# switch off everything but one variant, then the last one -> 409
allv = requests.get(G + f"/api/v1/admin/products/{pid}", headers={"Authorization": f"Bearer {admin}"}).json()["data"]["variants"]
for v in allv[:-1]:
    call(C, "PATCH", "/api/v1/admin/products/{id}/variants/{variantId}", path=f"/api/v1/admin/products/{pid}/variants/{v['id']}", token=admin, json_body={"active": False})
call(C, "PATCH", "/api/v1/admin/products/{id}/variants/{variantId}", path=f"/api/v1/admin/products/{pid}/variants/{allv[-1]['id']}", token=admin, json_body={"active": False}, note="(last)")  # 409
call(C, "DELETE", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{pid}", token=admin)        # 204
call(C, "DELETE", "/api/v1/admin/products/{id}", path=f"/api/v1/admin/products/{uuid.uuid4()}", token=admin)
call(C, "DELETE", "/api/v1/admin/products/{id}", path="/api/v1/admin/products/nope", token=admin)
call(C, "DELETE", "/api/v1/admin/categories/{id}", path=f"/api/v1/admin/categories/{uuid.uuid4()}", token=admin)  # 404
for hp in ("healthz", "readyz"):
    resp = requests.get(f"http://localhost:8082/{hp}"); check(C, "GET", f"/{hp}", resp)


# ================================================================= inventory
N = "inventory"
KEY = os.environ.get("INTERNAL_API_KEY") or env["INTERNAL_API_KEY"]  # env var wins: the running container may use another key than .env
vA, vB, vGhost = str(uuid.uuid4()), str(uuid.uuid4()), str(uuid.uuid4())
skuA, skuB = f"SPC-{tag.upper()}-A", f"SPC-{tag.upper()}-B"
S = "/api/v1/admin/stock"

call(N, "POST", S, token=me_tok, json_body={"variant_id": vA, "sku": skuA})                                         # 403
call(N, "POST", S, token=admin, json_body={"variant_id": vA, "sku": skuA, "on_hand": 5, "reason": "speccheck"})    # 201
call(N, "POST", S, token=admin, json_body={"variant_id": vA, "sku": skuA, "on_hand": 5})                           # 409 variant tracked
call(N, "POST", S, token=admin, json_body={"variant_id": str(uuid.uuid4()), "sku": skuA})                          # 409 sku taken
call(N, "POST", S, token=admin, json_body={"variant_id": vB, "sku": skuB})                                         # 201, on_hand 0
call(N, "POST", S, token=admin, json_body={"variant_id": "x", "sku": "a", "on_hand": -1})                          # 422
call(N, "POST", S, token=admin, raw="{")                                                                           # 400
call(N, "GET", S + "/{variantId}", path=f"{S}/{vA}", token=admin)                                                  # 200
call(N, "GET", S + "/{variantId}", path=f"{S}/{uuid.uuid4()}", token=admin)                                        # 404
call(N, "GET", S + "/{variantId}", path=f"{S}/nope", token=admin)                                                  # 400
call(N, "POST", S + "/{variantId}/receipts", path=f"{S}/{vA}/receipts", token=admin, json_body={"quantity": 25, "reason": "speccheck delivery"})  # 200
call(N, "POST", S + "/{variantId}/receipts", path=f"{S}/{vA}/receipts", token=admin, json_body={"quantity": 0})    # 422
call(N, "POST", S + "/{variantId}/receipts", path=f"{S}/{uuid.uuid4()}/receipts", token=admin, json_body={"quantity": 1})  # 404
call(N, "POST", S + "/{variantId}/receipts", path=f"{S}/nope/receipts", token=admin, json_body={"quantity": 1})    # 400
call(N, "POST", S + "/{variantId}/adjustments", path=f"{S}/{vA}/adjustments", token=admin, json_body={"delta": -2, "reason": "damaged"})  # 200
call(N, "POST", S + "/{variantId}/adjustments", path=f"{S}/{vA}/adjustments", token=admin, json_body={"delta": -2})  # 422 reason
call(N, "POST", S + "/{variantId}/adjustments", path=f"{S}/{vA}/adjustments", token=admin, json_body={"delta": 0, "reason": "x"})  # 422
call(N, "POST", S + "/{variantId}/adjustments", path=f"{S}/{uuid.uuid4()}/adjustments", token=admin, json_body={"delta": 1, "reason": "x"})  # 404
for q in (f"?sku={skuA.lower()}", "?below=10&sort=available_asc", "?sort=sku&page_size=1", "?page=999"):
    call(N, "GET", S, path=S + q, token=admin, note=q)
call(N, "GET", S, path=S + "?sort=evil", token=admin)                                                              # 400
call(N, "GET", S, path=S + "?below=0", token=admin)                                                                # 400
call(N, "GET", S + "/{variantId}/movements", path=f"{S}/{vA}/movements", token=admin)                              # 200
call(N, "GET", S + "/{variantId}/movements", path=f"{S}/{vA}/movements?page_size=1", token=admin)
call(N, "GET", S + "/{variantId}/movements", path=f"{S}/{uuid.uuid4()}/movements", token=admin)                    # 404
call(N, "GET", "/api/v1/stock", path=f"/api/v1/stock?variant_ids={vA},{vB},{vGhost},{vA}")                         # 200, unknown -> 0
call(N, "GET", "/api/v1/stock", path="/api/v1/stock")                                                              # 400
call(N, "GET", "/api/v1/stock", path="/api/v1/stock?variant_ids=nope")                                             # 400

R = "/api/v1/internal/reservations"
order1, order2, order3, order4 = (str(uuid.uuid4()) for _ in range(4))
body = {"order_id": order1, "items": [{"variant_id": vA, "quantity": 3}]}
call(N, "POST", R, base=INTERNAL, json_body=body, note="(no key)")                                                 # 401
call(N, "POST", R, base=INTERNAL, json_body=body, key="wrong-key", note="(wrong key)")                             # 401
r1 = call(N, "POST", R, base=INTERNAL, json_body=body, key=KEY)                                                    # 201
res1 = r1.json()["data"]["id"]
call(N, "POST", R, base=INTERNAL, json_body=body, key=KEY, note="(retry, idempotent)")                             # 200
call(N, "POST", R, base=INTERNAL, key=KEY, json_body={"order_id": order2, "items": [{"variant_id": vA, "quantity": 1000}]})   # 409
call(N, "POST", R, base=INTERNAL, key=KEY, json_body={"order_id": order3, "items": [{"variant_id": vGhost, "quantity": 1}]})  # 404
call(N, "POST", R, base=INTERNAL, key=KEY, json_body={"order_id": order3, "items": [{"variant_id": vA, "quantity": 1}, {"variant_id": vA, "quantity": 1}]})  # 422 duplicate
call(N, "POST", R, base=INTERNAL, key=KEY, json_body={"order_id": order3, "items": []})                            # 422
call(N, "POST", R, base=INTERNAL, key=KEY, raw="{")                                                                # 400
call(N, "GET", R + "/{id}", path=f"{R}/{res1}", base=INTERNAL, key=KEY)                                            # 200
call(N, "GET", R + "/{id}", path=f"{R}/{uuid.uuid4()}", base=INTERNAL, key=KEY)                                    # 404
call(N, "GET", R + "/{id}", path=f"{R}/nope", base=INTERNAL, key=KEY)                                              # 400
call(N, "GET", R + "/by-order/{orderId}", path=f"{R}/by-order/{order1}", base=INTERNAL, key=KEY)                   # 200
call(N, "GET", R + "/by-order/{orderId}", path=f"{R}/by-order/{uuid.uuid4()}", base=INTERNAL, key=KEY)             # 404
call(N, "GET", R + "/by-order/{orderId}", path=f"{R}/by-order/nope", base=INTERNAL, key=KEY)                       # 400
# on_hand 28, reserved 3: removing 26 units would leave 2 on hand, below the 3 reserved
call(N, "POST", S + "/{variantId}/adjustments", path=f"{S}/{vA}/adjustments", token=admin, json_body={"delta": -26, "reason": "too much"})  # 409
call(N, "POST", R + "/{id}/commit", path=f"{R}/{res1}/commit", base=INTERNAL, key=KEY)                             # 200
call(N, "POST", R + "/{id}/commit", path=f"{R}/{res1}/commit", base=INTERNAL, key=KEY, note="(again)")             # 200
call(N, "POST", R + "/{id}/commit", path=f"{R}/{uuid.uuid4()}/commit", base=INTERNAL, key=KEY)                     # 404
call(N, "POST", R + "/{id}/commit", path=f"{R}/nope/commit", base=INTERNAL, key=KEY)                               # 400
call(N, "POST", R + "/{id}/release", path=f"{R}/{res1}/release", base=INTERNAL, key=KEY, note="(committed)")       # 409
r4 = call(N, "POST", R, base=INTERNAL, key=KEY, json_body={"order_id": order4, "items": [{"variant_id": vA, "quantity": 2}]})
res4 = r4.json()["data"]["id"]
call(N, "POST", R + "/{id}/release", path=f"{R}/{res4}/release", base=INTERNAL, key=KEY)                           # 200
call(N, "POST", R + "/{id}/release", path=f"{R}/{res4}/release", base=INTERNAL, key=KEY, note="(again)")           # 200
call(N, "POST", R + "/{id}/commit", path=f"{R}/{res4}/commit", base=INTERNAL, key=KEY, note="(released)")          # 409
call(N, "POST", R + "/{id}/release", path=f"{R}/{uuid.uuid4()}/release", base=INTERNAL, key=KEY)                   # 404
call(N, "POST", R + "/{id}/release", path=f"{R}/nope/release", base=INTERNAL, key=KEY)                             # 400
for hp in ("healthz", "readyz"):
    resp = requests.get(f"{INTERNAL}/{hp}"); check(N, "GET", f"/{hp}", resp)

# ================================================================= payment
P = "payment"
RABBIT = "http://localhost:15672/api/exchanges/%2F/rovena.events/publish"
RABBIT_AUTH = (env["RABBITMQ_USER"], env["RABBITMQ_PASSWORD"])

def request_payment(order_id, customer_id, method, amount=459900):
    """Publish PaymentRequested the way the Order service will; the running Payment service picks it up."""
    envelope = {"event_id": str(uuid.uuid4()), "type": "PaymentRequested", "version": 1,
                "occurred_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "correlation_id": f"speccheck-{tag}",
                "payload": {"order_id": order_id, "customer_id": customer_id, "idempotency_key": f"speccheck-{order_id}",
                            "amount_cents": amount, "currency": "TRY", "payment_method": method}}
    r = requests.post(RABBIT, auth=RABBIT_AUTH, json={
        "properties": {"delivery_mode": 2, "content_type": "application/json"}, "routing_key": "PaymentRequested",
        "payload": json.dumps(envelope), "payload_encoding": "string"})
    if not r.ok or not r.json().get("routed"):
        sys.exit(f"could not publish PaymentRequested: {r.status_code} {r.text}")

def wait_for_payment(order_id):
    for _ in range(100):
        r = requests.get(f"{PAYMENT}/api/v1/internal/payments/by-order/{order_id}", headers={"X-Internal-Key": KEY})
        if r.status_code == 200 and r.json()["data"]["status"] != "pending":
            return r.json()["data"]
        time.sleep(0.1)
    sys.exit(f"payment for order {order_id} did not finish in time; is the payment service running?")

paid_order, declined_order, foreign_order = (str(uuid.uuid4()) for _ in range(3))
request_payment(paid_order, user_id, "tok_visa")
request_payment(declined_order, user_id, "tok_declined")
request_payment(foreign_order, str(uuid.uuid4()), "tok_visa")
paid = wait_for_payment(paid_order); declined = wait_for_payment(declined_order); wait_for_payment(foreign_order)

B = "/api/v1/payments/by-order/{orderId}"
call(P, "GET", B, path=f"/api/v1/payments/by-order/{paid_order}", token=me_tok)                                    # 200 succeeded
call(P, "GET", B, path=f"/api/v1/payments/by-order/{declined_order}", token=me_tok, note="(failed)")               # 200 failed
call(P, "GET", B, path=f"/api/v1/payments/by-order/{foreign_order}", token=me_tok, note="(not mine)")              # 404
call(P, "GET", B, path=f"/api/v1/payments/by-order/{uuid.uuid4()}", token=me_tok)                                  # 404
call(P, "GET", B, path="/api/v1/payments/by-order/nope", token=me_tok)                                             # 400
call(P, "GET", B, path=f"/api/v1/payments/by-order/{paid_order}")                                                  # 401

A = "/api/v1/admin/payments"
call(P, "GET", A, token=admin)                                                                                     # 200
call(P, "GET", A, path=A + "?status=failed", token=admin, note="?status=failed")
call(P, "GET", A, path=A + f"?order_id={paid_order}", token=admin, note="?order_id=")
call(P, "GET", A, path=A + "?page=2&page_size=1", token=admin, note="?page=2&page_size=1")
call(P, "GET", A, path=A + "?status=bogus", token=admin)                                                           # 400
call(P, "GET", A, path=A + "?order_id=nope", token=admin)                                                          # 400
call(P, "GET", A, token=me_tok)                                                                                    # 403
call(P, "GET", A)                                                                                                  # 401
call(P, "GET", A + "/{id}", path=f"{A}/{paid['id']}", token=admin)                                                 # 200
call(P, "GET", A + "/{id}", path=f"{A}/{uuid.uuid4()}", token=admin)                                               # 404
call(P, "GET", A + "/{id}", path=f"{A}/nope", token=admin)                                                         # 400

IP = "/api/v1/internal/payments/by-order/{orderId}"
call(P, "GET", IP, path=f"/api/v1/internal/payments/by-order/{paid_order}", base=PAYMENT, key=KEY)                 # 200
call(P, "GET", IP, path=f"/api/v1/internal/payments/by-order/{uuid.uuid4()}", base=PAYMENT, key=KEY)               # 404
call(P, "GET", IP, path="/api/v1/internal/payments/by-order/nope", base=PAYMENT, key=KEY)                          # 400
call(P, "GET", IP, path=f"/api/v1/internal/payments/by-order/{paid_order}", base=PAYMENT, key="wrong-key", note="(wrong key)")  # 401
for hp in ("healthz", "readyz"):
    resp = requests.get(f"{PAYMENT}/{hp}"); check(P, "GET", f"/{hp}", resp)

# ---- every protected operation: no token -> 401, customer token -> 403 (where documented)
for api, doc in specs.items():
    for tpl, ops in doc["paths"].items():
        for m, op in ops.items():
            if m not in ("get", "post", "patch", "delete") or not op.get("security"):
                continue
            path = tpl
            for name in ("id", "variantId", "orderId"):
                path = path.replace("{" + name + "}", str(uuid.uuid4()))
            if op["security"] == [{"internalKey": []}]:
                call(api, m.upper(), tpl, path=path, base=INTERNALS[api], json_body={} if m != "get" else None, note="(no key)")
                continue
            call(api, m.upper(), tpl, path=path, json_body={} if m != "get" else None, note="(no token)")
            if "403" in op["responses"]:
                call(api, m.upper(), tpl, path=path, token=me_tok, json_body={} if m != "get" else None, note="(customer)")

# ---- coverage: documented responses never exercised
print("== Problems ==")
for p in problems: print(" -", p)
if not problems: print(" none")
print("\n== Documented but not exercised ==")
for api, doc in specs.items():
    for tpl, ops in doc["paths"].items():
        for m, op in ops.items():
            if m not in ("get", "post", "patch", "delete"): continue
            miss = [c for c in op["responses"] if (api, m, tpl, c) not in seen]
            if miss: print(f" {api}: {m.upper()} {tpl}: {', '.join(miss)}")
print(f"\n{len(seen)} distinct (operation,status) pairs checked")
print("MARKER", tag)
