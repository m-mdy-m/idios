from pathlib import Path

from idios.config.loader import load_app_config


def test_load_default_config(tmp_path: Path):
    (tmp_path / "default.toml").write_text(
        """
[app]
name = "idios"
environment = "default"

[hardware]
gpu_vram_gb = 4
cpu_threads = 12
"""
    )
    config = load_app_config(tmp_path)
    assert config.app.name == "idios"
    assert config.hardware.gpu_vram_gb == 4


def test_local_overrides_default(tmp_path: Path):
    (tmp_path / "default.toml").write_text(
        """
[app]
environment = "default"
"""
    )
    (tmp_path / "local.toml").write_text(
        """
[app]
environment = "local-dev"
"""
    )
    config = load_app_config(tmp_path)
    assert config.app.environment == "local-dev"


def test_missing_config_files_fall_back_to_defaults(tmp_path: Path):
    config = load_app_config(tmp_path)
    assert config.app.name == "idios"
    assert config.storage.backend == "json_file"
