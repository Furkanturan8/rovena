"""End-to-end and fault-injection scenarios against the running stack (make e2e).

Every scenario places real orders through the gateway and watches the whole saga:
Order -> Inventory -> RabbitMQ -> Payment -> RabbitMQ -> Order -> Inventory. Failure scenarios stop and
restart real containers (docker compose). The Order service must run with the short recovery timings
of docker-compose.e2e.yml (the Makefile takes care of that).

    python e2e/run.py            # all scenarios
    python e2e/run.py restart    # only scenarios whose name contains "restart"

Everything the run creates (user, product, stock, orders, payments, reservations) is removed at the end,
and every container it stopped is started again, also when a scenario crashes.
"""
import atexit, base64, json, os, subprocess, sys, threading, time, traceback, uuid
import requests

ROOT = str(__import__("pathlib").Path(__file__).resolve().parents[1])
G = "http://localhost:8080"
PAYMENT = "http://localhost:8085"
MQ = "http://localhost:15672/api"
env = dict(l.split("=", 1) for l in open(f"{ROOT}/.env").read().splitlines() if "=" in l and not l.startswith("#"))
KEY = os.environ.get("INTERNAL_API_KEY") or env["INTERNAL_API_KEY"]
MQ_AUTH = (env["RABBITMQ_USER"], env["RABBITMQ_PASSWORD"])
tag = uuid.uuid4().hex[:8]

order_ids, variant_ids, stopped = [], [], set()
ctx = {}

# ------------------------------------------------------------------ plumbing

def compose(*args, check=True):
    return subprocess.run(["docker", "compose", *args], cwd=ROOT, check=check, capture_output=True, text=True)

def psql(db, sql):
    return compose("exec", "-T", "postgres", "psql", "-U", "postgres", "-d", db, "-tA", "-c", sql).stdout.strip()

def until(fn, timeout, what, every=0.3):
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        try:
            last = fn()
            if last:
                return last
        except requests.RequestException:
            pass
        time.sleep(every)
    raise AssertionError(f"timed out after {timeout}s waiting for: {what} (last: {last})")

def check(cond, msg):
    if not cond:
        raise AssertionError(msg)

PORTS = {"identity": 8081, "catalog": 8082, "inventory": 8083, "payment": 8085, "order": 8086}

def ready(service):
    try:
        return requests.get(f"http://localhost:{PORTS[service]}/readyz", timeout=2).status_code == 200
    except requests.RequestException:
        return False

def stop(service):
    compose("stop", service); stopped.add(service)

def start(service):
    compose("start", service); stopped.discard(service)
    if service in PORTS:
        until(lambda: ready(service), 60, f"{service} ready")
    if service == "rabbitmq":
        until(lambda: requests.get(f"{MQ}/overview", auth=MQ_AUTH, timeout=2).ok, 90, "rabbitmq management")

def restart(service):
    compose("restart", service)
    until(lambda: ready(service), 60, f"{service} ready")

