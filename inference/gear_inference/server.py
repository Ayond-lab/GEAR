import hashlib
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from .extract import extract_work_authorisation


SYNTHETIC_NAMESPACE = "synthetic-cv-lab"


def application_id_from_source_ref(source_ref: str) -> str:
    prefix = f"fixture://{SYNTHETIC_NAMESPACE}/applications/"
    if not source_ref.startswith(prefix):
        raise ValueError("sourceRef must point to the synthetic fixture namespace")
    application_id = source_ref[len(prefix) :]
    if not application_id or "/" in application_id:
        raise ValueError("sourceRef application id is invalid")
    return application_id


def build_extract_response(request: dict, fetch_application) -> dict:
    source_ref = request.get("sourceRef", "")
    payload_digest = request.get("payloadDigest", "")
    profile = request.get("profile", "")
    if not source_ref or not payload_digest or profile != "work-authorisation":
        raise ValueError("sourceRef, payloadDigest, and work-authorisation profile are required")

    application_id = application_id_from_source_ref(source_ref)
    application = fetch_application(application_id)
    if application.get("payloadDigest") != payload_digest:
        raise ValueError("payloadDigest mismatch")
    application_text = application.get("applicationText", "")
    if not application_text:
        raise ValueError("application content unavailable")

    extraction = extract_work_authorisation(application_text)
    fields = {"workAuthorisationStatus": extraction.work_authorisation_status}
    evidence_seed = json.dumps(
        {
            "sourceRef": source_ref,
            "payloadDigest": payload_digest,
            "fields": fields,
            "confidence": extraction.confidence,
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    evidence_ref = "fixture://synthetic-cv-lab/extractions/" + hashlib.sha256(
        evidence_seed.encode("utf-8")
    ).hexdigest()
    return {
        "fields": fields,
        "confidence": extraction.confidence,
        "evidenceRefs": [evidence_ref],
    }


class InferenceHandler(BaseHTTPRequestHandler):
    fixture_store_url = "http://gear-fixture-store.gear-system.svc.cluster.local:8080"

    def do_GET(self):
        if self.path != "/healthz":
            self.send_error(404)
            return
        self._write_json(200, {"component": "gear-inference", "status": "ok"})

    def do_POST(self):
        if self.path != "/v1/extract":
            self.send_error(404)
            return
        try:
            length = int(self.headers.get("content-length", "0"))
            request = json.loads(self.rfile.read(min(length, 1024 * 1024)).decode("utf-8"))
            response = build_extract_response(request, self._fetch_application)
        except ValueError as exc:
            self.send_error(400, str(exc))
            return
        except (HTTPError, URLError, TimeoutError) as exc:
            self.send_error(503, str(exc))
            return
        self._write_json(200, response)

    def log_message(self, format, *args):
        return

    def _fetch_application(self, application_id: str) -> dict:
        url = self.fixture_store_url.rstrip("/") + "/v1/applications/" + application_id
        req = Request(url, headers={"accept": "application/json"})
        with urlopen(req, timeout=5) as response:
            return json.loads(response.read().decode("utf-8"))

    def _write_json(self, status: int, value: dict):
        data = json.dumps(value).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main():
    addr = os.environ.get("GEAR_ADDR", "0.0.0.0:8080")
    host, port = addr.rsplit(":", 1)
    InferenceHandler.fixture_store_url = os.environ.get(
        "GEAR_FIXTURE_STORE_URL", InferenceHandler.fixture_store_url
    )
    ThreadingHTTPServer((host, int(port)), InferenceHandler).serve_forever()


if __name__ == "__main__":
    main()
