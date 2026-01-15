"""
Ravenforge Python SDK

This SDK provides utilities for developing Ravenforge tools in Python.
"""

import hashlib
import json
import os
import sys
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

__version__ = "1.0.0"

# Default paths inside containers
DEFAULT_INPUT_DIR = "/rf/in"
DEFAULT_OUTPUT_DIR = "/rf/out"


@dataclass
class Config:
    """SDK configuration."""
    input_dir: Path
    output_dir: Path
    run_id: str

    @classmethod
    def from_env(cls) -> "Config":
        """Create config from environment variables."""
        return cls(
            input_dir=Path(os.environ.get("RF_INPUT_DIR", DEFAULT_INPUT_DIR)),
            output_dir=Path(os.environ.get("RF_OUTPUT_DIR", DEFAULT_OUTPUT_DIR)),
            run_id=os.environ.get("RF_RUN_ID", ""),
        )


@dataclass
class OutputMeta:
    """Metadata for an output artifact."""
    name: str
    content_type: str
    hash: str
    size: int


@dataclass
class Result:
    """Tool execution result."""
    status: str
    outputs: List[OutputMeta] = field(default_factory=list)
    error: Optional[str] = None
    meta: Dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        d = {
            "status": self.status,
            "outputs": [
                {
                    "name": o.name,
                    "content_type": o.content_type,
                    "hash": o.hash,
                    "size": o.size,
                }
                for o in self.outputs
            ],
        }
        if self.error:
            d["error"] = self.error
        if self.meta:
            d["meta"] = self.meta
        return d


class Logger:
    """Structured JSON logger for tools."""

    def __init__(self):
        self._encoder = json.JSONEncoder()

    def _log(self, level: str, msg: str, **fields):
        entry = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "level": level,
            "msg": msg,
        }
        if fields:
            entry["fields"] = fields
        print(json.dumps(entry), flush=True)

    def debug(self, msg: str, **fields):
        """Log a debug message."""
        self._log("debug", msg, **fields)

    def info(self, msg: str, **fields):
        """Log an info message."""
        self._log("info", msg, **fields)

    def warn(self, msg: str, **fields):
        """Log a warning message."""
        self._log("warn", msg, **fields)

    def error(self, msg: str, **fields):
        """Log an error message."""
        self._log("error", msg, **fields)


class Tool:
    """Base class for Ravenforge tools."""

    def __init__(self, config: Optional[Config] = None):
        self.config = config or Config.from_env()
        self.logger = Logger()

    @property
    def input_dir(self) -> Path:
        """Get the input directory path."""
        return self.config.input_dir

    @property
    def output_dir(self) -> Path:
        """Get the output directory path."""
        return self.config.output_dir

    @property
    def run_id(self) -> str:
        """Get the current run ID."""
        return self.config.run_id

    def read_input(self, name: str) -> bytes:
        """Read an input file by name."""
        path = self.input_dir / name
        return path.read_bytes()

    def read_input_text(self, name: str, encoding: str = "utf-8") -> str:
        """Read an input file as text."""
        return self.read_input(name).decode(encoding)

    def read_input_json(self, name: str) -> Any:
        """Read and parse a JSON input file."""
        return json.loads(self.read_input_text(name))

    def read_input_jsonl(self, name: str) -> List[Dict]:
        """Read and parse a JSONL input file."""
        text = self.read_input_text(name)
        return [json.loads(line) for line in text.strip().split("\n") if line.strip()]

    def list_inputs(self) -> List[str]:
        """List all input file names."""
        if not self.input_dir.exists():
            return []
        return [f.name for f in self.input_dir.iterdir() if f.is_file()]

    def write_output(self, name: str, data: bytes):
        """Write an output file."""
        self.output_dir.mkdir(parents=True, exist_ok=True)
        path = self.output_dir / name
        path.write_bytes(data)

    def write_output_text(self, name: str, text: str, encoding: str = "utf-8"):
        """Write a text output file."""
        self.write_output(name, text.encode(encoding))

    def write_output_json(self, name: str, obj: Any, indent: int = 2):
        """Write a JSON output file."""
        self.write_output_text(name, json.dumps(obj, indent=indent))

    def write_output_jsonl(self, name: str, items: List[Dict]):
        """Write a JSONL output file."""
        lines = [json.dumps(item) for item in items]
        self.write_output_text(name, "\n".join(lines) + "\n")

    def _collect_output_meta(self) -> List[OutputMeta]:
        """Collect metadata for all output files."""
        outputs = []
        if not self.output_dir.exists():
            return outputs

        for path in self.output_dir.iterdir():
            if path.is_dir() or path.name == "result.json":
                continue

            data = path.read_bytes()
            hash_val = hashlib.sha256(data).hexdigest()

            outputs.append(OutputMeta(
                name=path.name,
                content_type="application/octet-stream",
                hash=hash_val,
                size=len(data),
            ))

        return outputs

    def write_result(self, result: Result):
        """Write the result.json file."""
        self.write_output_json("result.json", result.to_dict())

    def success(self, meta: Optional[Dict[str, str]] = None):
        """Write a success result."""
        outputs = self._collect_output_meta()
        result = Result(status="success", outputs=outputs, meta=meta or {})
        self.write_result(result)

    def fail(self, error: str):
        """Write a failure result."""
        result = Result(status="failed", error=error)
        self.write_result(result)


def get_param(name: str) -> Optional[str]:
    """Get a parameter value from environment."""
    return os.environ.get(f"RF_PARAM_{name}")


def get_param_with_default(name: str, default: str) -> str:
    """Get a parameter value or default."""
    return get_param(name) or default


def main_wrapper(func):
    """Decorator for tool main functions with error handling."""
    def wrapper(*args, **kwargs):
        tool = Tool()
        try:
            result = func(tool, *args, **kwargs)
            if result is None or result == 0:
                tool.success()
                return 0
            return result
        except Exception as e:
            tool.logger.error(f"Tool execution failed: {e}")
            tool.fail(str(e))
            return 1
    return wrapper


# Example usage:
# @main_wrapper
# def main(tool: Tool):
#     data = tool.read_input_json("input.json")
#     result = process(data)
#     tool.write_output_json("output.json", result)
