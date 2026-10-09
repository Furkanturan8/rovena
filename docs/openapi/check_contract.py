"""Checks docs/openapi/*.yaml against the running stack (docker compose up).

For every call it verifies that the status code is documented, the JSON body matches the
documented schema, and no undocumented fields appear. It creates rows whose names start with
"speccheck" and removes them at the end (psql inside the postgres container).

    python3 -m venv .venv && .venv/bin/pip install openapi-spec-validator jsonschema pyyaml requests
    .venv/bin/python docs/openapi/check_contract.py

Uses ~12 of the 20 requests/minute that /auth/* allows per IP, so do not run it twice within a minute.
"""
import json, os, sys, time, subprocess, uuid, yaml, requests
from openapi_schema_validator import OAS30Validator

ROOT = str(__import__("pathlib").Path(__file__).resolve().parents[2])
G = "http://localhost:8080"
specs = {n: yaml.safe_load(open(f"{ROOT}/docs/openapi/{n}.yaml")) for n in ("identity", "catalog")}
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

def call(api, method, template, path=None, token=None, json_body=None, raw=None, note=""):
    headers = {}
    if token: headers["Authorization"] = f"Bearer {token}"
    if raw is not None:
        headers["Content-Type"] = "application/json"
    time.sleep(0.06)  # stay under the gateway's 30 req/s limit
    resp = requests.request(method, G + (path or template), headers=headers,
                            json=json_body, data=raw)
    check(api, method, template, resp, note)
    return resp

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


# ---- every protected operation: no token -> 401, customer token -> 403 (where documented)
for api, doc in specs.items():
    for tpl, ops in doc["paths"].items():
        for m, op in ops.items():
            if m not in ("get", "post", "patch", "delete") or not op.get("security"):
                continue
            path = tpl
            for name in ("id", "variantId"):
                path = path.replace("{" + name + "}", str(uuid.uuid4()))
            call(api, m.upper(), tpl, path=path, json_body={} if m != "get" else None, note="(no token)")
            if "403" in op["responses"]:
                call(api, m.upper(), tpl, path=path, token=me_tok, json_body={} if m != "get" else None, note="(customer)")

# ---- cleanup of everything this script created
def psql(db, sql):
    subprocess.run(["docker", "compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", db, "-q", "-c", sql],
                   cwd=ROOT, check=True, capture_output=True)
psql("catalog", "DELETE FROM products WHERE name LIKE 'speccheck%'; DELETE FROM categories WHERE name LIKE 'speccheck%' AND parent_id IS NOT NULL; DELETE FROM categories WHERE name LIKE 'speccheck%';")
psql("identity", "DELETE FROM users WHERE email LIKE 'speccheck-%'")

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
