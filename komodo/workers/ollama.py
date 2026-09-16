"""Ollama adapter over urllib: text in, text out, no tools; unreachable is a soft failure."""

from __future__ import annotations

import json
import urllib.error
import urllib.request

from .base import Brief, Result, Worker


class OllamaWorker(Worker):
    """Calls a local Ollama server's generate endpoint."""

    name = "ollama"

    def __init__(self, url: str = "http://localhost:11434"):
        self.url = url.rstrip("/")

    def invoke(self, brief: Brief) -> Result:
        """Posts the brief as one generate request and parses the reply, as JSON when a schema was given."""
        payload = {
            "model": brief.spec.get("model", ""),
            "system": brief.system,
            "prompt": brief.prompt,
            "stream": False,
            "options": {"num_ctx": int(brief.spec.get("num_ctx", 16384))},
        }
        if brief.schema:
            payload["format"] = "json"
        request = urllib.request.Request(
            self.url + "/api/generate",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json"},
        )
        try:
            with urllib.request.urlopen(request, timeout=brief.timeout_s) as response:
                body = json.loads(response.read().decode("utf-8"))
        except (urllib.error.URLError, OSError, ValueError) as error:
            return Result(ok=False, error="ollama unreachable at %s: %s" % (self.url, error))
        text = str(body.get("response") or "")
        result = Result(
            ok=bool(text),
            text=text,
            input_tokens=int(body.get("prompt_eval_count") or 0),
            output_tokens=int(body.get("eval_count") or 0),
            turns=1,
        )
        if brief.schema:
            try:
                parsed = json.loads(text)
                result.data = parsed if isinstance(parsed, dict) else None
            except ValueError:
                result.ok = False
                result.error = "ollama returned non-JSON despite json format"
        if not result.ok and not result.error:
            result.error = "ollama returned an empty response"
        return result