def mq_publish(event_type, payload, correlation="e2e-" + tag):
    envelope = {"event_id": str(uuid.uuid4()), "type": event_type, "version": 1,
                "occurred_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "correlation_id": correlation, "payload": payload}
    r = requests.post(f"{MQ}/exchanges/%2F/rovena.events/publish", auth=MQ_AUTH, json={
        "properties": {"delivery_mode": 2, "content_type": "application/json"}, "routing_key": event_type,
        "payload": json.dumps(envelope), "payload_encoding": "string"})
    check(r.ok and r.json().get("routed"), f"could not publish {event_type}: {r.text}")

def order_logs(since):
    return compose("logs", "order", "--since", since, check=False).stdout

# ------------------------------------------------------------------ fixtures

def login(email, password):
    r = requests.post(f"{G}/api/v1/auth/login", json={"email": email, "password": password}); r.raise_for_status()
    return r.json()["data"]["tokens"]["access_token"]

def setup():
    email, pw = f"e2e-{tag}@example.com", "Passw0rd!Passw0rd"
    r = requests.post(f"{G}/api/v1/auth/register", json={"email": email, "password": pw, "first_name": "E", "last_name": "E"}); r.raise_for_status()
    ctx["user"] = {"id": r.json()["data"]["user"]["id"], "h": {"Authorization": f"Bearer {login(email, pw)}"}}
    ctx["admin"] = {"Authorization": f"Bearer {login(env['ADMIN_EMAIL'], env['ADMIN_PASSWORD'])}"}
    cats = requests.get(f"{G}/api/v1/categories").json()["data"]
    ctx["category"] = cats[0]["id"]

def new_variant(stock=5, price=1000):
    n = len(variant_ids)
    r = requests.post(f"{G}/api/v1/admin/products", headers=ctx["admin"], json={
        "name": f"e2e {tag} product {n}", "brand": "E2E", "category_id": ctx["category"], "status": "active",
        "images": [{"url": "https://img.test/e.jpg", "alt": "e"}],
        "variants": [{"sku": f"e2e-{tag}-{n}", "size": "M", "color": "Mavi", "price_cents": price}]})
    r.raise_for_status()
    v = r.json()["data"]["variants"][0]
    requests.post(f"{G}/api/v1/admin/stock", headers=ctx["admin"],
                  json={"variant_id": v["id"], "sku": v["sku"], "on_hand": stock, "reason": "e2e"}).raise_for_status()
    variant_ids.append(v["id"])
    return v["id"]

def stock(vid):
    d = requests.get(f"{G}/api/v1/admin/stock/{vid}", headers=ctx["admin"]).json()["data"]
    return d["on_hand"], d["reserved"]

def fill_cart(vid, qty):
    h = ctx["user"]["h"]
    requests.delete(f"{G}/api/v1/cart", headers=h)
    requests.put(f"{G}/api/v1/cart/items/{vid}", headers=h, json={"quantity": qty}).raise_for_status()

def checkout(vid, qty=1, method="tok_visa", key=None, fill=True):
    if fill:
        fill_cart(vid, qty)
    r = requests.post(f"{G}/api/v1/checkout", headers={**ctx["user"]["h"], "Idempotency-Key": key or str(uuid.uuid4())},
                      json={"payment_method": method})
    check(r.status_code in (200, 201), f"checkout answered {r.status_code} {r.text}")
    o = r.json()["data"]
    if r.status_code == 201:
        order_ids.append(o["id"])
    return o

def get_order(oid):
    return requests.get(f"{G}/api/v1/orders/{oid}", headers=ctx["user"]["h"]).json()["data"]

def wait_status(oid, statuses, timeout=60):
    return until(lambda: (lambda o: o if o["status"] in statuses else None)(get_order(oid)), timeout, f"order {oid[:8]} in {statuses}")

def admin_order(oid):
    return requests.get(f"{G}/api/v1/admin/orders/{oid}", headers=ctx["admin"]).json()["data"]

def payments(oid):
    return int(psql("payment", f"select count(*) from payments where order_id='{oid}'"))

def payment(oid):
    r = requests.get(f"{PAYMENT}/api/v1/internal/payments/by-order/{oid}", headers={"X-Internal-Key": KEY})
    return r.json()["data"] if r.ok else None

def reservations(oid):
    return int(psql("inventory", f"select count(*) from reservations where order_id='{oid}'"))

# ------------------------------------------------------------------ scenarios

def happy_path():
    v = new_variant(5)
    o = checkout(v, 2)
    check(o["status"] == "awaiting_payment", f"first status {o['status']}")
    done = wait_status(o["id"], ("confirmed", "cancelled"))
    check(done["status"] == "confirmed", f"ended {done}")
    check(stock(v) == (3, 0), f"stock {stock(v)}, want on_hand 3 reserved 0")
    check(payments(o["id"]) == 1 and payment(o["id"])["status"] == "succeeded", "expected exactly one succeeded payment")
    hist = [h["status"] for h in get_order(o["id"])["history"]]
    check(hist == ["pending", "awaiting_payment", "paid", "confirmed"], f"history {hist}")

def insufficient_stock():
    v = new_variant(2)
    o = checkout(v, 3)
    check(o["status"] == "cancelled" and o["failure_code"] == "out_of_stock", f"order {o}")
    check(stock(v) == (2, 0), f"stock must be untouched, got {stock(v)}")
    check(payments(o["id"]) == 0, "no payment may be requested for an order without stock")

def payment_declined():
    v = new_variant(5)
    o = checkout(v, 2, "tok_declined")
    done = wait_status(o["id"], ("confirmed", "cancelled"))
    check(done["status"] == "cancelled" and done["failure_code"] == "card_declined", f"ended {done}")
    until(lambda: stock(v) == (5, 0), 15, "stock released")
    until(lambda: admin_order(o["id"])["reservation_state"] == "released", 15, "reservation_state released")

def provider_outage_ends_with_an_answer():
    v = new_variant(5)
    o = checkout(v, 1, "tok_provider_error")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 90)
    check(done["status"] == "cancelled" and done["failure_code"] == "provider_unavailable", f"ended {done}")
    until(lambda: stock(v) == (5, 0), 15, "stock released")

