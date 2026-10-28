#!/usr/bin/env python3
"""Пример получателя вебхуков ФСП: проверяет подпись и печатает событие.
Запуск:  WEBHOOK_SECRET=whsec_... python3 webhook_receiver.py   (порт 9000)
В консоли интеграций добавьте подписку на http://host.docker.internal:9000/hook
(Docker Desktop) или на адрес машины, доступный из контейнера api."""
import hashlib, hmac, json, os, time
from http.server import BaseHTTPRequestHandler, HTTPServer

SECRET = os.environ.get("WEBHOOK_SECRET", "").encode()

class Hook(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        ts = self.headers.get("X-FSP-Timestamp", "")
        expected = "sha256=" + hmac.new(SECRET, ts.encode() + b"." + body, hashlib.sha256).hexdigest()
        got = self.headers.get("X-FSP-Signature", "")
        fresh = ts.isdigit() and abs(time.time() - int(ts)) < 300  # защита от повторной отправки
        if not (hmac.compare_digest(expected, got) and fresh):
            self.send_response(401); self.end_headers(); print("ПОДПИСЬ НЕВЕРНА"); return
        event = json.loads(body)
        print(f"{event['event']} #{self.headers.get('X-FSP-Delivery')}:", json.dumps(event["data"], ensure_ascii=False))
        self.send_response(204); self.end_headers()

HTTPServer(("0.0.0.0", 9000), Hook).serve_forever()
