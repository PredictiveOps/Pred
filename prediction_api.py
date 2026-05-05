from __future__ import annotations

import sys
from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SOURCE_PATH = ROOT / "ai-ml" / "prediction_api.py"

sys.path.insert(0, str(SOURCE_PATH.parent))
try:
    spec = spec_from_file_location("ai_ml_prediction_api", SOURCE_PATH)
    module = module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
finally:
    sys.path.pop(0)

app = module.app
