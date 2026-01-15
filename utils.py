"""Utility functions for SubTrends bot."""

import json
import logging
import os
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)


class EnvVarError(Exception):
    """Error for missing or invalid environment variables."""

    def __init__(self, var_name: str, message: str | None = None):
        self.var_name = var_name
        if message:
            super().__init__(f"environment variable {var_name}: {message}")
        else:
            super().__init__(f"environment variable {var_name} is not set")


def get_required_env_var(key: str) -> str:
    """Get a required environment variable or raise an error."""
    value = os.environ.get(key, "")
    if not value:
        raise EnvVarError(key)
    return value


def read_json_file(file_path: str, default: Any = None) -> Any:
    """Read and parse a JSON file.
    
    Args:
        file_path: Path to the JSON file
        default: Default value to return if file doesn't exist
        
    Returns:
        Parsed JSON data or default value
    """
    path = Path(file_path)
    
    if not path.exists():
        return default
    
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except json.JSONDecodeError as e:
        logger.error(f"Failed to parse JSON from {file_path}: {e}")
        return default
    except OSError as e:
        logger.error(f"Failed to read file {file_path}: {e}")
        return default


def write_json_file(file_path: str, data: Any) -> bool:
    """Write data to a JSON file, creating directories if needed.
    
    Args:
        file_path: Path to the JSON file
        data: Data to serialize to JSON
        
    Returns:
        True if successful, False otherwise
    """
    path = Path(file_path)
    
    try:
        # Ensure directory exists with restrictive permissions
        path.parent.mkdir(parents=True, exist_ok=True)
        os.chmod(path.parent, 0o700)
        
        # Write file
        with open(path, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2, default=str)
        
        # Set restrictive permissions on the file
        os.chmod(path, 0o600)
        
        return True
    except OSError as e:
        logger.error(f"Failed to write file {file_path}: {e}")
        return False
    except (TypeError, ValueError) as e:
        logger.error(f"Failed to serialize data for {file_path}: {e}")
        return False
