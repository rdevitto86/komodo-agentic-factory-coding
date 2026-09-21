"""The Brief in, Result out contract every provider implements."""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional


@dataclass
class Brief:
    """Everything one worker invocation receives; nothing else reaches it."""

    role: str
    system: str
    prompt: str
    cwd: str
    spec: Dict[str, Any]
    schema: Optional[Dict[str, Any]] = None
    tools: List[str] = field(default_factory=list)
    timeout_s: int = 900
    env: Optional[Dict[str, str]] = None
    task_id: str = ""

    @property
    def estimated_tokens(self) -> Dict[str, int]:
        """A rough token count per section, four characters per token."""
        return {"system": len(self.system) // 4, "prompt": len(self.prompt) // 4}


@dataclass
class Result:
    """What came back: parsed data when a schema was given, plus cost and timing."""

    ok: bool
    data: Optional[Dict[str, Any]] = None
    text: str = ""
    error: str = ""
    cost_usd: float = 0.0
    input_tokens: int = 0
    output_tokens: int = 0
    turns: int = 0
    seconds: float = 0.0
    provider: str = ""
    model: str = ""
    rate_limit: Optional[Dict[str, Any]] = None


class Worker:
    """Base provider; subclasses implement invoke."""

    name = "base"

    def run(self, brief: Brief) -> Result:
        """Times invoke and stamps the provider and model onto the result."""
        started = time.time()
        try:
            result = self.invoke(brief)
        except Exception as error:  # a provider crash is a failed worker, never a crashed run
            result = Result(ok=False, error="%s: %s" % (type(error).__name__, error))
        result.seconds = round(time.time() - started, 1)
        result.provider = self.name
        result.model = str(brief.spec.get("model", ""))
        return result

    def invoke(self, brief: Brief) -> Result:
        """Provider-specific execution."""
        raise NotImplementedError


def worker_for(provider: str, config: Any = None) -> Worker:
    """The provider instance for a name from a role spec."""
    if provider == "claude":
        from .claude import ClaudeWorker

        return ClaudeWorker()
    if provider == "ollama":
        from .ollama import OllamaWorker

        url = config.get("ollama.url") if config is not None else None
        return OllamaWorker(url or "http://localhost:11434")
    raise ValueError("unknown provider %r" % provider)
