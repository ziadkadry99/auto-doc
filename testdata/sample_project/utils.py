"""Utility functions for data processing."""

import hashlib
from typing import List, Dict, Any


def compute_hash(data: str) -> str:
    """Compute SHA256 hash of the given string."""
    return hashlib.sha256(data.encode()).hexdigest()


def flatten_dict(d: Dict[str, Any], prefix: str = "") -> Dict[str, Any]:
    """Flatten a nested dictionary into dot-notation keys."""
    items = {}
    for key, value in d.items():
        new_key = f"{prefix}.{key}" if prefix else key
        if isinstance(value, dict):
            items.update(flatten_dict(value, new_key))
        else:
            items[new_key] = value
    return items


def chunk_list(lst: List[Any], size: int) -> List[List[Any]]:
    """Split a list into chunks of the given size."""
    return [lst[i : i + size] for i in range(0, len(lst), size)]
