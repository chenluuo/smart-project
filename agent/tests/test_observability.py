import asyncio
import io
import json
import logging
import sys
import unittest
from pathlib import Path

AGENT_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(AGENT_DIR))

from shared.observability import ObservabilityMiddleware, _JSONFormatter  # noqa: E402
from shared.trace import ensure_request_id, ensure_trace_id, set_actor_id  # noqa: E402


class TestObservabilityMiddleware(unittest.TestCase):
    def test_emits_headers_and_one_structured_record(self):
        captured_messages = []
        captured_context = {}

        async def app(scope, receive, send):
            scope["route"] = type("Route", (), {"path": "/items/{item_id}"})()
            set_actor_id("42")
            captured_context["trace_id"] = ensure_trace_id()
            captured_context["request_id"] = ensure_request_id()
            await send({"type": "http.response.start", "status": 201, "headers": []})
            await send({"type": "http.response.body", "body": b'{"ok":true}'})

        async def receive():
            return {"type": "http.request", "body": b"", "more_body": False}

        async def send(message):
            captured_messages.append(message)

        stream = io.StringIO()
        handler = logging.StreamHandler(stream)
        handler.setFormatter(_JSONFormatter())
        logger = logging.getLogger("observability.http")
        previous_handlers, previous_propagate, previous_level = logger.handlers[:], logger.propagate, logger.level
        logger.handlers = [handler]
        logger.propagate = False
        logger.setLevel(logging.INFO)
        self.addCleanup(lambda: setattr(logger, "handlers", previous_handlers))
        self.addCleanup(lambda: setattr(logger, "propagate", previous_propagate))
        self.addCleanup(lambda: logger.setLevel(previous_level))

        scope = {
            "type": "http",
            "method": "POST",
            "path": "/items/7",
            "query_string": b"token=not-logged",
            "headers": [(b"x-request-id", b"request-123"), (b"x-trace-id", b"trace-456")],
            "client": ("127.0.0.1", 12345),
        }
        asyncio.run(ObservabilityMiddleware(app, "test-service")(scope, receive, send))

        response_headers = dict(captured_messages[0]["headers"])
        self.assertEqual(response_headers[b"X-Request-ID"], b"request-123")
        self.assertEqual(response_headers[b"X-Trace-Id"], b"trace-456")
        self.assertEqual(captured_context, {"trace_id": "trace-456", "request_id": "request-123"})

        lines = stream.getvalue().strip().splitlines()
        self.assertEqual(len(lines), 1)
        record = json.loads(lines[0])
        self.assertEqual(record["event"], "http_request_completed")
        self.assertEqual(record["service"], "test-service")
        self.assertEqual(record["route"], "/items/{item_id}")
        self.assertEqual(record["path"], "/items/7")
        self.assertEqual(record["status"], 201)
        self.assertEqual(record["actor_id"], "42")
        self.assertNotIn("token", lines[0])

    def test_unhandled_error_still_has_headers_and_a_failure_record(self):
        captured_messages = []

        async def app(scope, receive, send):
            raise RuntimeError("sensitive details are not logged")

        async def receive():
            return {"type": "http.request", "body": b"", "more_body": False}

        async def send(message):
            captured_messages.append(message)

        stream = io.StringIO()
        handler = logging.StreamHandler(stream)
        handler.setFormatter(_JSONFormatter())
        logger = logging.getLogger("observability.http")
        previous_handlers, previous_propagate, previous_level = logger.handlers[:], logger.propagate, logger.level
        logger.handlers = [handler]
        logger.propagate = False
        logger.setLevel(logging.INFO)
        self.addCleanup(lambda: setattr(logger, "handlers", previous_handlers))
        self.addCleanup(lambda: setattr(logger, "propagate", previous_propagate))
        self.addCleanup(lambda: logger.setLevel(previous_level))

        scope = {
            "type": "http",
            "method": "GET",
            "path": "/broken",
            "headers": [],
            "client": ("127.0.0.1", 12345),
        }
        asyncio.run(ObservabilityMiddleware(app, "test-service")(scope, receive, send))

        self.assertEqual(captured_messages[0]["status"], 500)
        response_headers = dict(captured_messages[0]["headers"])
        self.assertTrue(response_headers[b"X-Request-ID"])
        self.assertTrue(response_headers[b"X-Trace-Id"])
        lines = stream.getvalue().strip().splitlines()
        self.assertEqual(len(lines), 1)
        record = json.loads(lines[0])
        self.assertEqual(record["result"], "failure")
        self.assertEqual(record["exception_type"], "RuntimeError")
        self.assertNotIn("sensitive details", lines[0])


if __name__ == "__main__":
    unittest.main()
