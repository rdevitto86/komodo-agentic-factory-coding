"""Worker providers: each turns a Brief into a Result."""

from .base import Brief, Result, Worker, worker_for

__all__ = ["Brief", "Result", "Worker", "worker_for"]