def double_checkout_request():
    v = new_variant(5)
    fill_cart(v, 2)
    key = str(uuid.uuid4())
    results, errors = [], []
    def go():
        try:
            r = requests.post(f"{G}/api/v1/checkout", headers={**ctx["user"]["h"], "Idempotency-Key": key}, json={"payment_method": "tok_visa"})
            results.append((r.status_code, r.json().get("data", {}).get("id")))
        except Exception as e:
            errors.append(e)
    threads = [threading.Thread(target=go) for _ in range(8)]
    [t.start() for t in threads]; [t.join() for t in threads]
    check(not errors, f"requests failed: {errors}")
    ids = {i for s, i in results if i}
    codes = sorted(s for s, _ in results)
    check(len(ids) == 1, f"8 requests with one key produced orders {ids} (codes {codes})")
    oid = ids.pop(); order_ids.append(oid)
    check(codes.count(201) == 1, f"exactly one request may create the order, codes {codes}")
    done = wait_status(oid, ("confirmed", "cancelled"))
    check(done["status"] == "confirmed", f"ended {done}")
    check(stock(v) == (3, 0), f"stock {stock(v)}: it must be taken once")
    check(payments(oid) == 1 and reservations(oid) == 1, "one payment and one reservation expected")
    mine = requests.get(f"{G}/api/v1/orders", headers=ctx["user"]["h"]).json()["data"]
    check(sum(1 for x in mine["items"] if x["id"] == oid) == 1, "order listed once")

def duplicate_events_change_nothing():
    v = new_variant(5)
    o = checkout(v, 2)
    wait_status(o["id"], ("confirmed",))
    p = payment(o["id"])
    before = (stock(v), len(get_order(o["id"])["history"]), payments(o["id"]), p["attempts"])
    ok = {"payment_id": p["id"], "order_id": o["id"], "amount_cents": o["total_cents"], "currency": "TRY", "provider_ref": p["provider_ref"]}
    for _ in range(3):
        mq_publish("PaymentSucceeded", ok)
    # a contradicting late failure must not cancel a paid order
    mq_publish("PaymentFailed", {"payment_id": p["id"], "order_id": o["id"], "amount_cents": o["total_cents"], "currency": "TRY",
                                 "failure_code": "card_declined", "failure_message": "late"})
    mq_publish("PaymentRequested", {"order_id": o["id"], "customer_id": ctx["user"]["id"], "idempotency_key": f"order:{o['id']}",
                                    "amount_cents": o["total_cents"], "currency": "TRY", "payment_method": "tok_visa"})
    time.sleep(4)
    after = (stock(v), len(get_order(o["id"])["history"]), payments(o["id"]), payment(o["id"])["attempts"])
    check(get_order(o["id"])["status"] == "confirmed", "the order must stay confirmed")
    check(before == after, f"state changed by duplicate events: {before} -> {after}")

