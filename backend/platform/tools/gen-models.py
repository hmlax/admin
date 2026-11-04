#!/usr/bin/env python3
"""Generate operator persistence models from proto, without reading Go schemas."""

import pathlib
import subprocess
import tempfile


def main():
    root = pathlib.Path(__file__).resolve().parents[2]
    sources = root / "platform/proto"
    files = sorted((sources / "operator/v1").glob("model_*.proto"))
    if not files:
        raise RuntimeError("operator persistence protos are missing")
    with tempfile.TemporaryDirectory(prefix="operator-modelgen-") as workdir:
        temporary = pathlib.Path(workdir)
        deps = temporary / "deps"
        subprocess.run(["buf", "export", "--output", str(deps)], cwd=root, check=True)
        plugin = temporary / "protoc-gen-gorm-model"
        subprocess.run(
            ["go", "build", "-o", str(plugin), "./platform/tools/protoc-gen-gorm-model"],
            cwd=root, check=True,
        )
        mappings = [
            "M" + str(path.relative_to(deps)) + "=go-admin/platform/pb/" + str(path.parent.relative_to(deps))
            for path in sorted(deps.rglob("*.proto"))
            if not path.read_text().find("option go_package") >= 0
            and path.is_relative_to(deps)
        ]
        # Owned RPC messages use their explicit go_package options from make api.
        mappings = [mapping for mapping in mappings if not mapping.startswith("Mgoogle/")]
        command = [
            "protoc", "-I", str(sources), "-I", str(root / "platform/third_party"), "-I", str(deps),
            "--plugin=protoc-gen-gorm-model=" + str(plugin),
            "--gorm-model_out=paths=source_relative," + ",".join(mappings) + ":" + str(root),
            *[str(path.relative_to(sources)) for path in files],
        ]
        subprocess.run(command, cwd=root, check=True)


if __name__ == "__main__":
    main()