def restart_order_mid_saga():
    v = new_variant(5)
    stop("payment")
    o = checkout(v, 1)
    check(o["status"] == "awaiting_payment", o)
    restart("order")
    start("payment")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 90)
    check(done["status"] == "confirmed", f"ended {done}")
    check(payments(o["id"]) == 1 and stock(v) == (4, 0), f"payments {payments(o['id'])}, stock {stock(v)}")

def restart_payment_while_requests_wait():
    v = new_variant(5)
    stop("payment")
    orders = [checkout(v, 1) for _ in range(3)]
    time.sleep(2)
    start("payment")
    for o in orders:
        d = wait_status(o["id"], ("confirmed", "cancelled"), 90)
        check(d["status"] == "confirmed", f"ended {d}")
    check(all(payments(o["id"]) == 1 for o in orders), "one payment per order")
    check(stock(v) == (2, 0), f"stock {stock(v)}")

def rabbitmq_outage():
    v = new_variant(5)
    stop("rabbitmq")
    o = checkout(v, 1)
    check(o["status"] == "awaiting_payment", f"checkout must work without the broker, got {o}")
    until(lambda: int(psql("orders", f"select count(*) from outbox_events where published_at is null and envelope->'payload'->>'order_id'='{o['id']}'")) == 1,
          10, "PaymentRequested waiting in the outbox")
    time.sleep(3)
    check(get_order(o["id"])["status"] == "awaiting_payment", "nothing can move while the broker is down")
    start("rabbitmq")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 120)
    check(done["status"] == "confirmed", f"ended {done}")
    check(payments(o["id"]) == 1 and stock(v) == (4, 0), "one payment, stock taken once")

def inventory_down_at_checkout():
    v = new_variant(5)
    stop("inventory")
    o = checkout(v, 1)
    check(o["status"] == "pending", f"order must wait as pending, got {o['status']}")
    start("inventory")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 90)
    check(done["status"] == "confirmed", f"ended {done}")
    check(reservations(o["id"]) == 1 and stock(v) == (4, 0), "one reservation, stock taken once")

def inventory_down_when_payment_arrives():
    v = new_variant(5)
    stop("payment")
    o = checkout(v, 1)
    stop("inventory")
    start("payment")
    until(lambda: payment(o["id"]) and payment(o["id"])["status"] == "succeeded", 30, "payment succeeded")
    time.sleep(2)
    check(get_order(o["id"])["status"] in ("awaiting_payment", "paid"), "the order cannot be confirmed without Inventory")
    start("inventory")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 90)
    check(done["status"] == "confirmed", f"ended {done}")
    check(stock(v) == (4, 0), f"stock {stock(v)}")

def lost_payment_request_is_asked_again():
    v = new_variant(5)
    stop("payment")
    o = checkout(v, 1)
    time.sleep(2)  # the relay has put it into the payment queue
    r = requests.delete(f"{MQ}/queues/%2F/payment.requests/contents", auth=MQ_AUTH)
    check(r.status_code == 204, f"purge: {r.status_code}")
    start("payment")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 90)
    check(done["status"] == "confirmed", f"ended {done}")
    check(admin_order(o["id"])["payment_requests"] >= 2, "the recovery job must have asked again")
    check(payments(o["id"]) == 1, "asking again must not charge twice")

def silent_payment_cancels_and_late_success_is_flagged():
    v = new_variant(5)
    stop("payment")
    t0 = time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime())
    o = checkout(v, 2)
    check(stock(v) == (5, 2), f"stock must be held, got {stock(v)}")
    done = wait_status(o["id"], ("confirmed", "cancelled"), 120)
    check(done["status"] == "cancelled" and done["failure_code"] == "payment_timeout", f"ended {done}")
    until(lambda: stock(v) == (5, 0), 15, "stock released")
    check(admin_order(o["id"])["payment_requests"] == 3, "three payment requests expected")
    start("payment")  # it now works through the three queued requests and succeeds, too late
    until(lambda: payment(o["id"]) and payment(o["id"])["status"] == "succeeded", 30, "late payment")
    until(lambda: "REFUND NEEDED" in order_logs("10m") and o["id"] in order_logs("10m"), 30, "REFUND NEEDED in the order log")
    check(get_order(o["id"])["status"] == "cancelled" and stock(v) == (5, 0), "a late success must not revive the order")

SCENARIOS = [happy_path, insufficient_stock, payment_declined, provider_outage_ends_with_an_answer, double_checkout_request,
             duplicate_events_change_nothing, restart_order_mid_saga, restart_payment_while_requests_wait, rabbitmq_outage,
             inventory_down_at_checkout, inventory_down_when_payment_arrives, lost_payment_request_is_asked_again,
             silent_payment_cancels_and_late_success_is_flagged]

# ------------------------------------------------------------------ cleanup

def cleanup():
    for s in sorted(stopped, key=lambda x: x != "rabbitmq"):
        compose("start", s, check=False)
    if stopped:
        time.sleep(5)
    if not ctx:
        return
    try:
        oids = ",".join(f"'{o}'" for o in order_ids) or "'00000000-0000-0000-0000-000000000000'"
        uid = ctx["user"]["id"]
        psql("orders", f"DELETE FROM outbox_events WHERE envelope->'payload'->>'order_id' IN ({oids}); DELETE FROM orders WHERE customer_id='{uid}'; DELETE FROM carts WHERE customer_id='{uid}'")
        psql("payment", f"DELETE FROM outbox_events WHERE envelope->'payload'->>'order_id' IN ({oids}); DELETE FROM payments WHERE customer_id='{uid}'")
        psql("inventory", f"""DELETE FROM stock_movements WHERE variant_id IN (SELECT variant_id FROM stock_items WHERE sku LIKE 'E2E-{tag.upper()}-%');
            DELETE FROM reservations WHERE order_id IN ({oids});
            DELETE FROM stock_items WHERE sku LIKE 'E2E-{tag.upper()}-%'""")
        psql("catalog", f"DELETE FROM products WHERE name LIKE 'e2e {tag} %'")
        psql("identity", f"DELETE FROM users WHERE id='{uid}'")
    except Exception as e:  # cleanup must never hide the real result
        print("cleanup problem:", e)

def main():
    for s in ("identity", "catalog", "inventory", "payment", "order"):
        if not ready(s):
            sys.exit(f"{s} is not ready; run `make up` first")
    atexit.register(cleanup)
    setup()
    only = sys.argv[1:]
    failed = []
    for fn in SCENARIOS:
        if only and not any(o in fn.__name__ for o in only):
            continue
        t0 = time.time()
        try:
            fn()
            print(f"PASS  {fn.__name__:55s} {time.time()-t0:5.1f}s", flush=True)
        except Exception as e:
            failed.append(fn.__name__)
            print(f"FAIL  {fn.__name__:55s} {time.time()-t0:5.1f}s\n      {e}", flush=True)
            if not isinstance(e, AssertionError):
                traceback.print_exc()
    dlq = [q for q in ("order.payment-results.dlq", "payment.requests.dlq") if requests.get(f"{MQ}/queues/%2F/{q}", auth=MQ_AUTH).json().get("messages", 0) > 0] if not stopped else []
    print("\n" + ("ALL SCENARIOS PASSED" if not failed else f"FAILED: {', '.join(failed)}"))
    if dlq:
        print("note: messages in", dlq)
    sys.exit(1 if failed else 0)

if __name__ == "__main__":
    main()
